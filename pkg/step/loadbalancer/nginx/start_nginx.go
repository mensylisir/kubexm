package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const nginxServiceName = "nginx"

type StartNginxStep struct {
	step.Base
}

type StartNginxStepBuilder struct {
	step.Builder[StartNginxStepBuilder, *StartNginxStep]
}

func NewStartNginxStepBuilder(ctx runtime.Context, instanceName string) *StartNginxStepBuilder {
	s := &StartNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start NGINX service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(StartNginxStepBuilder).Init(s)
	return b
}

func (s *StartNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, nginxServiceName)
	if err != nil {
		logger.Warnf("Failed to check if NGINX service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("NGINX service is already active. Step is done.")
		return true, nil
	}

	logger.Info("NGINX service is not active. Step needs to run.")
	return false, nil
}

func (s *StartNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to start service: %w", err)
	}

	logger.Infof("Starting NGINX service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", nginxServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start NGINX service. Recent logs:\n%s", out)
		return fmt.Errorf("failed to start NGINX service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, nginxServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify NGINX service status after starting: %w", err)
	}
	if !active {
		return fmt.Errorf("NGINX service did not become active after start command")
	}

	logger.Info("NGINX service started successfully.")
	return nil
}

func (s *StartNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot stop service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping NGINX service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		logger.Errorf("Failed to stop NGINX service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StartNginxStep)(nil)
