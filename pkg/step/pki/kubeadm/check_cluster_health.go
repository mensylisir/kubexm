// FILE: pkg/kubeadm/step_verify_cluster_health.go

package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmVerifyClusterHealthStep struct {
	step.Base
	retries    int
	retryDelay time.Duration
}

type KubeadmVerifyClusterHealthStepBuilder struct {
	step.Builder[KubeadmVerifyClusterHealthStepBuilder, *KubeadmVerifyClusterHealthStep]
}

func NewKubeadmVerifyClusterHealthStepBuilder(ctx runtime.Context, instanceName string) *KubeadmVerifyClusterHealthStepBuilder {
	s := &KubeadmVerifyClusterHealthStep{
		retries:    12,
		retryDelay: 10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify overall cluster health from the current master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmVerifyClusterHealthStepBuilder).Init(s)
	return b
}

func (s *KubeadmVerifyClusterHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmVerifyClusterHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'kubectl' command is available...")

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

func (s *KubeadmVerifyClusterHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Performing comprehensive cluster health verification from this node...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	var lastErr error
	for i := 0; i < s.retries; i++ {
		log := logger.With("attempt", i+1)
		log.Infof("Attempting to verify cluster health...")

		err := s.checkClusterHealth(ctx, conn)
		if err == nil {
			logger.Info("Cluster is fully healthy and operational.")
			return nil
		}

		lastErr = err
		log.Warnf("Cluster health check failed: %v. Retrying in %v...", err, s.retryDelay)
		time.Sleep(s.retryDelay)
	}

	return fmt.Errorf("cluster did not pass health checks after multiple retries: %w", lastErr)
}

func (s *KubeadmVerifyClusterHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step.")
	return nil
}

func (s *KubeadmVerifyClusterHealthStep) checkClusterHealth(ctx runtime.ExecutionContext, conn connector.Connector) error {
	runner := ctx.GetRunner()
	logger := ctx.GetLogger()
	currentHostName := ctx.GetHost().GetName()

	logger.Info("Checking node status...")
	getNodesCmd := "kubectl --kubeconfig /etc/kubernetes/admin.conf get nodes --no-headers"
	logger.Debugf("Executing command: %s", getNodesCmd)
	stdout, err := runner.Run(ctx.GoContext(), conn, getNodesCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute 'kubectl get nodes': %w. Output: %s", err, string(stdout))
	}
	for _, line := range strings.Split(string(stdout), "\n") {
		if strings.Contains(line, "NotReady") {
			return fmt.Errorf("found a node in 'NotReady' state: %s", line)
		}
	}
	logger.Info("Check passed: All nodes appear to be Ready.")

	logger.Info("Checking kube-system pods...")
	getPodsCmd := "kubectl --kubeconfig /etc/kubernetes/admin.conf get pods -n kube-system --no-headers"
	logger.Debugf("Executing command: %s", getPodsCmd)
	stdout, err = runner.Run(ctx.GoContext(), conn, getPodsCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute 'kubectl get pods': %w. Output: %s", err, string(stdout))
	}
	for _, line := range strings.Split(string(stdout), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		status := fields[2]
		if status != "Running" && status != "Completed" {
			return fmt.Errorf("found non-running/completed pod in kube-system: %s", line)
		}
	}
	logger.Info("Check passed: All pods in kube-system are Running or Succeeded.")

	logger.Info("Checking etcd cluster health...")
	etcdPodName := fmt.Sprintf("etcd-%s", currentHostName)
	etcdHealthCmd := fmt.Sprintf(
		"kubectl --kubeconfig /etc/kubernetes/admin.conf exec -n kube-system %s -- etcdctl endpoint health --cluster --endpoints=https://127.0.0.1:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt --key=/etc/kubernetes/pki/etcd/healthcheck-client.key",
		etcdPodName,
	)
	logger.Debugf("Executing command for etcd health on pod %s", etcdPodName)
	stdout, err = runner.Run(ctx.GoContext(), conn, etcdHealthCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute etcd health check: %w. Output: %s", err, string(stdout))
	}
	if !strings.Contains(string(stdout), "is healthy") {
		return fmt.Errorf("etcd cluster health check failed, output: %s", string(stdout))
	}
	logger.Info("Check passed: Etcd cluster is healthy.")

	return nil
}

var _ step.Step = (*KubeadmVerifyClusterHealthStep)(nil)
