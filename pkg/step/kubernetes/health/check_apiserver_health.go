package health

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifyAPIServerHealthStep struct {
	step.Base
	serviceName     string
	healthzEndpoint string
	securePort      string
	retryDelay      time.Duration
}

type VerifyAPIServerHealthStepBuilder struct {
	step.Builder[VerifyAPIServerHealthStepBuilder, *VerifyAPIServerHealthStep]
}

func NewVerifyAPIServerHealthStepBuilder(ctx runtime.Context, instanceName string) *VerifyAPIServerHealthStepBuilder {
	s := &VerifyAPIServerHealthStep{
		serviceName:     "kube-apiserver",
		healthzEndpoint: "https://127.0.0.1:6443/healthz",
		securePort:      "6443",
		retryDelay:      10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the kube-apiserver service on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(VerifyAPIServerHealthStepBuilder).Init(s)
	return b
}

func (s *VerifyAPIServerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyAPIServerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying required commands are available...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v systemctl && command -v ss && command -v curl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: one or more required commands (systemctl, ss, curl) not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: all required commands are available.")
	return false, nil
}

func (s *VerifyAPIServerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying kube-apiserver health...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("kube-apiserver health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("kube-apiserver health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Info("Attempting to verify kube-apiserver health...")

			err := s.checkAPIServer(ctx)
			if err == nil {
				logger.Info("kube-apiserver is healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Health check failed: %v. Retrying...", err)
		}
	}
}

func (s *VerifyAPIServerHealthStep) checkAPIServer(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	isActiveCmd := fmt.Sprintf("systemctl is-active %s", s.serviceName)
	stdout, err := runner.Run(ctx.GoContext(), conn, isActiveCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("systemd service '%s' is not active. Status: %s", s.serviceName, string(stdout))
	}
	if strings.TrimSpace(string(stdout)) != "active" {
		return fmt.Errorf("systemd service '%s' status is '%s', expected 'active'", s.serviceName, string(stdout))
	}

	checkPortCmd := fmt.Sprintf("ss -lntp | grep -q ':%s.*kube-apiserver'", s.securePort)
	if _, err := runner.Run(ctx.GoContext(), conn, checkPortCmd, s.Sudo); err != nil {
		return fmt.Errorf("kube-apiserver process is not listening on secure port %s", s.securePort)
	}

	caPath := "/etc/kubernetes/ssl/ca.crt"
	curlCmd := fmt.Sprintf("curl --cacert %s --resolve kubernetes.default.svc:%s:127.0.0.1 %s", caPath, s.securePort, s.healthzEndpoint)
	stdout, err = runner.Run(ctx.GoContext(), conn, curlCmd, false)
	if err != nil {
		return fmt.Errorf("failed to connect to healthz endpoint '%s': %w", s.healthzEndpoint, err)
	}
	if strings.TrimSpace(string(stdout)) != "ok" {
		return fmt.Errorf("healthz endpoint returned '%s', expected 'ok'", string(stdout))
	}

	return nil
}

func (s *VerifyAPIServerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*VerifyAPIServerHealthStep)(nil)
