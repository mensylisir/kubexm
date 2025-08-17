package kube_scheduler

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckSchedulerHealthStep struct {
	step.Base
	HealthCheckURL string
	RetryDelay     time.Duration
}

type CheckSchedulerHealthStepBuilder struct {
	step.Builder[CheckSchedulerHealthStepBuilder, *CheckSchedulerHealthStep]
}

func NewCheckSchedulerHealthStepBuilder(ctx runtime.Context, instanceName string) *CheckSchedulerHealthStepBuilder {
	s := &CheckSchedulerHealthStep{
		HealthCheckURL: "https://127.0.0.1:10259/healthz",
		RetryDelay:     5 * time.Second,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check kube-scheduler health on localhost", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CheckSchedulerHealthStepBuilder).Init(s)
	return b
}

func (s *CheckSchedulerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckSchedulerHealthStep) checkHealth(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	cmd := fmt.Sprintf("curl -k -s -o /dev/null -w '%%{http_code}' %s", s.HealthCheckURL)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("health check command failed: %w, stderr: %s", err, stderr)
	}

	if stdout == "200" {
		return true, nil
	}

	return false, fmt.Errorf("health check failed with status code: %s", stdout)
}

func (s *CheckSchedulerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	healthy, err := s.checkHealth(ctx)
	if err != nil {
		logger.Infof("Precheck: Kube Scheduler is not yet healthy. Step needs to run. (Error: %v)", err)
		return false, nil
	}

	if healthy {
		logger.Info("Precheck: Kube Scheduler is already healthy. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *CheckSchedulerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Waiting for kube-scheduler to be healthy...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.RetryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("kube-scheduler health check timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("kube-scheduler health check timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			healthy, err := s.checkHealth(ctx)
			if healthy {
				logger.Info("kube-scheduler is healthy!")
				return nil
			}
			if err != nil {
				lastErr = err
				logger.Debugf("Health check attempt failed: %v", err)
			}
		}
	}
}

func (s *CheckSchedulerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Check health step has no rollback action.")
	return nil
}

var _ step.Step = (*CheckSchedulerHealthStep)(nil)
