package kubeconfig

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

type GenerateAdminKubeconfigStep struct {
	step.Base
	ClusterName          string
	APIServerURL         string
	PKIDir               string
	RemoteKubeconfigFile string
}

type GenerateAdminKubeconfigStepBuilder struct {
	step.Builder[GenerateAdminKubeconfigStepBuilder, *GenerateAdminKubeconfigStep]
}

func NewGenerateAdminKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateAdminKubeconfigStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	controlEndpoint := "127.0.0.1"
	if clusterCfg.Spec.ControlPlaneEndpoint.Address != "" {
		controlEndpoint = clusterCfg.Spec.ControlPlaneEndpoint.Address
	}

	s := &GenerateAdminKubeconfigStep{
		ClusterName:          k8sSpec.ClusterName,
		APIServerURL:         fmt.Sprintf("https://%s:%d", controlEndpoint, common.DefaultAPIServerPort),
		PKIDir:               ctx.GetKubernetesCertsDir(),
		RemoteKubeconfigFile: filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate admin kubeconfig file (admin.conf)", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateAdminKubeconfigStepBuilder).Init(s)
	return b
}

func (s *GenerateAdminKubeconfigStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *GenerateAdminKubeconfigStep) renderKubeconfig() (string, error) {
	caCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.CACertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}

	clientCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.AdminCertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read admin client certificate (%s): %w", common.AdminCertFileName, err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.PKIDir, common.AdminKeyFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read admin client key (%s): %w", common.AdminKeyFileName, err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          s.ClusterName,
		APIServerURL:         s.APIServerURL,
		CACertDataBase64:     base64.StdEncoding.EncodeToString(caCert),
		UserName:             "kubernetes-admin",
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	return templates.Render(tmplContent, data)
}

func (s *GenerateAdminKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderKubeconfig()
	if err != nil {
		return false, fmt.Errorf("failed to render expected kubeconfig for precheck: %w", err)
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteKubeconfigFile)
	if err != nil {
		logger.Infof("Remote admin kubeconfig file %s not found, configuration is required.", s.RemoteKubeconfigFile)
		return false, nil
	}
	if string(remoteContent) != expectedContent {
		logger.Warn("Remote admin.conf file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("admin.conf file is up to date. Step is done.")
	return true, nil
}

func (s *GenerateAdminKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeconfigContent, err := s.renderKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to render admin.conf: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, kubeconfigContent, s.RemoteKubeconfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write admin.conf file: %w", err)
	}

	logger.Info("admin.conf has been created successfully.")
	return nil
}

func (s *GenerateAdminKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing kubeconfig file: %s", s.RemoteKubeconfigFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteKubeconfigFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove admin.conf file during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*GenerateAdminKubeconfigStep)(nil)
