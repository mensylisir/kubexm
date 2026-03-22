package kubeadm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type KubeadmJoinWorkerStep struct {
	step.Base
}
type KubeadmJoinWorkerStepBuilder struct {
	step.Builder[KubeadmJoinWorkerStepBuilder, *KubeadmJoinWorkerStep]
}

func NewKubeadmJoinWorkerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmJoinWorkerStepBuilder {
	s := &KubeadmJoinWorkerStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Join a new worker node to the cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	b := new(KubeadmJoinWorkerStepBuilder).Init(s)
	return b
}
func (s *KubeadmJoinWorkerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmJoinWorkerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	kubeletConfPath := filepath.Join(common.KubernetesConfigDir, common.KubeletKubeconfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, kubeletConfPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s': %w", kubeletConfPath, err)
	}

	if exists {
		logger.Info("This node has already joined the cluster as a worker (kubelet.conf exists). Step is done.")
		return true, nil
	}

	logger.Info("This node has not joined the cluster as a worker yet. Step needs to run.")
	return false, nil
}

func (s *KubeadmJoinWorkerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinWorkerConfigFileName)
	cmd := fmt.Sprintf("kubeadm join --config %s", configPath)

	logger.Infof("Running command: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, true); err != nil {
		err = fmt.Errorf("kubeadm join worker failed: %w", err)
		result.MarkFailed(err, "kubeadm join worker failed")
		return result, err
	}

	logger.Info("Successfully joined the cluster as a worker node.")
	result.MarkCompleted("joined cluster as worker node")
	return result, nil
}

func (s *KubeadmJoinWorkerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	criSocket := getCriSocketFromSpec(ctx.GetClusterConfig())
	cmd := fmt.Sprintf("kubeadm reset --cri-socket %s --force", criSocket)

	logger.Warnf("Rolling back by running command: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, true); err != nil {
		logger.Warnf("kubeadm reset command failed, but continuing rollback: %v", err)
	} else {
		logger.Info("Kubeadm reset completed.")
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinWorkerConfigFileName)
	logger.Warnf("Rolling back by removing: %s", configPath)
	if err := runner.Remove(ctx.GoContext(), conn, configPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", configPath, err)
	}

	return nil
}

var _ step.Step = (*KubeadmJoinWorkerStep)(nil)
