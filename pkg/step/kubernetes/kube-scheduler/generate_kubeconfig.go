package kube_scheduler

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

type CreateSchedulerKubeconfigStep struct {
	step.Base
	ClusterName          string
	APIServerURL         string
	PKIDir               string
	RemoteKubeconfigFile string
}

type CreateSchedulerKubeconfigStepBuilder struct {
	step.Builder[CreateSchedulerKubeconfigStepBuilder, *CreateSchedulerKubeconfigStep]
}

func NewCreateSchedulerKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *CreateSchedulerKubeconfigStepBuilder {
	k8sSpec := ctx.GetClusterConfig().Spec.Kubernetes
	host := ctx.GetHost()
	s := &CreateSchedulerKubeconfigStep{
		ClusterName:          k8sSpec.ClusterName,
		APIServerURL:         fmt.Sprintf("https://%s:%d", host.GetInternalAddress(), common.DefaultAPIServerPort),
		PKIDir:               common.KubernetesPKIDir,
		RemoteKubeconfigFile: filepath.Join(common.KubernetesConfigDir, common.SchedulerKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kube-scheduler kubeconfig file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CreateSchedulerKubeconfigStepBuilder).Init(s)
	return b
}

func (s *CreateSchedulerKubeconfigStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *CreateSchedulerKubeconfigStep) renderKubeconfig() (string, error) {
	caCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.CACertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}

	clientCert, err := os.ReadFile(filepath.Join(s.PKIDir, common.SchedulerCertFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read system:kube-scheduler.crt: %w", err)
	}
	clientKey, err := os.ReadFile(filepath.Join(s.PKIDir, common.SchedulerKeyFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read system:kube-scheduler.key: %w", err)
	}

	data := KubeconfigTemplateData{
		ClusterName:          s.ClusterName,
		APIServerURL:         s.APIServerURL,
		CACertDataBase64:     base64.StdEncoding.EncodeToString(caCert),
		UserName:             "system:kube-scheduler",
		ClientCertDataBase64: base64.StdEncoding.EncodeToString(clientCert),
		ClientKeyDataBase64:  base64.StdEncoding.EncodeToString(clientKey),
	}

	tmplContent, err := templates.Get("kubernetes/kubeconfig.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig template: %w", err)
	}

	return templates.Render(tmplContent, data)
}

func (s *CreateSchedulerKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warn("Remote kube-scheduler.kubeconfig file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kube-scheduler.kubeconfig file is up to date. Step is done.")
	return true, nil
}

func (s *CreateSchedulerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeconfigContent, err := s.renderKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to render kube-scheduler.kubeconfig: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, kubeconfigContent, s.RemoteKubeconfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kube-scheduler.kubeconfig file: %w", err)
	}

	logger.Info("kube-scheduler.kubeconfig has been created successfully.")
	return nil
}

func (s *CreateSchedulerKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*CreateSchedulerKubeconfigStep)(nil)
