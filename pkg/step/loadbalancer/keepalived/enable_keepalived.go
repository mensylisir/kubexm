package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableKeepalivedStep struct {
	step.Base
}

type EnableKeepalivedStepBuilder struct {
	step.Builder[EnableKeepalivedStepBuilder, *EnableKeepalivedStep]
}

func NewEnableKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *EnableKeepalivedStepBuilder {
	s := &EnableKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable Keepalived service to start on boot", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(EnableKeepalivedStepBuilder).Init(s)
	return b
}

func (s *EnableKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableKeepalivedStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, keepalivedServiceName)
	if err != nil {
		logger.Warnf("Failed to check if Keepalived service is enabled, assuming it needs to be enabled. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("Keepalived service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("Keepalived service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Enabling Keepalived service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
		return fmt.Errorf("failed to enable Keepalived service: %w", err)
	}

	logger.Info("Keepalived service enabled successfully.")
	return nil
}

func (s *EnableKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by disabling Keepalived service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
		logger.Errorf("Failed to disable Keepalived service during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*EnableKeepalivedStep)(nil)
