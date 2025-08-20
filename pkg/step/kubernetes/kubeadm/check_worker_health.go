package kubeadm

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmVerifyWorkerHealthStep struct {
	step.Base
	kubeletServiceName string
	kubeProxyPodName   string
	retryDelay         time.Duration
	maxRestarts        int
}

type KubeadmVerifyWorkerHealthStepBuilder struct {
	step.Builder[KubeadmVerifyWorkerHealthStepBuilder, *KubeadmVerifyWorkerHealthStep]
}

func NewKubeadmVerifyWorkerHealthStepBuilder(ctx runtime.Context, instanceName string) *KubeadmVerifyWorkerHealthStepBuilder {
	s := &KubeadmVerifyWorkerHealthStep{
		kubeletServiceName: "kubelet.service",
		kubeProxyPodName:   "kube-proxy",
		retryDelay:         10 * time.Second,
		maxRestarts:        3,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify health of worker node components (kubelet, kube-proxy)"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmVerifyWorkerHealthStepBuilder).Init(s)
	return b
}

func (s *KubeadmVerifyWorkerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmVerifyWorkerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'systemctl' and 'crictl' commands are available...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runner := ctx.GetRunner()

	if _, err := runner.Run(ctx.GoContext(), conn, "command -v systemctl", s.Sudo); err != nil {
		logger.Errorf("'systemctl' command not found.")
		return false, fmt.Errorf("precheck failed: 'systemctl' not found on host '%s'", ctx.GetHost().GetName())
	}

	if _, err := runner.Run(ctx.GoContext(), conn, "command -v crictl", s.Sudo); err != nil {
		logger.Errorf("'crictl' command not found.")
		return false, fmt.Errorf("precheck failed: 'crictl' not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'systemctl' and 'crictl' commands are available.")
	return false, nil
}

func (s *KubeadmVerifyWorkerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying worker node component health...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("worker health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("worker health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Info("Attempting to verify worker components...")

			err := s.checkWorkerHealth(ctx)
			if err == nil {
				logger.Info("All worker components are healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Health check failed: %v. Retrying in %v...", err, s.retryDelay)
		}
	}
}

func (s *KubeadmVerifyWorkerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

// checkWorkerHealth performs the actual checks for kubelet and kube-proxy.
func (s *KubeadmVerifyWorkerHealthStep) checkWorkerHealth(ctx runtime.ExecutionContext) error {
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := s.checkKubeletHealth(ctx, conn); err != nil {
		return err
	}

	if err := s.checkKubeProxyHealth(ctx, conn); err != nil {
		return err
	}

	return nil
}

func (s *KubeadmVerifyWorkerHealthStep) checkKubeletHealth(ctx runtime.ExecutionContext, conn connector.Connector) error {
	logger := ctx.GetLogger().With("component", s.kubeletServiceName)
	runner := ctx.GetRunner()
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather host facts for kubelet check: %w", err)
	}

	logger.Info("Checking kubelet service status...")

	isActive, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.kubeletServiceName)
	if err != nil {
		return fmt.Errorf("failed to check if kubelet service is active: %w", err)
	}
	if !isActive {
		return fmt.Errorf("kubelet service '%s' is not active", s.kubeletServiceName)
	}
	logger.Debug("Kubelet service is active.")

	isFailedCmd := fmt.Sprintf("systemctl is-failed %s", s.kubeletServiceName)
	if _, err := runner.Run(ctx.GoContext(), conn, isFailedCmd, s.Sudo); err == nil {
		return fmt.Errorf("kubelet service '%s' is in a 'failed' state", s.kubeletServiceName)
	}
	logger.Debug("Kubelet service is not in a failed state.")

	logger.Info("Kubelet service is active and not in a failed state.")
	return nil
}
func (s *KubeadmVerifyWorkerHealthStep) checkKubeProxyHealth(ctx runtime.ExecutionContext, conn connector.Connector) error {
	logger := ctx.GetLogger().With("component", s.kubeProxyPodName)
	runner := ctx.GetRunner()

	logger.Info("Checking kube-proxy container status...")

	cmd := fmt.Sprintf("crictl ps --name %s --no-trunc", s.kubeProxyPodName)
	stdout, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to list containers for kube-proxy: %w", err)
	}

	output := string(stdout)
	if !strings.Contains(output, s.kubeProxyPodName) {
		return fmt.Errorf("container for kube-proxy not found")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("no container entry found in crictl output for kube-proxy")
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
		return fmt.Errorf("could not parse 'crictl ps' output for kube-proxy: missing STATE or ATTEMPTS column")
	}

	fields := strings.Fields(lines[1])
	if len(fields) <= stateIdx || len(fields) <= attemptIdx {
		return fmt.Errorf("malformed 'crictl ps' output for kube-proxy")
	}

	state := fields[stateIdx]
	if state != "Running" {
		return fmt.Errorf("container for kube-proxy is in state '%s', expected 'Running'", state)
	}

	restarts, err := strconv.Atoi(fields[attemptIdx])
	if err != nil {
		return fmt.Errorf("failed to parse restart count for kube-proxy: %w", err)
	}
	if restarts > s.maxRestarts {
		return fmt.Errorf("container for kube-proxy has restarted %d times (max allowed: %d), indicating a crash loop", restarts, s.maxRestarts)
	}

	logger.Info("Kube-proxy container is Running and has not restarted excessively.")
	return nil
}

var _ step.Step = (*KubeadmVerifyWorkerHealthStep)(nil)
