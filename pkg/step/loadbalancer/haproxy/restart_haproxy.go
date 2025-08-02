package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartHAProxyStep struct {
	step.Base
}

func NewRestartHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *RestartHAProxyStep] {
	s := &RestartHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart haproxy service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *RestartHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Restarting haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl restart haproxy"); err != nil {
		return fmt.Errorf("failed to restart haproxy service: %w", err)
	}

	logger.Info("haproxy service restarted successfully.")
	return nil
}

func (s *RestartHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func (s *RestartHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
