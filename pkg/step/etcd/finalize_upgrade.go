// FILE: pkg/etcd/step_finalize_upgrade.go

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

type EtcdFinalizeUpgradeStep struct {
	step.Base
	remoteCertsDir string
	targetVersion  string
}

type EtcdFinalizeUpgradeStepBuilder struct {
	step.Builder[EtcdFinalizeUpgradeStepBuilder, *EtcdFinalizeUpgradeStep]
}

func NewEtcdFinalizeUpgradeStepBuilder(ctx runtime.Context, instanceName string) *EtcdFinalizeUpgradeStepBuilder {
	s := &EtcdFinalizeUpgradeStep{
		remoteCertsDir: common.DefaultEtcdPKIDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Finalize the etcd cluster upgrade by setting the cluster version (IRREVERSIBLE)"
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(EtcdFinalizeUpgradeStepBuilder).Init(s)
	return b
}

func (s *EtcdFinalizeUpgradeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdFinalizeUpgradeStep) getTargetVersionFromBinary(ctx runtime.ExecutionContext) (string, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", err
	}

	versionCmd := "etcd --version | grep 'etcd version' | awk '{print $3}'"
	stdout, err := runner.Run(ctx.GoContext(), conn, versionCmd, false)
	if err != nil {
		return "", fmt.Errorf("failed to determine etcd binary version: %w", err)
	}

	version := strings.TrimSpace(string(stdout))
	if version == "" {
		return "", fmt.Errorf("could not parse etcd version from binary")
	}

	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid etcd version format: %s", version)
	}

	return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
}

func (s *EtcdFinalizeUpgradeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for etcd upgrade finalization...")

	targetVersion, err := s.getTargetVersionFromBinary(ctx)
	if err != nil {
		return false, fmt.Errorf("precheck failed to determine target etcd version: %w", err)
	}
	s.targetVersion = targetVersion
	logger.Infof("Determined target etcd cluster version to be: %s", s.targetVersion)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	nodeName := ctx.GetHost().GetName()
	baseEtcdctlCmd := fmt.Sprintf("etcdctl --cacert=%s --cert=%s --key=%s --endpoints=https://127.0.0.1:2379",
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	statusCmd := fmt.Sprintf("%s endpoint status -w simple | grep 'Cluster-Version' | awk '{print $2}'", baseEtcdctlCmd)
	shellCmd := fmt.Sprintf("bash -c \"%s\"", statusCmd)

	stdout, err := runner.Run(ctx.GoContext(), conn, shellCmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("precheck failed to get current etcd cluster version: %w", err)
	}

	currentVersion := strings.TrimSpace(string(stdout))
	logger.Infof("Current etcd cluster version is: %s", currentVersion)

	if currentVersion == s.targetVersion {
		logger.Info("Etcd cluster version is already at the target version. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: Etcd cluster version needs to be updated.")
	return false, nil
}

func (s *EtcdFinalizeUpgradeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Warn("!!!!!!!!!! WARNING !!!!!!!!!!!")
	logger.Warn("This step is IRREVERSIBLE. Once the cluster version is set, you CANNOT downgrade.")
	logger.Warnf("Setting etcd cluster version to: %s", s.targetVersion)

	time.Sleep(5 * time.Second)

	nodeName := ctx.GetHost().GetName()
	baseEtcdctlCmd := fmt.Sprintf("etcdctl --cacert=%s --cert=%s --key=%s --endpoints=https://127.0.0.1:2379",
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	finalizeCmd := fmt.Sprintf("%s cluster-version set %s", baseEtcdctlCmd, s.targetVersion)

	if _, err := runner.Run(ctx.GoContext(), conn, finalizeCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to set etcd cluster version: %w", err)
	}

	logger.Info("Successfully finalized the etcd cluster upgrade.")
	return nil
}

func (s *EtcdFinalizeUpgradeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Error("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	logger.Error("ROLLBACK IS NOT POSSIBLE FOR THIS STEP.")
	logger.Error("The etcd cluster version has been permanently upgraded.")
	logger.Error("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	return nil
}

var _ step.Step = (*EtcdFinalizeUpgradeStep)(nil)
