package kubeadm

import (
	"bytes"
	"fmt"
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

type GenerateJoinWorkerConfigStep struct {
	step.Base
}
type GenerateJoinWorkerConfigStepBuilder struct {
	step.Builder[GenerateJoinWorkerConfigStepBuilder, *GenerateJoinWorkerConfigStep]
}

func NewGenerateJoinWorkerConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateJoinWorkerConfigStepBuilder {
	s := &GenerateJoinWorkerConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm join configuration for worker", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateJoinWorkerConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateJoinWorkerConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type JoinWorkerTemplateData struct {
	Discovery        DiscoveryTemplate
	NodeRegistration NodeRegistrationTemplate
}

func (s *GenerateJoinWorkerConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()

	data := JoinWorkerTemplateData{}

	data.Discovery.APIServerEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, cluster.Spec.ControlPlaneEndpoint.Port)
	// IMPORTANT: Use fixed task name "KubeadmInit" for cache keys, not the current task name.
	// The token is written by BootstrapFirstMasterTask.KubeadmInit step,
	// and read by JoinWorkersTask.GenerateJoinWorkerConfig step.
	cacheKey := fmt.Sprintf(common.CacheKubeadmInitToken, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), "KubeadmInit")
	tokenVal, found := ctx.GetTaskCache().Get(cacheKey)
	if !found {
		return nil, fmt.Errorf("bootstrap token not found in task cache with key '%s' (did KubeadmInit step run successfully?)", cacheKey)
	}
	token, ok := tokenVal.(string)
	if !ok {
		return nil, fmt.Errorf("cached bootstrap token is not a string")
	}
	data.Discovery.BootstrapToken = token

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

	templateContent, err := templates.Get("kubernetes/kubeadm-join-worker-config.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeadm join worker template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubeadm join worker template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateJoinWorkerConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	_ = ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinWorkerConfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, err
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

func (s *GenerateJoinWorkerConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render join worker config")
		return result, err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		err = fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
		result.MarkFailed(err, "failed to create remote directory")
		return result, err
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, common.KubeadmJoinWorkerConfigFileName)
	logger.Infof("Uploading/Updating rendered join-worker config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", false); err != nil {
		err = fmt.Errorf("failed to upload kubeadm config file: %w", err)
		result.MarkFailed(err, "failed to upload kubeadm config file")
		return result, err
	}
	logger.Info("Kubeadm join-worker configuration generated and uploaded successfully.")
	result.MarkCompleted("kubeadm join-worker config generated and uploaded successfully")
	return result, nil
}

func (s *GenerateJoinWorkerConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinWorkerConfigFileName)
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateJoinWorkerConfigStep)(nil)
