package kubeadm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// JoinWorkersStep is a step to run kubeadm join on worker nodes.
type JoinWorkersStep struct {
	step.Base
}

// JoinWorkersStepBuilder is a builder for JoinWorkersStep.
type JoinWorkersStepBuilder struct {
	step.Builder[JoinWorkersStepBuilder, *JoinWorkersStep]
}

// NewJoinWorkersStepBuilder creates a new JoinWorkersStepBuilder.
func NewJoinWorkersStepBuilder(ctx runtime.Context, instanceName string) *JoinWorkersStepBuilder {
	s := &JoinWorkersStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm join on worker nodes", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	b := new(JoinWorkersStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *JoinWorkersStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck checks if the node has already joined the cluster.
func (s *JoinWorkersStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// If kubelet.conf exists, we assume the node has joined.
	kubeletConfigPath := filepath.Join(common.KubernetesConfigDir, "kubelet.conf")
	exists, err := runner.Exists(ctx.GoContext(), conn, kubeletConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", kubeletConfigPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Info("Kubelet config file already exists. Step is done.")
		return true, nil
	}
	logger.Info("Kubelet config file does not exist. Step needs to run.")
	return false, nil
}

// Run executes the kubeadm join command.
func (s *JoinWorkersStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinWorkerConfigFileName)
	cmd := fmt.Sprintf("kubeadm join --config %s", configPath)

	logger.Info("Running kubeadm join for worker node...")
	output, err := runner.SudoExec(ctx.GoContext(), conn, cmd)
	if err != nil {
		return fmt.Errorf("failed to run kubeadm join for worker: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Kubeadm join for worker completed successfully.")
	logger.Debugf("Kubeadm join output:\n%s", string(output))
	return nil
}

// Rollback runs kubeadm reset.
func (s *JoinWorkersStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	cmd := "kubeadm reset -f"
	logger.Warnf("Rolling back by running '%s'", cmd)
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		logger.Errorf("Failed to run '%s' during rollback: %v", cmd, err)
	}

	return nil
}

var _ step.Step = (*JoinWorkersStep)(nil)
