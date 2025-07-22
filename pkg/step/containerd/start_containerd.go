package containerd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartContainerdStep struct {
	step.Base
}

type StartContainerdStepBuilder struct {
	step.Builder[StartContainerdStepBuilder, *StartContainerdStep]
}

func NewStartContainerdStepBuilder(ctx runtime.Context, instanceName string) *StartContainerdStepBuilder {
	s := &StartContainerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start containerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StartContainerdStepBuilder).Init(s)
	return b
}

func (s *StartContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check if containerd service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("Containerd service is already active. Step is done.")
		return true, nil
	}

	logger.Info("Containerd service is not active. Step needs to run.")
	return false, nil
}

func (s *StartContainerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Starting containerd service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", containerdServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start containerd service. Recent logs:\n%s", out)

		return fmt.Errorf("failed to start containerd service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify containerd service status after starting: %w", err)
	}
	if !active {
		return fmt.Errorf("containerd service did not become active after start command")
	}

	logger.Info("Containerd service started successfully.")
	return nil
}

func (s *StartContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot stop service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping containerd service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logger.Errorf("Failed to stop containerd service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StartContainerdStep)(nil)
