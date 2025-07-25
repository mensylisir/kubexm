package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	CriDockerdServiceName = "cri-dockerd"
)

type EnableCriDockerdStep struct {
	step.Base
}

type EnableCriDockerdStepBuilder struct {
	step.Builder[EnableCriDockerdStepBuilder, *EnableCriDockerdStep]
}

func NewEnableCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *EnableCriDockerdStepBuilder {
	s := &EnableCriDockerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable cri-dockerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableCriDockerdStepBuilder).Init(s)
	return b
}

func (s *EnableCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check if cri-dockerd service is enabled, assuming it needs to be. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("CriDockerd service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("CriDockerd service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to enable service: %w", err)
	}

	logger.Infof("Enabling cri-dockerd service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		return fmt.Errorf("failed to enable cri-dockerd service: %w", err)
	}

	logger.Info("CriDockerd service enabled successfully.")
	return nil
}

func (s *EnableCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot disable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling cri-dockerd service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logger.Errorf("Failed to disable cri-dockerd service during rollback: %v", err)
	}

	return nil
}
