package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableCriDockerdStep struct {
	step.Base
}

type DisableCriDockerdStepBuilder struct {
	step.Builder[DisableCriDockerdStepBuilder, *DisableCriDockerdStep]
}

func NewDisableCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *DisableCriDockerdStepBuilder {
	s := &DisableCriDockerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable cri-dockerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableCriDockerdStepBuilder).Init(s)
	return b
}

func (s *DisableCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service is enabled, assuming it's disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("CriDockerd service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("CriDockerd service is currently enabled. Step needs to run.")
	return false, nil
}

func (s *DisableCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Disabling cri-dockerd service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		return fmt.Errorf("failed to disable cri-dockerd service: %w", err)
	}

	logger.Info("CriDockerd service disabled successfully.")
	return nil
}

func (s *DisableCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot enable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by enabling cri-dockerd service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logger.Errorf("Failed to enable cri-dockerd service during rollback: %v", err)
	}

	return nil
}
