package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartCriDockerdStep struct {
	step.Base
}

type StartCriDockerdStepBuilder struct {
	step.Builder[StartCriDockerdStepBuilder, *StartCriDockerdStep]
}

func NewStartCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *StartCriDockerdStepBuilder {
	s := &StartCriDockerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start cri-dockerd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StartCriDockerdStepBuilder).Init(s)
	return b
}

func (s *StartCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		logger.Warnf("Failed to check if cri-dockerd service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("CriDockerd service is already active. Step is done.")
		return true, nil
	}

	logger.Info("CriDockerd service is not active. Step needs to run.")
	return false, nil
}

func (s *StartCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Starting cri-dockerd service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", CriDockerdServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start cri-dockerd service. Recent logs:\n%s", out)

		return fmt.Errorf("failed to start cri-dockerd service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify cri-dockerd service status after starting: %w", err)
	}
	if !active {
		return fmt.Errorf("cri-dockerd service did not become active after start command")
	}

	logger.Info("CriDockerd service started successfully.")
	return nil
}

func (s *StartCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by stopping cri-dockerd service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logger.Errorf("Failed to stop cri-dockerd service during rollback: %v", err)
	}

	return nil
}
