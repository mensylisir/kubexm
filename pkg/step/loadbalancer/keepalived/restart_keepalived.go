package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartKeepalivedStep struct {
	step.Base
}

type RestartKeepalivedStepBuilder struct {
	step.Builder[RestartKeepalivedStepBuilder, *RestartKeepalivedStep]
}

func NewRestartKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *RestartKeepalivedStepBuilder {
	s := &RestartKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(RestartKeepalivedStepBuilder).Init(s)
	return b
}

func (s *RestartKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKeepalivedStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Restarting Keepalived service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
		return fmt.Errorf("failed to restart Keepalived service: %w", err)
	}

	logger.Info("Keepalived service restarted successfully.")
	return nil
}

func (s *RestartKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback for a restart step is not applicable. No action taken.")
	return nil
}

var _ step.Step = (*RestartKeepalivedStep)(nil)
