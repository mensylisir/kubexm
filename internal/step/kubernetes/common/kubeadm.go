package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// KubeadmInitStep runs kubeadm init on the control plane node.
type KubeadmInitStep struct {
	step.Base
	InitConfigPath string
	SkipPreflight  bool
}

type KubeadmInitStepBuilder struct {
	step.Builder[KubeadmInitStepBuilder, *KubeadmInitStep]
}

func NewKubeadmInitStepBuilder(ctx runtime.ExecutionContext, instanceName, initConfigPath string) *KubeadmInitStepBuilder {
	s := &KubeadmInitStep{
		InitConfigPath: initConfigPath,
		SkipPreflight:  false,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm init", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(KubeadmInitStepBuilder).Init(s)
}

func (s *KubeadmInitStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmInitStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *KubeadmInitStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	initCmd := "kubeadm init"
	if s.InitConfigPath != "" {
		initCmd = fmt.Sprintf("kubeadm init --config=%s", s.InitConfigPath)
	}
	if s.SkipPreflight {
		initCmd += " --skip-preflight-checks"
	}

	logger.Infof("Running: %s", initCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, initCmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "kubeadm init failed")
		return result, err
	}

	logger.Infof("kubeadm init completed successfully")
	result.MarkCompleted("kubeadm init completed")
	return result, nil
}

func (s *KubeadmInitStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by running kubeadm reset")
	runner.Run(ctx.GoContext(), conn, "kubeadm reset -f", false)
	return nil
}

var _ step.Step = (*KubeadmInitStep)(nil)

// KubeadmJoinStep joins a node to the cluster using kubeadm join.
type KubeadmJoinStep struct {
	step.Base
	JoinConfigPath string
	NodeType       string // "control-plane" or "worker"
}

type KubeadmJoinStepBuilder struct {
	step.Builder[KubeadmJoinStepBuilder, *KubeadmJoinStep]
}

func NewKubeadmJoinStepBuilder(ctx runtime.ExecutionContext, instanceName, joinConfigPath, nodeType string) *KubeadmJoinStepBuilder {
	s := &KubeadmJoinStep{
		JoinConfigPath: joinConfigPath,
		NodeType:       nodeType,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm join as %s", instanceName, nodeType)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(KubeadmJoinStepBuilder).Init(s)
}

func (s *KubeadmJoinStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmJoinStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *KubeadmJoinStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	joinCmd := "kubeadm join"
	if s.JoinConfigPath != "" {
		joinCmd = fmt.Sprintf("kubeadm join --config=%s", s.JoinConfigPath)
	}

	logger.Infof("Running: %s", joinCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, joinCmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "kubeadm join failed")
		return result, err
	}

	logger.Infof("kubeadm join completed successfully")
	result.MarkCompleted("kubeadm join completed")
	return result, nil
}

func (s *KubeadmJoinStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by running kubeadm reset")
	runner.Run(ctx.GoContext(), conn, "kubeadm reset -f", false)
	return nil
}

var _ step.Step = (*KubeadmJoinStep)(nil)

// KubeadmResetStep runs kubeadm reset on a node.
type KubeadmResetStep struct {
	step.Base
	Force bool
}

type KubeadmResetStepBuilder struct {
	step.Builder[KubeadmResetStepBuilder, *KubeadmResetStep]
}

func NewKubeadmResetStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmResetStepBuilder {
	s := &KubeadmResetStep{
		Force: true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm reset", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = true
	s.Base.Timeout = 2 * time.Minute
	return new(KubeadmResetStepBuilder).Init(s)
}

func (s *KubeadmResetStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmResetStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *KubeadmResetStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	resetCmd := "kubeadm reset -f"
	logger.Infof("Running: %s", resetCmd)
	runner.Run(ctx.GoContext(), conn, resetCmd, s.Base.Sudo)

	logger.Infof("kubeadm reset completed")
	result.MarkCompleted("kubeadm reset completed")
	return result, nil
}

func (s *KubeadmResetStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*KubeadmResetStep)(nil)
