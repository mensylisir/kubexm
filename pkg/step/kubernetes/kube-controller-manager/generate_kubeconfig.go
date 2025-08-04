package kube_controller_manager

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
	"os"
	"path/filepath"
	"time"
)

type KubeconfigTemplateData struct {
	ClusterName          string
	APIServerURL         string
	CACertDataBase64     string
	UserName             string
	ClientCertDataBase64 string
	ClientKeyDataBase64  string
}

type CreateControllerManagerKubeconfigStep struct {
	step.Base
	ClusterName          string
	APIServerURL         string
	PKIDir               string
	RemoteKubeconfigFile string
}

type CreateControllerManagerKubeconfigStepBuilder struct {
	step.Builder[CreateControllerManagerKubeconfigStepBuilder, *CreateControllerManagerKubeconfigStep]
}

func NewCreateControllerManagerKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *CreateControllerManagerKubeconfigStepBuilder {
	k8sSpec := ctx.GetClusterConfig().Spec.Kubernetes
	host := ctx.GetHost()
	s := &CreateControllerManagerKubeconfigStep{
		ClusterName:          k8sSpec.ClusterName,
		APIServerURL:         fmt.Sprintf("https://%s:%d", host.GetInternalAddress(), common.DefaultAPIServerPort),
		PKIDir:               common.KubernetesPKIDir,
		RemoteKubeconfigFile: filepath.Join(common.KubernetesConfigDir, common.ControllerManagerKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kube-controller-manager kubeconfig file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CreateControllerManagerKubeconfigStepBuilder).Init(s)
	return b
}

func (s *CreateControllerManagerKubeconfigStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *CreateControllerManagerKubeconfigStep) renderKubeconfig() (string, error) {
	caCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.CACertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}

	clientCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.ControllerManagerCertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read system:kube-controller-manager.crt: %w", err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.PKIDir, common.ControllerManagerKeyFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read system:kube-controller-manager.key: %w", err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          s.ClusterName,
		APIServerURL:         s.APIServerURL,
		CACertDataBase64:     base64.StdEncoding.EncodeToString(caCert),
		UserName:             "system:kube-controller-manager",
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	return templates.Render(tmplContent, data)
}

func (s *CreateControllerManagerKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warn("Remote kube-controller-manager.kubeconfig file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kube-controller-manager.kubeconfig file is up to date. Step is done.")
	return true, nil
}

func (s *CreateControllerManagerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeconfigContent, err := s.renderKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to render kube-controller-manager.kubeconfig: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, kubeconfigContent, s.RemoteKubeconfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kube-controller-manager.kubeconfig file: %w", err)
	}

	logger.Info("kube-controller-manager.kubeconfig has been created successfully.")
	return nil
}

func (s *CreateControllerManagerKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*CreateControllerManagerKubeconfigStep)(nil)
