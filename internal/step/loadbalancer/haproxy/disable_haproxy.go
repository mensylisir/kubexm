package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type DisableHAProxyStep struct {
	step.Base
}

type DisableHAProxyStepBuilder struct {
	step.Builder[DisableHAProxyStepBuilder, *DisableHAProxyStep]
}

func NewDisableHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableHAProxyStepBuilder {
	s := &DisableHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable HAProxy service from starting on boot", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(DisableHAProxyStepBuilder).Init(s)
	return b
}

func (s *DisableHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, haproxyServiceName)
	if err != nil {
		logger.Warnf("Failed to check if HAProxy service is enabled, assuming it needs to be disabled. Error: %v", err)
		return false, nil
	}

	if !enabled {
		logger.Infof("HAProxy service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("HAProxy service is enabled. Step needs to run to disable it.")
	return false, nil
}

func (s *DisableHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		result.MarkFailed(err, "failed to gather facts to disable service")
		return result, err
	}

	logger.Infof("Disabling HAProxy service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		result.MarkFailed(err, "failed to disable HAProxy service")
		return result, err
	}

	logger.Info("HAProxy service disabled successfully.")
	result.MarkCompleted("HAProxy service disabled successfully")
	return result, nil
}

func (s *DisableHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a disable step.")
	return nil
}

var _ step.Step = (*DisableHAProxyStep)(nil)
