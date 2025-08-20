package kubexm

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type KubeconfigTemplateData struct {
	ClusterName          string
	APIServerURL         string
	CACertDataBase64     string
	UserName             string
	ClientCertDataBase64 string
	ClientKeyDataBase64  string
}

type BinaryRenewAllKubeconfigsStep struct {
	step.Base
	pkiDir    string
	outputDir string
}

type BinaryRenewAllKubeconfigsStepBuilder struct {
	step.Builder[BinaryRenewAllKubeconfigsStepBuilder, *BinaryRenewAllKubeconfigsStep]
}

func NewBinaryRenewAllKubeconfigsStepBuilder(ctx runtime.Context, instanceName string) *BinaryRenewAllKubeconfigsStepBuilder {
	s := &BinaryRenewAllKubeconfigsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate/Renew all Kubernetes kubeconfig files"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(BinaryRenewAllKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *BinaryRenewAllKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type kubeconfigDefinition struct {
	fileName       string
	userName       string
	certFile       string
	keyFile        string
	isNodeSpecific bool
}

func (s *BinaryRenewAllKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for kubeconfig renewal...")

	baseCertsDir := ctx.GetKubernetesCertsDir()
	baseKubeconfigDir := filepath.Join(filepath.Dir(ctx.GetKubernetesCertsDir()), "kubeconfig")

	s.pkiDir = baseCertsDir
	s.outputDir = baseKubeconfigDir

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *BinaryRenewAllKubeconfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	clusterSpec := ctx.GetClusterConfig().Spec

	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", s.outputDir, err)
	}

	defs := []kubeconfigDefinition{
		{
			fileName: common.ControllerManagerKubeconfigFileName,
			userName: common.KubeControllerManagerUser,
			certFile: common.ControllerManagerCertFileName,
			keyFile:  common.ControllerManagerKeyFileName,
		},
		{
			fileName: common.SchedulerKubeconfigFileName,
			userName: common.KubeSchedulerUser,
			certFile: common.SchedulerCertFileName,
			keyFile:  common.SchedulerKeyFileName,
		},
		{
			fileName: common.AdminKubeconfigFileName,
			userName: "kubernetes-admin",
			certFile: common.AdminCertFileName,
			keyFile:  common.AdminKeyFileName,
		},
		{
			fileName: common.KubeProxyKubeconfigFileName,
			userName: common.KubeProxyUser,
			certFile: common.KubeProxyClientCertFileName,
			keyFile:  common.KubeProxyClientKeyFileName,
		},
		{
			fileName:       common.KubeletKubeconfigFileName,
			userName:       "system:node:%s",
			certFile:       "kubelet-client-%s.crt",
			keyFile:        "kubelet-client-%s.key",
			isNodeSpecific: true,
		},
	}

	serverURL := fmt.Sprintf("https://%s:%s", clusterSpec.ControlPlaneEndpoint, common.DefaultAPIServerPort)
	caCert, err := os.ReadFile(filepath.Join(s.pkiDir, common.CACertFileName))
	if err != nil {
		return fmt.Errorf("failed to read ca.crt: %w", err)
	}
	caCertBase64 := base64.StdEncoding.EncodeToString(caCert)

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	for _, def := range defs {
		if def.isNodeSpecific {
			allHosts := ctx.GetHostsByRole(common.RoleMaster)
			allHosts = append(allHosts, ctx.GetHostsByRole(common.RoleWorker)...)
			for _, node := range allHosts {
				nodeName := node.GetName()
				userName := fmt.Sprintf(def.userName, nodeName)
				certFile := fmt.Sprintf(def.certFile, nodeName)
				keyFile := fmt.Sprintf(def.keyFile, nodeName)
				fileName := fmt.Sprintf(def.fileName, nodeName)
				if err := s.renderAndWriteKubeconfig(ctx, tmplContent, clusterSpec.Kubernetes.DNSDomain, serverURL, caCertBase64, userName, certFile, keyFile, fileName); err != nil {
					return err
				}
			}
		} else {
			if err := s.renderAndWriteKubeconfig(ctx, tmplContent, clusterSpec.Kubernetes.DNSDomain, serverURL, caCertBase64, def.userName, def.certFile, def.keyFile, def.fileName); err != nil {
				return err
			}
		}
	}

	logger.Info("All kubeconfig files generated successfully.")
	return nil
}

func (s *BinaryRenewAllKubeconfigsStep) renderAndWriteKubeconfig(ctx runtime.ExecutionContext, tmpl, clusterName, serverURL, caCertB64, userName, certFile, keyFile, outFileName string) error {
	log := ctx.GetLogger().With("kubeconfig", outFileName)
	log.Info("Generating kubeconfig...")

	clientCert, err := os.ReadFile(filepath.Join(s.pkiDir, certFile))
	if err != nil {
		return fmt.Errorf("failed to read client cert '%s': %w", certFile, err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.pkiDir, keyFile))
	if err != nil {
		return fmt.Errorf("failed to read client key '%s': %w", keyFile, err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          clusterName,
		APIServerURL:         serverURL,
		CACertDataBase64:     caCertB64,
		UserName:             userName,
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	content, err := templates.Render(tmpl, data)
	if err != nil {
		return fmt.Errorf("failed to render kubeconfig template for '%s': %w", outFileName, err)
	}

	outputPath := filepath.Join(s.outputDir, outFileName)
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file '%s': %w", outputPath, err)
	}
	return nil
}

func (s *BinaryRenewAllKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if s.outputDir != "" && strings.HasSuffix(s.outputDir, "-new") {
		logger.Warnf("Rolling back by deleting output directory: %s", s.outputDir)
		if err := os.RemoveAll(s.outputDir); err != nil {
			logger.Errorf("Failed to remove output directory '%s' during rollback: %v", s.outputDir, err)
		}
	} else {
		logger.Warn("Rollback for in-place kubeconfig generation is not performed automatically.")
	}
	return nil
}

var _ step.Step = (*BinaryRenewAllKubeconfigsStep)(nil)
