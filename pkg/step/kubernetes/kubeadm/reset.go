package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmResetStep struct {
	step.Base
}

type KubeadmResetStepBuilder struct {
	step.Builder[KubeadmResetStepBuilder, *KubeadmResetStep]
}

func NewKubeadmResetStepBuilder(ctx runtime.Context, instanceName string) *KubeadmResetStepBuilder {
	s := &KubeadmResetStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Reset node with kubeadm", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmResetStepBuilder).Init(s)
	return b
}

func (s *KubeadmResetStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmResetStep) getCriSocketFromSpec(cluster *v1alpha1.Cluster) string {
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		if cluster.Spec.Kubernetes.ContainerRuntime.Containerd.Endpoint == "" {
			return common.ContainerdDefaultEndpoint
		}
		return cluster.Spec.Kubernetes.ContainerRuntime.Containerd.Endpoint
	case common.RuntimeTypeCRIO:
		if cluster.Spec.Kubernetes.ContainerRuntime.Crio.Endpoint == "" {
			return common.CRIODefaultEndpoint
		}
		return cluster.Spec.Kubernetes.ContainerRuntime.Crio.Endpoint
	case common.RuntimeTypeDocker:
		if cluster.Spec.Kubernetes.ContainerRuntime.Docker.Endpoint == "" {
			return common.CriDockerdSocketPath
		}
		return cluster.Spec.Kubernetes.ContainerRuntime.Docker.Endpoint
	case common.RuntimeTypeIsula:
		if cluster.Spec.Kubernetes.ContainerRuntime.Isulad.Endpoint == "" {
			return common.IsuladDefaultEndpoint
		}
		return cluster.Spec.Kubernetes.ContainerRuntime.Isulad.Endpoint
	default:
		return common.ContainerdDefaultEndpoint
	}
}

func (s *KubeadmResetStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, common.KubernetesConfigDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for directory '%s': %w", common.KubernetesConfigDir, err)
	}

	if !exists {
		logger.Info("Kubernetes config directory does not exist. Node is considered reset. Step is done.")
		return true, nil
	}

	logger.Info("Kubernetes config directory exists. Kubeadm reset is required.")
	return false, nil
}

func (s *KubeadmResetStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	criSocket := s.getCriSocketFromSpec(ctx.GetClusterConfig())

	cmd := fmt.Sprintf("kubeadm reset --force --cri-socket %s", criSocket)

	logger.Warnf("Running command to reset node: %s", cmd)

	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Warnf("'kubeadm reset' command failed, but continuing cleanup. This might be expected if the node was already partially cleaned. Error: %v, Output: \n%s", err, output)
	} else {
		logger.Info("Kubeadm reset completed successfully.")
	}

	logger.Info("Performing additional cleanup of common Kubernetes directories...")
	dirsToClean := []string{
		"/var/lib/kubelet",
		"~/.kube",
	}
	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		dirsToClean = append(dirsToClean, "/var/lib/etcd")
	}
	for _, dir := range dirsToClean {
		exists, _ := runner.Exists(ctx.GoContext(), conn, dir)
		if exists {
			logger.Warnf("Forcefully removing directory: %s", dir)
			if err := runner.Remove(ctx.GoContext(), conn, dir, s.Sudo, true); err != nil {
				if !strings.Contains(err.Error(), "no such file or directory") {
					logger.Warnf("Failed to remove directory '%s': %v", dir, err)
				}
			}
		}
	}

	return nil
}

func (s *KubeadmResetStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Reset step has no rollback action.")
	return nil
}

var _ step.Step = (*KubeadmResetStep)(nil)
