package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type KubeadmVerifyEtcdClusterHealthStep struct {
	step.Base
	retryDelay time.Duration
}

type KubeadmVerifyEtcdClusterHealthStepBuilder struct {
	step.Builder[KubeadmVerifyEtcdClusterHealthStepBuilder, *KubeadmVerifyEtcdClusterHealthStep]
}

func NewKubeadmVerifyEtcdClusterHealthStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmVerifyEtcdClusterHealthStepBuilder {
	s := &KubeadmVerifyEtcdClusterHealthStep{
		retryDelay: 10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the overall health of the stacked Etcd cluster"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmVerifyEtcdClusterHealthStepBuilder).Init(s)
	return b
}

func (s *KubeadmVerifyEtcdClusterHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmVerifyEtcdClusterHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'kubectl' command is available on the remote host...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		logger.Errorf("'kubectl' command not found.")
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'kubectl' command is available.")
	return false, nil
}

func (s *KubeadmVerifyEtcdClusterHealthStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying overall Etcd cluster health...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				err := fmt.Errorf("etcd cluster health verification timed out after %v: %w", s.Base.Timeout, lastErr)
				result.MarkFailed(err, "etcd cluster health verification timed out")
				return result, err
			}
			err := fmt.Errorf("etcd cluster health verification timed out after %v", s.Base.Timeout)
			result.MarkFailed(err, "etcd cluster health verification timed out")
			return result, err
		case <-ticker.C:
			logger.Info("Attempting to verify Etcd cluster health...")

			err := s.checkClusterHealth(ctx)
			if err == nil {
				logger.Info("Etcd cluster is healthy.")
				result.MarkCompleted("etcd cluster is healthy")
				return result, nil
			}

			lastErr = err
			logger.Warnf("Etcd cluster health check failed: %v. Retrying...", err)
		}
	}
}

func (s *KubeadmVerifyEtcdClusterHealthStep) checkClusterHealth(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	nodeName := ctx.GetHost().GetName()
	etcdPodName := fmt.Sprintf("etcd-%s", nodeName)

	etcdHealthCmd := fmt.Sprintf(
		"kubectl --kubeconfig /etc/kubernetes/admin.conf exec -n kube-system %s -- "+
			"etcdctl endpoint health --cluster "+
			"--endpoints=$(etcdctl member list -w json | jq -r '[.members[] | .clientURLs[]] | join(\",\")') "+
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt "+
			"--cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt "+
			"--key=/etc/kubernetes/pki/etcd/healthcheck-client.key",
		etcdPodName,
	)

	shellCmd := fmt.Sprintf("bash -c \"%s\"", etcdHealthCmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, shellCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute 'etcdctl endpoint health': %w. Output: %s", err, runResult.Stdout)
	}

	output := runResult.Stdout

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		return fmt.Errorf("etcd health check returned empty output")
	}

	etcdNodeCount := len(ctx.GetHostsByRole(common.RoleEtcd))
	if len(lines) != etcdNodeCount {
		return fmt.Errorf("expected health status for %d etcd members, but got %d lines. Output: %s", etcdNodeCount, len(lines), output)
	}

	for _, line := range lines {
		if !strings.Contains(line, "is healthy") {
			return fmt.Errorf("found an unhealthy etcd member in cluster. Output: %s", output)
		}
	}

	return nil
}

func (s *KubeadmVerifyEtcdClusterHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmVerifyEtcdClusterHealthStep)(nil)
