package health

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifySchedulerHealthStep struct {
	step.Base
	serviceName     string
	healthzEndpoint string
	retries         int
	retryDelay      time.Duration
}

type VerifySchedulerHealthStepBuilder struct {
	step.Builder[VerifySchedulerHealthStepBuilder, *VerifySchedulerHealthStep]
}

func NewVerifySchedulerHealthStepBuilder(ctx runtime.Context, instanceName string) *VerifySchedulerHealthStepBuilder {
	s := &VerifySchedulerHealthStep{
		serviceName:     "kube-scheduler",
		healthzEndpoint: "http://127.0.0.1:10259/healthz",
		retries:         12, // 2 minutes
		retryDelay:      10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the kube-scheduler service on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(VerifySchedulerHealthStepBuilder).Init(s)
	return b
}

func (s *VerifySchedulerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifySchedulerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying required commands are available...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v systemctl && command -v curl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'systemctl' or 'curl' not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: all required commands are available.")
	return false, nil
}

func (s *VerifySchedulerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying kube-scheduler health...")

	var lastErr error
	for i := 0; i < s.retries; i++ {
		log := logger.With("attempt", i+1)
		log.Infof("Attempting to verify kube-scheduler health...")

		err := s.checkScheduler(ctx)
		if err == nil {
			logger.Info("kube-scheduler is healthy.")
			return nil
		}

		lastErr = err
		log.Warnf("Health check failed: %v. Retrying in %v...", err, s.retryDelay)
		time.Sleep(s.retryDelay)
	}

	logger.Errorf("kube-scheduler did not become healthy after %d retries.", s.retries)
	return fmt.Errorf("kube-scheduler health verification failed: %w", lastErr)
}

func (s *VerifySchedulerHealthStep) checkScheduler(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	isActiveCmd := fmt.Sprintf("systemctl is-active %s", s.serviceName)
	stdout, err := runner.Run(ctx.GoContext(), conn, isActiveCmd, false)
	if err != nil || strings.TrimSpace(string(stdout)) != "active" {
		status := "inactive"
		if err == nil {
			status = string(stdout)
		}
		return fmt.Errorf("systemd service '%s' is not active. Status: %s", s.serviceName, status)
	}

	curlCmd := fmt.Sprintf("curl -s --fail %s", s.healthzEndpoint)
	stdout, err = runner.Run(ctx.GoContext(), conn, curlCmd, false)
	if err != nil {
		return fmt.Errorf("failed to connect to healthz endpoint '%s': %w", s.healthzEndpoint, err)
	}
	if strings.TrimSpace(string(stdout)) != "ok" {
		return fmt.Errorf("healthz endpoint returned '%s', expected 'ok'", string(stdout))
	}

	return nil
}

func (s *VerifySchedulerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*VerifySchedulerHealthStep)(nil)
