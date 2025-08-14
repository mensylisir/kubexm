package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopKeepalivedStep struct {
	step.Base
}

type StopKeepalivedStepBuilder struct {
	step.Builder[StopKeepalivedStepBuilder, *StopKeepalivedStep]
}

func NewStopKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *StopKeepalivedStepBuilder {
	s := &StopKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(StopKeepalivedStepBuilder).Init(s)
	return b
}

func (s *StopKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopKeepalivedStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, keepalivedServiceName)
	if err != nil {
		logger.Warnf("Failed to check if Keepalived service is active, assuming it needs to be stopped. Error: %v", err)
		return false, nil
	}

	if !active {
		logger.Infof("Keepalived service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("Keepalived service is active. Step needs to run to stop it.")
	return false, nil
}

func (s *StopKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to stop service: %w", err)
	}

	logger.Infof("Stopping Keepalived service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
		return fmt.Errorf("failed to stop Keepalived service: %w", err)
	}

	logger.Info("Keepalived service stopped successfully.")
	return nil
}

func (s *StopKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a stop step.")
	return nil
}

var _ step.Step = (*StopKeepalivedStep)(nil)
