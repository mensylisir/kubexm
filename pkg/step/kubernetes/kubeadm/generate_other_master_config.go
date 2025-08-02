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
	KubeadmJoinMasterConfigFileName = "kubeadm-join-master-config.yaml"
)

// GenerateOtherMasterConfigStep is a step to generate the kubeadm config for other master nodes.
type GenerateOtherMasterConfigStep struct {
	step.Base
}

// GenerateOtherMasterConfigStepBuilder is a builder for GenerateOtherMasterConfigStep.
type GenerateOtherMasterConfigStepBuilder struct {
	step.Builder[GenerateOtherMasterConfigStepBuilder, *GenerateOtherMasterConfigStep]
}

// NewGenerateOtherMasterConfigStepBuilder creates a new GenerateOtherMasterConfigStepBuilder.
func NewGenerateOtherMasterConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateOtherMasterConfigStepBuilder {
	s := &GenerateOtherMasterConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm join configuration for other masters", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateOtherMasterConfigStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *GenerateOtherMasterConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// JoinMasterTemplateData holds the data for the kubeadm join config template.
type JoinMasterTemplateData struct {
	APIServerEndpoint string
	Token             string
	CACertHash        string
	CRISocket         string
	CgroupDriver      string
	AdvertiseAddress  string
	BindPort          int
	CertificateKey    string
}

func (s *GenerateOtherMasterConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()

	// Fetch join information from the context. This must be set by the 'init_first_master' step.
	token, ok := ctx.Get(common.ContextKeyBootstrapToken)
	if !ok {
		return nil, fmt.Errorf("bootstrap token not found in context")
	}
	caCertHash, ok := ctx.Get(common.ContextKeyCaCertHash)
	if !ok {
		return nil, fmt.Errorf("CA cert hash not found in context")
	}
	certificateKey, ok := ctx.Get(common.ContextKeyCertificateKey)
	if !ok {
		return nil, fmt.Errorf("certificate key not found in context")
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

	data := JoinMasterTemplateData{
		APIServerEndpoint: fmt.Sprintf("%s:%d", cpDomain, cpPort),
		Token:             token.(string),
		CACertHash:        caCertHash.(string),
		CertificateKey:    certificateKey.(string),
		CRISocket:         criSocket,
		CgroupDriver:      cgroupDriver,
		AdvertiseAddress:  currentHost.GetInternalAddress(),
		BindPort:          cpPort,
	}

	templateContent, err := templates.Get("kubernetes/kubeadm/kubeadm-join-master-config.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeadm join master template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubeadm join master template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateOtherMasterConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinMasterConfigFileName)
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
	if err != nil {
		// If we can't render, we can't compare. Assume it needs to run.
		// This can happen if the join token is not yet in the context.
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

func (s *GenerateOtherMasterConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Rendering kubeadm join config for master")
	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, KubeadmJoinMasterConfigFileName)
	logger.Infof("Uploading/Updating rendered config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := runner.WriteFile(ctx.GoContext(), conn, renderedConfig, remoteConfigPath, "0644", false); err != nil {
		return fmt.Errorf("failed to upload kubeadm join config file: %w", err)
	}
	logger.Info("Kubeadm join configuration for master generated and uploaded successfully.")
	return nil
}

func (s *GenerateOtherMasterConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinMasterConfigFileName)
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateOtherMasterConfigStep)(nil)
