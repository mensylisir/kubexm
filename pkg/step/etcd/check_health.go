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

type CheckEtcdHealthStep struct {
	step.Base
	EtcdctlBinaryPath string
	CACertPath        string
	CertPath          string
	KeyPath           string
	RetryCount        int
	RetryDelay        time.Duration
}

type CheckEtcdHealthStepBuilder struct {
	step.Builder[CheckEtcdHealthStepBuilder, *CheckEtcdHealthStep]
}

func NewCheckEtcdHealthStepBuilder(ctx runtime.Context, instanceName string) *CheckEtcdHealthStepBuilder {
	s := &CheckEtcdHealthStep{
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
		CACertPath:        filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName),
		CertPath:          "",
		KeyPath:           "",
		RetryCount:        10,
		RetryDelay:        6 * time.Second,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check etcd cluster health status from current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CheckEtcdHealthStepBuilder).Init(s)
	return b
}

func (b *CheckEtcdHealthStepBuilder) WithRetryCount(count int) *CheckEtcdHealthStepBuilder {
	b.Step.RetryCount = count
	return b
}

func (b *CheckEtcdHealthStepBuilder) WithRetryDelay(delay time.Duration) *CheckEtcdHealthStepBuilder {
	b.Step.RetryDelay = delay
	return b
}

func (s *CheckEtcdHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckEtcdHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CheckEtcdHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	currentHost := ctx.GetHost()
	nodeName := currentHost.GetName()

	logger.Info("Performing etcd cluster health check from this node...", "node", nodeName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	s.CertPath = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName))
	s.KeyPath = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName))

	// 构建 etcdctl 命令
	cmd := fmt.Sprintf("ETCDCTL_API=3 %s endpoint health --cluster --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath,
		s.CACertPath,
		s.CertPath,
		s.KeyPath,
	)

	var lastErr error
	for i := 0; i < s.RetryCount; i++ {
		if i > 0 {
			time.Sleep(s.RetryDelay)
		}

		logger.Info("Attempting to check etcd health...", "attempt", i+1)

		stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
		if err != nil {
			lastErr = fmt.Errorf("etcd health check command failed: %w, stderr: %s", err, stderr)
			logger.Warn("Health check attempt failed.", "error", lastErr)
			continue
		}

		if isClusterHealthy(stdout) {
			logger.Info("Etcd cluster is healthy.", "output", stdout)
			return nil
		}

		lastErr = fmt.Errorf("etcd cluster is not healthy. Output: %s", stdout)
		logger.Warn("Cluster is not fully healthy yet, will retry...", "output", stdout)
	}

	logger.Error(lastErr, "Etcd cluster health check failed after all retries.")
	return lastErr
}

func (s *CheckEtcdHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func isClusterHealthy(output string) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return false
	}

	healthyEndpoints := 0
	for _, line := range lines {
		if strings.Contains(line, "is healthy") {
			healthyEndpoints++
		}
	}
	return healthyEndpoints > 0 && healthyEndpoints == len(lines)
}

var _ step.Step = (*CheckEtcdHealthStep)(nil)
