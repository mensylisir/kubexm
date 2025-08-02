package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartHAProxyStep struct {
	step.Base
}

func NewStartHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StartHAProxyStep] {
	s := &StartHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Start haproxy service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StartHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Starting haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start haproxy"); err != nil {
		return fmt.Errorf("failed to start haproxy service: %w", err)
	}

	logger.Info("haproxy service started successfully.")
	return nil
}

func (s *StartHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func (s *StartHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
