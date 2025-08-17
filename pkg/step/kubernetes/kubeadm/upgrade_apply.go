package kubeadm

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	CacheKeyKubeadmBackupPath = "kubeadm_upgrade_backup_path_%s" // %s = hostname
)

type KubeadmUpgradeApplyStep struct {
	step.Base
}

type KubeadmUpgradeApplyStepBuilder struct {
	step.Builder[KubeadmUpgradeApplyStepBuilder, *KubeadmUpgradeApplyStep]
}

func NewKubeadmUpgradeApplyStepBuilder(ctx runtime.Context, instanceName string) *KubeadmUpgradeApplyStepBuilder {
	s := &KubeadmUpgradeApplyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Apply the Kubernetes control plane upgrade using 'kubeadm upgrade apply'"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(KubeadmUpgradeApplyStepBuilder).Init(s)
	return b
}

func (s *KubeadmUpgradeApplyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmUpgradeApplyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for kubeadm upgrade apply...")
	_, ok := ctx.GetTaskCache().Get(CacheKeyTargetVersion)
	if !ok {
		return false, fmt.Errorf("precheck failed: target version not found in cache. The 'upgrade plan' step must run first")
	}

	logger.Debug("Assuming this step is running on the first master node to be upgraded.")
	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmUpgradeApplyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetVersion, ok := ctx.GetTaskCache().Get(CacheKeyTargetVersion)
	if !ok {
		return fmt.Errorf("could not retrieve target version from cache")
	}
	versionStr, ok := targetVersion.(string)
	if !ok || versionStr == "" {
		return fmt.Errorf("invalid target version in cache")
	}

	logger.Infof("Applying control plane upgrade to version %s...", versionStr)
	applyCmd := fmt.Sprintf("kubeadm upgrade apply %s --yes --etcd-upgrade=false", versionStr)

	stdout, err := runner.Run(ctx.GoContext(), conn, applyCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("'kubeadm upgrade apply' failed. Output:\n%s\nError: %w", string(stdout), err)
	}

	output := string(stdout)
	logger.Infof("`kubeadm upgrade apply` output:\n%s", output)
	
	re := regexp.MustCompile(`located at (/etc/kubernetes/tmp/kubeadm-backup-[a-z0-9-]+)`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		logger.Warn("Could not automatically detect the backup path from kubeadm output. Manual rollback might be required if subsequent steps fail.")
	} else {
		backupPath := matches[1]
		cacheKey := fmt.Sprintf(CacheKeyKubeadmBackupPath, ctx.GetHost().GetName())
		ctx.GetTaskCache().Set(cacheKey, backupPath)
		logger.Infof("Successfully detected and saved kubeadm backup path to cache: %s (key: %s)", backupPath, cacheKey)
	}

	if !strings.Contains(output, "SUCCESS! Your Kubernetes control plane has been upgraded successfully!") {
		return fmt.Errorf("could not find final success message in 'kubeadm upgrade apply' output. The upgrade might be incomplete")
	}

	logger.Info("Control plane upgrade applied successfully.")
	return nil
}

func (s *KubeadmUpgradeApplyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for 'kubeadm upgrade apply' is not performed by this step.")
	logger.Warn("A separate 'RestoreKubeadmBackupStep' is required to restore the backup created by kubeadm.")
	logger.Warn("If needed, find the backup path in the task logs or cache and restore it manually on the host.")
	return nil
}

var _ step.Step = (*KubeadmUpgradeApplyStep)(nil)
