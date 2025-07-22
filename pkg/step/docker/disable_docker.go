package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableDockerStep struct {
	step.Base
}

type DisableDockerStepBuilder struct {
	step.Builder[DisableDockerStepBuilder, *DisableDockerStep]
}

func NewDisableDockerStepBuilder(ctx runtime.Context, instanceName string) *DisableDockerStepBuilder {
	s := &DisableDockerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable docker service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableDockerStepBuilder).Init(s)
	return b
}

func (s *DisableDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service is enabled, assuming it's disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("Docker service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("Docker service is currently enabled. Step needs to run.")
	return false, nil
}

func (s *DisableDockerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Disabling docker service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		return fmt.Errorf("failed to disable docker service: %w", err)
	}

	logger.Info("Docker service disabled successfully.")
	return nil
}

func (s *DisableDockerStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by enabling docker service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logger.Errorf("Failed to enable docker service during rollback: %v", err)
	}

	return nil
}
