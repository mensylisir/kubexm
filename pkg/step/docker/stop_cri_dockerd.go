package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopCriDockerdStep struct {
	step.Base
}

type StopCriDockerdStepBuilder struct {
	step.Builder[StopCriDockerdStepBuilder, *StopCriDockerdStep]
}

func NewStopCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *StopCriDockerdStepBuilder {
	s := &StopCriDockerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop cri-dockerd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopCriDockerdStepBuilder).Init(s)
	return b
}

func (s *StopCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check service status, assuming it's not active. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("CriDockerd service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("CriDockerd service is currently active. Step needs to run.")
	return false, nil
}

func (s *StopCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return err
	}

	logger.Infof("Stopping cri-dockerd service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		return fmt.Errorf("failed to stop cri-dockerd service: %w", err)
	}

	logger.Info("CriDockerd service stopped successfully.")
	return nil
}

func (s *StopCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot start service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by starting cri-dockerd service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logger.Errorf("Failed to start cri-dockerd service during rollback: %v", err)
	}

	return nil
}
