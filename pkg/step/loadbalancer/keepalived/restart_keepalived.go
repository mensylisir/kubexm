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

func NewRestartKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *RestartKeepalivedStep] {
	s := &RestartKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart keepalived service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *RestartKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Restarting keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl restart keepalived"); err != nil {
		return fmt.Errorf("failed to restart keepalived service: %w", err)
	}

	logger.Info("keepalived service restarted successfully.")
	return nil
}

func (s *RestartKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	// It's hard to define a meaningful rollback for a restart action.
	// Maybe try to restart again? Or stop it?
	// For now, we do nothing, assuming the service is in a reasonable state.
	return nil
}

func (s *RestartKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
