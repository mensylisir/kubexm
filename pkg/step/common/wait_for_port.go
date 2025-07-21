package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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

func (s *WaitForPortStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector: %w", err)
	}

	logger.Infof("Waiting for port %d to become available (timeout: %v)...", s.Port, s.Timeout)

	return runnerSvc.WaitForPort(ctx.GoContext(), conn, nil, s.Port, s.Timeout)
}

func (s *WaitForPortStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for WaitForPortStep is a no-op.")
	return nil
}
