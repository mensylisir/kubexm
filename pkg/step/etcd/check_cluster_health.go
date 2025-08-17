package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EtcdVerifyClusterHealthStep struct {
	step.Base
	remoteCertsDir string
	retryDelay     time.Duration
}

type EtcdVerifyClusterHealthStepBuilder struct {
	step.Builder[EtcdVerifyClusterHealthStepBuilder, *EtcdVerifyClusterHealthStep]
}

func NewEtcdVerifyClusterHealthStepBuilder(ctx runtime.Context, instanceName string) *EtcdVerifyClusterHealthStepBuilder {
	s := &EtcdVerifyClusterHealthStep{
		remoteCertsDir: common.DefaultEtcdPKIDir,
		retryDelay:     10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the entire etcd cluster"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(EtcdVerifyClusterHealthStepBuilder).Init(s)
	return b
}

func (s *EtcdVerifyClusterHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdVerifyClusterHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for etcd cluster health verification...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v etcdctl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'etcdctl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'etcdctl' command is available.")
	return false, nil
}

func (s *EtcdVerifyClusterHealthStep) Run(ctx runtime.ExecutionContext) error {
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
				return fmt.Errorf("etcd cluster health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("etcd cluster health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Infof("Attempting to verify Etcd cluster health...")

			err := s.checkClusterHealth(ctx)
			if err == nil {
				logger.Info("Etcd cluster is fully healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Cluster health check failed: %v. Retrying...", err)
		}
	}
}

func (s *EtcdVerifyClusterHealthStep) checkClusterHealth(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	nodeName := ctx.GetHost().GetName()
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)

	baseEtcdctlCmd := fmt.Sprintf("etcdctl --cacert=%s --cert=%s --key=%s",
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	listEndpointsCmd := fmt.Sprintf("%s --endpoints=https://127.0.0.1:2379 member list -w simple | awk -F', ' '{print $5}' | paste -sd, -", baseEtcdctlCmd)

	shellCmd := fmt.Sprintf("bash -c \"%s\"", listEndpointsCmd)
	stdout, err := runner.Run(ctx.GoContext(), conn, shellCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to get etcd member endpoints: %w. Output: %s", err, string(stdout))
	}
	endpoints := strings.TrimSpace(string(stdout))
	if endpoints == "" {
		return fmt.Errorf("could not determine any etcd endpoints from member list")
	}

	healthCmd := fmt.Sprintf("%s --endpoints=%s endpoint health --cluster", baseEtcdctlCmd, endpoints)

	stdout, err = runner.Run(ctx.GoContext(), conn, healthCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("cluster health check command failed: %w. Output: %s", err, string(stdout))
	}

	output := string(stdout)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != len(etcdNodes) {
		return fmt.Errorf("expected health status for %d members, but got %d lines. Output: %s", len(etcdNodes), len(lines), output)
	}

	for _, line := range lines {
		if !strings.Contains(line, "is healthy") {
			return fmt.Errorf("found an unhealthy etcd member. Output:\n%s", output)
		}
	}

	return nil
}

func (s *EtcdVerifyClusterHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*EtcdVerifyClusterHealthStep)(nil)
