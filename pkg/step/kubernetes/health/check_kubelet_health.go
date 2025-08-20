package health

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifyKubeletHealthStep struct {
	step.Base
	serviceName     string
	healthzEndpoint string
	retryDelay      time.Duration
}

type VerifyKubeletHealthStepBuilder struct {
	step.Builder[VerifyKubeletHealthStepBuilder, *VerifyKubeletHealthStep]
}

func NewVerifyKubeletHealthStepBuilder(ctx runtime.Context, instanceName string) *VerifyKubeletHealthStepBuilder {
	s := &VerifyKubeletHealthStep{
		serviceName:     "kubelet",
		healthzEndpoint: "http://127.0.0.1:10248/healthz",
		retryDelay:      10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the kubelet service on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(VerifyKubeletHealthStepBuilder).Init(s)
	return b
}

func (s *VerifyKubeletHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyKubeletHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

func (s *VerifyKubeletHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying kubelet health...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("kubelet health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("kubelet health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Info("Attempting to verify kubelet health...")

			err := s.checkKubelet(ctx)
			if err == nil {
				logger.Info("kubelet is healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Health check failed: %v. Retrying...", err)
		}
	}
}

func (s *VerifyKubeletHealthStep) checkKubelet(ctx runtime.ExecutionContext) error {
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

func (s *VerifyKubeletHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*VerifyKubeletHealthStep)(nil)
