package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableNginxStep struct {
	step.Base
}

type EnableNginxStepBuilder struct {
	step.Builder[EnableNginxStepBuilder, *EnableNginxStep]
}

func NewEnableNginxStepBuilder(ctx runtime.Context, instanceName string) *EnableNginxStepBuilder {
	s := &EnableNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable NGINX service to start on boot", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(EnableNginxStepBuilder).Init(s)
	return b
}

func (s *EnableNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, nginxServiceName)
	if err != nil {
		logger.Warnf("Failed to check if NGINX service is enabled, assuming it needs to be enabled. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("NGINX service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("NGINX service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to enable service: %w", err)
	}

	logger.Infof("Enabling NGINX service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		return fmt.Errorf("failed to enable NGINX service: %w", err)
	}

	logger.Info("NGINX service enabled successfully.")
	return nil
}

func (s *EnableNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot disable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling NGINX service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		logger.Errorf("Failed to disable NGINX service during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*EnableNginxStep)(nil)
