package kube_proxy

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type KubeconfigTemplateData struct {
	ClusterName          string
	APIServerURL         string
	CACertDataBase64     string
	UserName             string
	ClientCertDataBase64 string
	ClientKeyDataBase64  string
}

type CreateKubeProxyKubeconfigStep struct {
	step.Base
	ClusterName          string
	APIServerURL         string
	PKIDir               string
	RemoteKubeconfigFile string
}

type CreateKubeProxyKubeconfigStepBuilder struct {
	step.Builder[CreateKubeProxyKubeconfigStepBuilder, *CreateKubeProxyKubeconfigStep]
}

func NewCreateKubeProxyKubeconfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CreateKubeProxyKubeconfigStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	controlEndpoint := "127.0.0.1"
	if clusterCfg.Spec.ControlPlaneEndpoint.Address != "" {
		controlEndpoint = clusterCfg.Spec.ControlPlaneEndpoint.Address
	}

	s := &CreateKubeProxyKubeconfigStep{
		ClusterName:          k8sSpec.ClusterName,
		APIServerURL:         fmt.Sprintf("https://%s:%d", controlEndpoint, common.DefaultAPIServerPort),
		PKIDir:               ctx.GetKubernetesCertsDir(),
		RemoteKubeconfigFile: filepath.Join(common.KubernetesConfigDir, common.KubeProxyKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kube-proxy kubeconfig file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CreateKubeProxyKubeconfigStepBuilder).Init(s)
	return b
}

func (s *CreateKubeProxyKubeconfigStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *CreateKubeProxyKubeconfigStep) renderKubeconfig() (string, error) {
	caCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.CACertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}

	clientCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.KubeProxyClientCertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read kube-proxy client certificate (%s): %w", common.KubeProxyClientCertFileName, err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.PKIDir, common.KubeProxyClientKeyFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read kube-proxy client key (%s): %w", common.KubeProxyClientKeyFileName, err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          s.ClusterName,
		APIServerURL:         s.APIServerURL,
		CACertDataBase64:     base64.StdEncoding.EncodeToString(caCert),
		UserName:             common.KubeProxyUser,
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	return templates.Render(tmplContent, data)
}

func (s *CreateKubeProxyKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Infof("Remote kubeconfig file %s not found, configuration is required.", s.RemoteKubeconfigFile)
		return false, nil
	}
	if string(remoteContent) != expectedContent {
		logger.Warn("Remote kube-proxy.kubeconfig file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kube-proxy.kubeconfig file is up to date. Step is done.")
	return true, nil
}

func (s *CreateKubeProxyKubeconfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	kubeconfigContent, err := s.renderKubeconfig()
	if err != nil {
		err = fmt.Errorf("failed to render kube-proxy.kubeconfig: %w", err)
		result.MarkFailed(err, "failed to render kubeconfig")
		return result, err
	}

	if err := helpers.WriteContentToRemote(ctx, conn, kubeconfigContent, s.RemoteKubeconfigFile, "0644", s.Sudo); err != nil {
		err = fmt.Errorf("failed to write kube-proxy.kubeconfig file: %w", err)
		result.MarkFailed(err, "failed to write kubeconfig")
		return result, err
	}

	logger.Info("kube-proxy.kubeconfig has been created successfully.")
	result.MarkCompleted("kubeconfig created successfully")
	return result, nil
}

func (s *CreateKubeProxyKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*CreateKubeProxyKubeconfigStep)(nil)
