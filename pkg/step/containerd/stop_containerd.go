package containerd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopContainerdStep struct {
	step.Base
}

type StopContainerdStepBuilder struct {
	step.Builder[StopContainerdStepBuilder, *StopContainerdStep]
}

func NewStopContainerdStepBuilder(ctx runtime.Context, instanceName string) *StopContainerdStepBuilder {
	s := &StopContainerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop containerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopContainerdStepBuilder).Init(s)
	return b
}

func (s *StopContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check service status, assuming it's not active. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("Containerd service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("Containerd service is currently active. Step needs to run.")
	return false, nil
}

func (s *StopContainerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Stopping containerd service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		return fmt.Errorf("failed to stop containerd service: %w", err)
	}

	logger.Info("Containerd service stopped successfully.")
	return nil
}

func (s *StopContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by starting containerd service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logger.Errorf("Failed to start containerd service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StopContainerdStep)(nil)
