package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StartDockerStep struct {
	step.Base
}

type StartDockerStepBuilder struct {
	step.Builder[StartDockerStepBuilder, *StartDockerStep]
}

func NewStartDockerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StartDockerStepBuilder {
	s := &StartDockerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start docker service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StartDockerStepBuilder).Init(s)
	return b
}

func (s *StartDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if docker service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("Docker service is already active. Step is done.")
		return true, nil
	}

	logger.Info("Docker service is not active. Step needs to run.")
	return false, nil
}

func (s *StartDockerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		result.MarkFailed(err, "failed to gather facts to start service")
		return result, fmt.Errorf("failed to gather facts to start service: %w", err)
	}

	logger.Infof("Starting docker service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", DockerServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start docker service. Recent logs:\n%s", out)

		result.MarkFailed(err, "failed to start docker service")
		return result, fmt.Errorf("failed to start docker service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		result.MarkFailed(err, "failed to verify docker service status after starting")
		return result, fmt.Errorf("failed to verify docker service status after starting: %w", err)
	}
	if !active {
		result.MarkFailed(err, "docker service did not become active after start command")
		return result, fmt.Errorf("docker service did not become active after start command")
	}

	logger.Info("Docker service started successfully.")
	result.MarkCompleted("Docker service started successfully")
	return result, nil
}

func (s *StartDockerStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by stopping docker service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logger.Errorf("Failed to stop docker service during rollback: %v", err)
	}

	return nil
}
