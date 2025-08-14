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

type RestartNginxStepBuilder struct {
	step.Builder[RestartNginxStepBuilder, *RestartNginxStep]
}

func NewRestartNginxStepBuilder(ctx runtime.Context, instanceName string) *RestartNginxStepBuilder {
	s := &RestartNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart NGINX service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(RestartNginxStepBuilder).Init(s)
	return b
}

func (s *RestartNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartNginxStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Restarting NGINX service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		return fmt.Errorf("failed to restart NGINX service: %w", err)
	}

	logger.Info("NGINX service restarted successfully.")
	return nil
}

func (s *RestartNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback for a restart step is not applicable. No action taken.")
	return nil
}

var _ step.Step = (*RestartNginxStep)(nil)
