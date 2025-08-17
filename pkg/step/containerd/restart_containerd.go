package containerd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartContainerdStep struct {
	step.Base
}

type RestartContainerdStepBuilder struct {
	step.Builder[RestartContainerdStepBuilder, *RestartContainerdStep]
}

func NewRestartContainerdStepBuilder(ctx runtime.Context, instanceName string) *RestartContainerdStepBuilder {
	s := &RestartContainerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart containerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartContainerdStepBuilder).Init(s)
	return b
}

func (s *RestartContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Restart step will always run if scheduled.")
	return false, nil
}

func (s *RestartContainerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Info("Restarting containerd service.")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, containerdServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", containerdServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Error(err, "Failed to restart containerd service.", "logs", out)
		return fmt.Errorf("failed to restart containerd service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, containerdServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify containerd service status after restarting: %w", err)
	}
	if !active {
		return fmt.Errorf("containerd service did not become active after restart command")
	}

	logger.Info("Containerd service restarted successfully.")
	return nil
}

func (s *RestartContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}

var _ step.Step = (*RestartContainerdStep)(nil)
