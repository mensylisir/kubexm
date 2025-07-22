package containerd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableContainerdStep struct {
	step.Base
}

type DisableContainerdStepBuilder struct {
	step.Builder[DisableContainerdStepBuilder, *DisableContainerdStep]
}

func NewDisableContainerdStepBuilder(ctx runtime.Context, instanceName string) *DisableContainerdStepBuilder {
	s := &DisableContainerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable containerd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableContainerdStepBuilder).Init(s)
	return b
}

func (s *DisableContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service is enabled, assuming it's disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("Containerd service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("Containerd service is currently enabled. Step needs to run.")
	return false, nil
}

func (s *DisableContainerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Disabling containerd service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		return fmt.Errorf("failed to disable containerd service: %w", err)
	}

	logger.Info("Containerd service disabled successfully.")
	return nil
}

func (s *DisableContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by enabling containerd service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logger.Errorf("Failed to enable containerd service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*DisableContainerdStep)(nil)
