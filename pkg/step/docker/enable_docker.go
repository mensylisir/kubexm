package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	DockerServiceName = "docker"
)

type EnableDockerStep struct {
	step.Base
}

type EnableDockerStepBuilder struct {
	step.Builder[EnableDockerStepBuilder, *EnableDockerStep]
}

func NewEnableDockerStepBuilder(ctx runtime.Context, instanceName string) *EnableDockerStepBuilder {
	s := &EnableDockerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable docker service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableDockerStepBuilder).Init(s)
	return b
}

func (s *EnableDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if docker service is enabled, assuming it needs to be. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("Docker service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("Docker service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableDockerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Enabling docker service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		return fmt.Errorf("failed to enable docker service: %w", err)
	}

	logger.Info("Docker service enabled successfully.")
	return nil
}

func (s *EnableDockerStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by disabling docker service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logger.Errorf("Failed to disable docker service during rollback: %v", err)
	}

	return nil
}
