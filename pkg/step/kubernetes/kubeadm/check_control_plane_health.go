package kubeadm

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmVerifyControlPlaneHealthStep struct {
	step.Base
	components  map[string]string
	retryDelay  time.Duration
	maxRestarts int
}

type KubeadmVerifyControlPlaneHealthStepBuilder struct {
	step.Builder[KubeadmVerifyControlPlaneHealthStepBuilder, *KubeadmVerifyControlPlaneHealthStep]
}

func NewKubeadmVerifyControlPlaneHealthStepBuilder(ctx runtime.Context, instanceName string) *KubeadmVerifyControlPlaneHealthStepBuilder {
	s := &KubeadmVerifyControlPlaneHealthStep{
		components: map[string]string{
			"kube-apiserver":          "kube-apiserver",
			"kube-controller-manager": "kube-controller-manager",
			"kube-scheduler":          "kube-scheduler",
		},
		retryDelay:  10 * time.Second,
		maxRestarts: 2,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify health of local control plane pods via container runtime"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmVerifyControlPlaneHealthStepBuilder).Init(s)
	return b
}

func (s *KubeadmVerifyControlPlaneHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmVerifyControlPlaneHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'crictl' command is available on the remote host...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v crictl", s.Sudo); err != nil {
		logger.Errorf("'crictl' command not found.")
		return false, fmt.Errorf("precheck failed: 'crictl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'crictl' command is available.")
	return false, nil
}

func (s *KubeadmVerifyControlPlaneHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying local control plane health via container runtime...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("control plane health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("control plane health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Info("Attempting to verify control plane pods...")

			err := s.checkPodsHealth(ctx)
			if err == nil {
				logger.Info("All control plane pods are running and healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Health check failed: %v. Retrying...", err)
		}
	}
}

func (s *KubeadmVerifyControlPlaneHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

func (s *KubeadmVerifyControlPlaneHealthStep) checkPodsHealth(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	for component, namePattern := range s.components {
		cmd := fmt.Sprintf("crictl ps --name %s --no-trunc", namePattern)

		stdout, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
		if err != nil {
			return fmt.Errorf("failed to list containers for component '%s': %w", component, err)
		}

		output := string(stdout)
		if !strings.Contains(output, namePattern) {
			return fmt.Errorf("container for component '%s' not found", component)
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 2 {
			return fmt.Errorf("no container entry found in output for '%s'", component)
		}

		header := strings.Fields(lines[0])
		stateIdx, attemptIdx := -1, -1
		for i, h := range header {
			if h == "STATE" {
				stateIdx = i
			} else if h == "ATTEMPTS" {
				attemptIdx = i
			}
		}
		if stateIdx == -1 || attemptIdx == -1 {
			return fmt.Errorf("could not parse 'crictl ps' output for '%s': missing STATE or ATTEMPTS column", component)
		}

		fields := strings.Fields(lines[1])
		if len(fields) <= stateIdx || len(fields) <= attemptIdx {
			return fmt.Errorf("malformed 'crictl ps' output for '%s'", component)
		}

		state := fields[stateIdx]
		if state != "Running" {
			return fmt.Errorf("container for component '%s' is in state '%s', expected 'Running'", component, state)
		}

		restarts, err := strconv.Atoi(fields[attemptIdx])
		if err != nil {
			return fmt.Errorf("failed to parse restart count for '%s': %w", component, err)
		}
		if restarts > s.maxRestarts {
			return fmt.Errorf("container for component '%s' has restarted %d times (max allowed: %d), indicating a crash loop", component, restarts, s.maxRestarts)
		}
	}

	return nil
}

var _ step.Step = (*KubeadmVerifyControlPlaneHealthStep)(nil)
