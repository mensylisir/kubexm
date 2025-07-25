package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartDockerStep struct {
	step.Base
}

type RestartDockerStepBuilder struct {
	step.Builder[RestartDockerStepBuilder, *RestartDockerStep]
}

func NewRestartDockerStepBuilder(ctx runtime.Context, instanceName string) *RestartDockerStepBuilder {
	s := &RestartDockerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart docker service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartDockerStepBuilder).Init(s)
	return b
}

func (s *RestartDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to restart service: %w", err)
	}

	logger.Infof("Restarting docker service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, DockerServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", DockerServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart docker service. Recent logs:\n%s", out)
		return fmt.Errorf("failed to restart docker service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, DockerServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify docker service status after restarting: %w", err)
	}
	if !active {
		return fmt.Errorf("docker service did not become active after restart command")
	}

	logger.Info("Docker service restarted successfully.")
	return nil
}

func (s *RestartDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}
