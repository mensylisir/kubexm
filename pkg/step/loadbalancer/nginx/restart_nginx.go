package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartNginxStep struct {
	step.Base
}

func NewRestartNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *RestartNginxStep] {
	s := &RestartNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart nginx service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *RestartNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Restarting nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl restart nginx"); err != nil {
		return fmt.Errorf("failed to restart nginx service: %w", err)
	}

	logger.Info("nginx service restarted successfully.")
	return nil
}

func (s *RestartNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func (s *RestartNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
