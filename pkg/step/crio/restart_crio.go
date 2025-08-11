package crio

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartCrioStep struct {
	step.Base
}

type RestartCrioStepBuilder struct {
	step.Builder[RestartCrioStepBuilder, *RestartCrioStep]
}

func NewRestartCrioStepBuilder(ctx runtime.Context, instanceName string) *RestartCrioStepBuilder {
	s := &RestartCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartCrioStepBuilder).Init(s)
	return b
}

func (s *RestartCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartCrioStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Restarting CRI-O service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		return fmt.Errorf("failed to restart CRI-O service: %w", err)
	}
	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify cri-o service status after restarting: %w", err)
	}
	if !active {
		return fmt.Errorf("CRI-O service did not become active after restart command")
	}
	logger.Info("CRI-O service restarted successfully.")
	return nil
}

func (s *RestartCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rollback for 'restart_crio' is a no-op.")
	return nil
}

var _ step.Step = (*RestartCrioStep)(nil)
