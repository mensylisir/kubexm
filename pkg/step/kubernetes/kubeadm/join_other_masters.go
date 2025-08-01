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

// JoinOtherMastersStep is a step to run kubeadm join on other master nodes.
type JoinOtherMastersStep struct {
	step.Base
}

// JoinOtherMastersStepBuilder is a builder for JoinOtherMastersStep.
type JoinOtherMastersStepBuilder struct {
	step.Builder[JoinOtherMastersStepBuilder, *JoinOtherMastersStep]
}

// NewJoinOtherMastersStepBuilder creates a new JoinOtherMastersStepBuilder.
func NewJoinOtherMastersStepBuilder(ctx runtime.Context, instanceName string) *JoinOtherMastersStepBuilder {
	s := &JoinOtherMastersStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm join on other master nodes", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute
	b := new(JoinOtherMastersStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *JoinOtherMastersStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck checks if the node has already joined the cluster.
func (s *JoinOtherMastersStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
func (s *JoinOtherMastersStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, KubeadmJoinMasterConfigFileName)
	cmd := fmt.Sprintf("kubeadm join --config %s", configPath)

	logger.Info("Running kubeadm join for master node...")
	output, err := runner.SudoExec(ctx.GoContext(), conn, cmd)
	if err != nil {
		return fmt.Errorf("failed to run kubeadm join for master: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Kubeadm join for master completed successfully.")
	logger.Debugf("Kubeadm join output:\n%s", string(output))
	return nil
}

// Rollback runs kubeadm reset.
func (s *JoinOtherMastersStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*JoinOtherMastersStep)(nil)
