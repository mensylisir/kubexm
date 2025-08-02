package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartNginxStep struct {
	step.Base
}

func NewStartNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StartNginxStep] {
	s := &StartNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Start nginx service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StartNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Starting nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start nginx"); err != nil {
		return fmt.Errorf("failed to start nginx service: %w", err)
	}

	logger.Info("nginx service started successfully.")
	return nil
}

func (s *StartNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func (s *StartNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
