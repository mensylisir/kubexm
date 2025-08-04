package kubelet

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

type CreateKubeletKubeconfigStep struct {
	step.Base
	ClusterName          string
	APIServerURL         string
	PKIDir               string
	RemoteKubeconfigFile string
}

type CreateKubeletKubeconfigStepBuilder struct {
	step.Builder[CreateKubeletKubeconfigStepBuilder, *CreateKubeletKubeconfigStep]
}

func NewCreateKubeletKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *CreateKubeletKubeconfigStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	controlEndpoint := "127.0.0.1"
	if clusterCfg.Spec.ControlPlaneEndpoint.Address != "" {
		controlEndpoint = clusterCfg.Spec.ControlPlaneEndpoint.Address
	}
	s := &CreateKubeletKubeconfigStep{
		ClusterName:          k8sSpec.ClusterName,
		APIServerURL:         fmt.Sprintf("https://%s:%d", controlEndpoint, common.DefaultAPIServerPort),
		PKIDir:               ctx.GetKubernetesCertsDir(),
		RemoteKubeconfigFile: filepath.Join(common.KubernetesConfigDir, common.KubeletKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kubelet kubeconfig file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CreateKubeletKubeconfigStepBuilder).Init(s)
	return b
}

func (s *CreateKubeletKubeconfigStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *CreateKubeletKubeconfigStep) renderKubeconfig(ctx runtime.ExecutionContext) (string, error) {
	nodeName := ctx.GetHost().GetName()
	userName := fmt.Sprintf("system:node:%s", nodeName)

	caCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.CACertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}

	clientCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.KubeletClientCertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read kubelet client certificate %s: %w", common.KubeletClientCertFileName, err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.PKIDir, common.KubeletClientKeyFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read kubelet client key %s: %w", common.KubeletClientKeyFileName, err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          s.ClusterName,
		APIServerURL:         s.APIServerURL,
		CACertDataBase64:     base64.StdEncoding.EncodeToString(caCert),
		UserName:             userName,
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	return templates.Render(tmplContent, data)
}

func (s *CreateKubeletKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderKubeconfig(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to render expected kubeconfig for precheck: %w", err)
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteKubeconfigFile)
	if err != nil {
		logger.Infof("Remote kubeconfig file %s not found, configuration is required.", s.RemoteKubeconfigFile)
		return false, nil
	}
	if string(remoteContent) != expectedContent {
		logger.Warn("Remote kubelet.kubeconfig file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kubelet.kubeconfig file is up to date. Step is done.")
	return true, nil
}

func (s *CreateKubeletKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeconfigContent, err := s.renderKubeconfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to render kubelet.kubeconfig: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, kubeconfigContent, s.RemoteKubeconfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kubelet.kubeconfig file: %w", err)
	}

	logger.Info("kubelet.kubeconfig has been created successfully.")
	return nil
}

func (s *CreateKubeletKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing kubeconfig file: %s", s.RemoteKubeconfigFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteKubeconfigFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kubeconfig file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*CreateKubeletKubeconfigStep)(nil)
