package kubexm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifyControllerManagerHealthStep struct {
	step.Base
	serviceName     string
	healthzEndpoint string
	retries         int
	retryDelay      time.Duration
}

type VerifyControllerManagerHealthStepBuilder struct {
	step.Builder[VerifyControllerManagerHealthStepBuilder, *VerifyControllerManagerHealthStep]
}

func NewVerifyControllerManagerHealthStepBuilder(ctx runtime.Context, instanceName string) *VerifyControllerManagerHealthStepBuilder {
	s := &VerifyControllerManagerHealthStep{
		serviceName:     "kube-controller-manager",
		healthzEndpoint: "http://127.0.0.1:10257/healthz",
		retries:         12, // 2 minutes
		retryDelay:      10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the kube-controller-manager service on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(VerifyControllerManagerHealthStepBuilder).Init(s)
	return b
}

func (s *VerifyControllerManagerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyControllerManagerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

func (s *VerifyControllerManagerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying kube-controller-manager health...")

	var lastErr error
	for i := 0; i < s.retries; i++ {
		log := logger.With("attempt", i+1)
		log.Infof("Attempting to verify kube-controller-manager health...")

		err := s.checkControllerManager(ctx)
		if err == nil {
			logger.Info("kube-controller-manager is healthy.")
			return nil
		}

		lastErr = err
		log.Warnf("Health check failed: %v. Retrying in %v...", err, s.retryDelay)
		time.Sleep(s.retryDelay)
	}

	logger.Errorf("kube-controller-manager did not become healthy after %d retries.", s.retries)
	return fmt.Errorf("kube-controller-manager health verification failed: %w", lastErr)
}

func (s *VerifyControllerManagerHealthStep) checkControllerManager(ctx runtime.ExecutionContext) error {
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

func (s *VerifyControllerManagerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*VerifyControllerManagerHealthStep)(nil)
