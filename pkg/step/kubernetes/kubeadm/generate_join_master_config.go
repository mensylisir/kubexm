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

type GenerateJoinMasterConfigStep struct {
	step.Base
}
type GenerateJoinMasterConfigStepBuilder struct {
	step.Builder[GenerateJoinMasterConfigStepBuilder, *GenerateJoinMasterConfigStep]
}

func NewGenerateJoinMasterConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateJoinMasterConfigStepBuilder {
	s := &GenerateJoinMasterConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm join configuration for master", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateJoinMasterConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateJoinMasterConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type JoinMasterTemplateData struct {
	Discovery        DiscoveryTemplate
	ControlPlane     ControlPlaneTemplate
	NodeRegistration NodeRegistrationTemplate
}

type DiscoveryTemplate struct {
	APIServerEndpoint string
	BootstrapToken    string
}

type ControlPlaneTemplate struct {
	AdvertiseAddress string
	BindPort         int
	CertificateKey   string
}

func (s *GenerateJoinMasterConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()

	data := JoinMasterTemplateData{}

	data.Discovery.APIServerEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, cluster.Spec.ControlPlaneEndpoint.Port)
	tokenVal, found := ctx.GetTaskCache().Get(common.CacheKubeadmInitToken)
	if !found {
		return nil, fmt.Errorf("bootstrap token not found in task cache with key '%s'", common.CacheKubeadmInitToken)
	}
	token, ok := tokenVal.(string)
	if !ok {
		return nil, fmt.Errorf("cached bootstrap token is not a string")
	}
	data.Discovery.BootstrapToken = token

	certKeyVal, found := ctx.GetTaskCache().Get(common.CacheKubeadmInitCertKey)
	if !found {
		return nil, fmt.Errorf("certificate key not found in task cache with key '%s'", common.CacheKubeadmInitCertKey)
	}
	certKey, ok := certKeyVal.(string)
	if !ok {
		return nil, fmt.Errorf("cached certificate key is not a string")
	}
	data.ControlPlane.CertificateKey = certKey

	data.ControlPlane.AdvertiseAddress = currentHost.GetInternalAddress()
	data.ControlPlane.BindPort = common.DefaultAPIServerPort

	var cgroupDriver string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		data.NodeRegistration.CRISocket = common.ContainerdDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		data.NodeRegistration.CRISocket = common.CRIODefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeDocker:
		data.NodeRegistration.CRISocket = common.CriDockerdSocketPath
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeIsula:
		data.NodeRegistration.CRISocket = common.IsuladDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	default:
		data.NodeRegistration.CRISocket = common.ContainerdDefaultEndpoint
		cgroupDriver = common.CgroupDriverSystemd
	}

	data.NodeRegistration.KubeletExtraArgs = map[string]string{
		"cgroup-driver": cgroupDriver,
	}
	if cluster.Spec.Kubernetes.Kubelet.ExtraArgs != nil {
		for k, v := range cluster.Spec.Kubernetes.Kubelet.ExtraArgs {
			data.NodeRegistration.KubeletExtraArgs[k] = v
		}
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

func (s *GenerateJoinMasterConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	_ = ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinMasterConfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s': %w", remoteConfigPath, err)
	}
	if !exists {
		return false, nil
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, err
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, err
	}
	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		return true, nil
	}

	return false, nil
}

func (s *GenerateJoinMasterConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, common.KubeadmJoinMasterConfigFileName)
	logger.Infof("Uploading/Updating rendered join-master config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", false); err != nil {
		return fmt.Errorf("failed to upload kubeadm config file: %w", err)
	}
	logger.Info("Kubeadm join-master configuration generated and uploaded successfully.")
	return nil
}

func (s *GenerateJoinMasterConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinMasterConfigFileName)
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateJoinMasterConfigStep)(nil)
