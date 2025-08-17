package containerd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	containerdServiceName = "containerd"
)

type EnableContainerdStep struct {
	step.Base
}

type EnableContainerdStepBuilder struct {
	step.Builder[EnableContainerdStepBuilder, *EnableContainerdStep]
}

func NewEnableContainerdStepBuilder(ctx runtime.Context, instanceName string) *EnableContainerdStepBuilder {
	s := &EnableContainerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable containerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableContainerdStepBuilder).Init(s)
	return b
}

func (s *EnableContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		logger.Warn(err, "Failed to check if containerd service is enabled, assuming it needs to be.")
		return false, nil
	}

	if enabled {
		logger.Info("Containerd service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("Containerd service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableContainerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Info("Enabling containerd service.")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		return fmt.Errorf("failed to enable containerd service: %w", err)
	}

	logger.Info("Containerd service enabled successfully.")
	return nil
}

func (s *EnableContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Error(err, "Failed to gather facts for rollback, cannot disable service.")
		return nil
	}

	logger.Warn("Rolling back by disabling containerd service.")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logger.Error(err, "Failed to disable containerd service during rollback.")
	}

	return nil
}

var _ step.Step = (*EnableContainerdStep)(nil)
