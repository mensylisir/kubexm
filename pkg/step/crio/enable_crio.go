package crio

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const crioServiceName = "crio.service"

type EnableCrioStep struct {
	step.Base
}

type EnableCrioStepBuilder struct {
	step.Builder[EnableCrioStepBuilder, *EnableCrioStep]
}

func NewEnableCrioStepBuilder(ctx runtime.Context, instanceName string) *EnableCrioStepBuilder {
	s := &EnableCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableCrioStepBuilder).Init(s)
	return b
}

func (s *EnableCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		logger.Infof("Failed to check if CRI-O service is enabled, assuming it's not. Error: %v", err)
		return false, nil
	}
	if enabled {
		logger.Infof("Cri-o service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("CRI-O service status is '%s'. Enable is required.")
	return false, nil
}

func (s *EnableCrioStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Enabling CRI-O service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		return fmt.Errorf("failed to enable cri-o service: %w", err)
	}

	logger.Info("CRI-O service enabled successfully.")
	return nil
}

func (s *EnableCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling CRI-O service")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot disable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling cri-o service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		logger.Errorf("Failed to disable cri-o service during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*EnableCrioStep)(nil)
