package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartKeepalivedStep struct {
	step.Base
}

func NewStartKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StartKeepalivedStep] {
	s := &StartKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Start keepalived service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StartKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Starting keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start keepalived"); err != nil {
		return fmt.Errorf("failed to start keepalived service: %w", err)
	}

	logger.Info("keepalived service started successfully.")
	return nil
}

func (s *StartKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	// No rollback action for starting a service.
	// The service will be stopped by the StopKeepalivedStep's rollback if needed.
	return nil
}

func (s *StartKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
