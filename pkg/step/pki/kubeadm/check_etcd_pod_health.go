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

type KubeadmVerifyEtcdPodHealthStep struct {
	step.Base
	retries     int
	retryDelay  time.Duration
	maxRestarts int
}

type KubeadmVerifyEtcdPodHealthStepBuilder struct {
	step.Builder[KubeadmVerifyEtcdPodHealthStepBuilder, *KubeadmVerifyEtcdPodHealthStep]
}

func NewKubeadmVerifyEtcdPodHealthStepBuilder(ctx runtime.Context, instanceName string) *KubeadmVerifyEtcdPodHealthStepBuilder {
	s := &KubeadmVerifyEtcdPodHealthStep{
		retries:     18,
		retryDelay:  10 * time.Second,
		maxRestarts: 2,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the local etcd pod via container runtime"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmVerifyEtcdPodHealthStepBuilder).Init(s)
	return b
}

func (s *KubeadmVerifyEtcdPodHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmVerifyEtcdPodHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

func (s *KubeadmVerifyEtcdPodHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying local etcd pod health via container runtime...")

	var lastErr error
	for i := 0; i < s.retries; i++ {
		log := logger.With("attempt", i+1)
		log.Infof("Attempting to verify etcd pod container status...")

		err := s.checkPodHealth(ctx)
		if err == nil {
			logger.Info("Etcd pod container is Running and stable.")
			return nil
		}

		lastErr = err
		log.Warnf("Etcd pod health check failed: %v. Retrying in %v...", err, s.retryDelay)
		time.Sleep(s.retryDelay)
	}

	logger.Errorf("Etcd pod did not become healthy after %d retries.", s.retries)
	return fmt.Errorf("etcd pod health verification failed: %w", lastErr)
}

func (s *KubeadmVerifyEtcdPodHealthStep) checkPodHealth(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	nodeName := ctx.GetHost().GetName()
	etcdContainerNamePattern := fmt.Sprintf("k8s_etcd_etcd-%s_kube-system", nodeName)
	cmd := fmt.Sprintf("crictl ps --name %s --no-trunc", etcdContainerNamePattern)

	stdout, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to list etcd container: %w. Output: %s", err, string(stdout))
	}

	output := string(stdout)
	if !strings.Contains(output, "etcd") {
		return fmt.Errorf("etcd container not found using pattern '%s'", etcdContainerNamePattern)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("no container entry found for etcd in 'crictl ps' output")
	}

	header := strings.Fields(lines[0])
	stateIdx, attemptIdx := -1, -1
	for i, h := range header {
		if h == "STATE" {
			stateIdx = i
		}
		if h == "ATTEMPTS" {
			attemptIdx = i
		}
	}
	if stateIdx == -1 || attemptIdx == -1 {
		return fmt.Errorf("could not parse 'crictl ps' header: missing STATE or ATTEMPTS column. Header: %s", lines[0])
	}

	fields := strings.Fields(lines[1])
	if len(fields) <= stateIdx || len(fields) <= attemptIdx {
		return fmt.Errorf("malformed 'crictl ps' output line for etcd: %s", lines[1])
	}

	state := fields[stateIdx]
	if state != "Running" {
		return fmt.Errorf("etcd container is in state '%s', expected 'Running'", state)
	}

	restarts, err := strconv.Atoi(fields[attemptIdx])
	if err != nil {
		return fmt.Errorf("failed to parse restart count ('%s') for etcd container: %w", fields[attemptIdx], err)
	}
	if restarts > s.maxRestarts {
		return fmt.Errorf("etcd container has restarted %d times (max allowed: %d), indicating a crash loop", restarts, s.maxRestarts)
	}

	return nil
}

func (s *KubeadmVerifyEtcdPodHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmVerifyEtcdPodHealthStep)(nil)
