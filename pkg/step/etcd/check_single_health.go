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

type EtcdVerifyMemberHealthStep struct {
	step.Base
	remoteCertsDir string
	endpoint       string
	retryDelay     time.Duration
}

type EtcdVerifyMemberHealthStepBuilder struct {
	step.Builder[EtcdVerifyMemberHealthStepBuilder, *EtcdVerifyMemberHealthStep]
}

func NewEtcdVerifyMemberHealthStepBuilder(ctx runtime.Context, instanceName string) *EtcdVerifyMemberHealthStepBuilder {
	s := &EtcdVerifyMemberHealthStep{
		remoteCertsDir: common.DefaultEtcdPKIDir,
		endpoint:       "https://127.0.0.1:2379",
		retryDelay:     10 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify the health of the local etcd member"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(EtcdVerifyMemberHealthStepBuilder).Init(s)
	return b
}

func (s *EtcdVerifyMemberHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdVerifyMemberHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for etcd member health verification...")

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

func (s *EtcdVerifyMemberHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Verifying local etcd member health...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("etcd member health verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("etcd member health verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			logger.Infof("Attempting to verify etcd member health at %s...", s.endpoint)

			err := s.checkMemberHealth(ctx)
			if err == nil {
				logger.Info("Local etcd member is healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Health check failed: %v. Retrying...", err)
		}
	}
}

func (s *EtcdVerifyMemberHealthStep) checkMemberHealth(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	nodeName := ctx.GetHost().GetName()

	healthCmd := fmt.Sprintf("etcdctl endpoint health --endpoints=%s "+
		"--cacert=%s "+
		"--cert=%s "+
		"--key=%s",
		s.endpoint,
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	stdout, err := runner.Run(ctx.GoContext(), conn, healthCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("health check command failed for endpoint '%s': %w. Output: %s", s.endpoint, err, string(stdout))
	}

	output := string(stdout)
	
	if !strings.Contains(output, "is healthy") {
		return fmt.Errorf("endpoint '%s' reported as unhealthy. Output: %s", s.endpoint, output)
	}

	return nil
}

func (s *EtcdVerifyMemberHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*EtcdVerifyMemberHealthStep)(nil)
