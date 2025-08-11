package crio

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartCrioStep struct {
	step.Base
}

type StartCrioStepBuilder struct {
	step.Builder[StartCrioStepBuilder, *StartCrioStep]
}

func NewStartCrioStepBuilder(ctx runtime.Context, instanceName string) *StartCrioStepBuilder {
	s := &StartCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(StartCrioStepBuilder).Init(s)
	return b
}

func (s *StartCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		logger.Infof("Failed to check if CRI-O service is active, assuming it's not. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("CRI-O service is already active. Step is done.")
		return true, nil
	}

	logger.Info("CRI-O service is not active. Start is required.")
	return false, nil
}

func (s *StartCrioStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to start service: %w", err)
	}

	logger.Infof("Starting CRI-O service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		return fmt.Errorf("failed to start CRI-O service: %w", err)
	}

	logger.Info("CRI-O service started successfully.")
	return nil
}

func (s *StartCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping CRI-O service")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot stop service: %v", err)
		return nil
	}

	if err := runner.StopService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		logger.Errorf("Failed to stop CRI-O service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StartCrioStep)(nil)
