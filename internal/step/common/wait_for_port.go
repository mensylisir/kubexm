package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type WaitForPortStep struct {
	step.Base
	Port int
}

type WaitForPortStepBuilder struct {
	step.Builder[WaitForPortStepBuilder, *WaitForPortStep]
}

func NewWaitForPortStepBuilder(ctx runtime.ExecutionContext, instanceName string, port int) *WaitForPortStepBuilder {
	cs := &WaitForPortStep{Port: port}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Waiting for port %d to be available", instanceName, port)
	cs.Base.Timeout = 2 * time.Minute
	return new(WaitForPortStepBuilder).Init(cs)
}

func (s *WaitForPortStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *WaitForPortStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector: %w", err)
	}

	isOpen, err := runnerSvc.IsPortOpen(ctx.GoContext(), conn, nil, s.Port)
	if err != nil {
		logger.Warnf("Port check failed, assuming port is not available. Error: %v", err)
		return false, nil
	}
	return isOpen, nil
}

func (s *WaitForPortStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("run: failed to get connector: %v", err))
		return result, err
	}

	logger.Infof("Waiting for port %d to become available (timeout: %v)...", s.Port, s.Timeout)

	err = runnerSvc.WaitForPort(ctx.GoContext(), conn, nil, s.Port, s.Timeout)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to wait for port %d: %v", s.Port, err))
		return result, err
	}
	result.MarkCompleted(fmt.Sprintf("Port %d is available", s.Port))
	return result, nil
}

func (s *WaitForPortStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for WaitForPortStep is a no-op.")
	return nil
}
