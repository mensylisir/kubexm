package kubeadm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	// JoinWorkerConfigTemplate is the template for the kubeadm join config for a worker node.
	JoinWorkerConfigTemplate = `apiVersion: kubeadm.k8s.io/v1beta3
kind: JoinConfiguration
discovery:
  bootstrapToken:
    apiServerEndpoint: "{{ .APIServerEndpoint }}"
    token: "{{ .Token }}"
    caCertHashes:
    - "{{ .CACertHash }}"
nodeRegistration:
  criSocket: "{{ .CRISocket }}"
  cgroupDriver: "{{ .CgroupDriver }}"
  kubeletExtraArgs:
    cgroup-driver: "{{ .CgroupDriver }}"
`
	KubeadmJoinWorkerConfigFileName = "kubeadm-join-worker-config.yaml"
)

// GenerateWorkerConfigStep is a step to generate the kubeadm config for worker nodes.
type GenerateWorkerConfigStep struct {
	step.Base
}

// GenerateWorkerConfigStepBuilder is a builder for GenerateWorkerConfigStep.
type GenerateWorkerConfigStepBuilder struct {
	step.Builder[GenerateWorkerConfigStepBuilder, *GenerateWorkerConfigStep]
}

// NewGenerateWorkerConfigStepBuilder creates a new GenerateWorkerConfigStepBuilder.
func NewGenerateWorkerConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateWorkerConfigStepBuilder {
	s := &GenerateWorkerConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm join configuration for worker nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateWorkerConfigStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *GenerateWorkerConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// JoinWorkerTemplateData holds the data for the kubeadm join config template for a worker.
type JoinWorkerTemplateData struct {
	APIServerEndpoint string
	Token             string
	CACertHash        string
	CRISocket         string
	CgroupDriver      string
}

func (s *GenerateWorkerConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()

	// Fetch join information from the context.
	token, ok := ctx.Get(common.ContextKeyBootstrapToken)
	if !ok {
		return nil, fmt.Errorf("bootstrap token not found in context")
	}
	caCertHash, ok := ctx.Get(common.ContextKeyCaCertHash)
	if !ok {
		return nil, fmt.Errorf("CA cert hash not found in context")
	}

	// Determine CRI socket and cgroup driver from cluster spec
	var criSocket, cgroupDriver string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		criSocket = common.ContainerdDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		criSocket = common.CRIODefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeDocker:
		criSocket = common.CriDockerdSocketPath
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeIsula:
		criSocket = common.IsuladDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	default:
		return nil, fmt.Errorf("unsupported container runtime: %s", cluster.Spec.Kubernetes.ContainerRuntime.Type)
	}

	// Control plane endpoint
	cpEndpoint := cluster.Spec.ControlPlaneEndpoint
	cpDomain := helpers.FirstNonEmpty(cpEndpoint.Domain, cpEndpoint.Address)
	cpPort := helpers.FirstNonZeroInteger(cpEndpoint.Port, common.DefaultAPIServerPort)

	data := JoinWorkerTemplateData{
		APIServerEndpoint: fmt.Sprintf("%s:%d", cpDomain, cpPort),
		Token:             token.(string),
		CACertHash:        caCertHash.(string),
		CRISocket:         criSocket,
		CgroupDriver:      cgroupDriver,
	}

	renderedConfig, err := templates.Render(JoinWorkerConfigTemplate, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubeadm join worker template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateWorkerConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinWorkerConfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", remoteConfigPath, ctx.GetHost().GetName(), err)
	}
	if !exists {
		logger.Info("Remote join config file does not exist. Step needs to run.")
		return false, nil
	}

	logger.Info("Remote join config file exists. Comparing content.")
	expectedContent, err := s.renderContent(ctx)
	if err !=.
		logger.Warnf("Could not render expected config for precheck: %v. Assuming step needs to run.", err)
		return false, nil
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read remote config file '%s': %w", remoteConfigPath, err)
	}
	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("Remote join config file content matches the expected content. Step is done.")
		return true, nil
	}

	logger.Info("Remote join config file content differs from expected content. Step needs to run to update it.")
	return false, nil
}

func (s *GenerateWorkerConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Rendering kubeadm join config for worker")
	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, KubeadmJoinWorkerConfigFileName)
	logger.Infof("Uploading/Updating rendered config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := runner.WriteFile(ctx.GoContext(), conn, renderedConfig, remoteConfigPath, "0644", false); err != nil {
		return fmt.Errorf("failed to upload kubeadm join config file: %w", err)
	}
	logger.Info("Kubeadm join configuration for worker generated and uploaded successfully.")
	return nil
}

func (s *GenerateWorkerConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinWorkerConfigFileName)
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateWorkerConfigStep)(nil)
