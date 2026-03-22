package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StopDockerStep struct {
	step.Base
}

type StopDockerStepBuilder struct {
	step.Builder[StopDockerStepBuilder, *StopDockerStep]
}

func NewStopDockerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopDockerStepBuilder {
	s := &StopDockerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop docker service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopDockerStepBuilder).Init(s)
	return b
}

func (s *StopDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		logger.Warnf("Failed to check service status, assuming it's not active. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("Docker service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("Docker service is currently active. Step needs to run.")
	return false, nil
}

func (s *StopDockerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		result.MarkFailed(err, "failed to gather facts")
		return result, err
	}

	logger.Infof("Stopping docker service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		result.MarkFailed(err, "failed to stop docker service")
		return result, fmt.Errorf("failed to stop docker service: %w", err)
	}

	logger.Info("Docker service stopped successfully.")
	result.MarkCompleted("Docker service stopped successfully")
	return result, nil
}

func (s *StopDockerStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by starting docker service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logger.Errorf("Failed to start docker service during rollback: %v", err)
	}

	return nil
}
