package kubeadm

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"k8s.io/apimachinery/pkg/util/version"
)

type KubeadmUpgradeNodeStep struct {
	step.Base
}

type KubeadmUpgradeNodeStepBuilder struct {
	step.Builder[KubeadmUpgradeNodeStepBuilder, *KubeadmUpgradeNodeStep]
}

func NewKubeadmUpgradeNodeStepBuilder(ctx runtime.Context, instanceName string) *KubeadmUpgradeNodeStepBuilder {
	s := &KubeadmUpgradeNodeStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Upgrade the node's kubelet configuration using 'kubeadm upgrade node'"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmUpgradeNodeStepBuilder).Init(s)
	return b
}

func (s *KubeadmUpgradeNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmUpgradeNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for 'kubeadm upgrade node'...")

	targetVersion, ok := ctx.GetTaskCache().Get(
		fmt.Sprintf(common.CacheKeyTargetVersion, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName()),
	)
	if !ok {
		return false, fmt.Errorf("precheck failed: target version not found in cache")
	}
	targetVer, err := version.ParseGeneric(targetVersion.(string))
	if err != nil {
		return false, fmt.Errorf("precheck failed: invalid target version in cache: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	versionCmd := "kubelet --version | awk '{print $2}'"
	stdout, err := runner.Run(ctx.GoContext(), conn, versionCmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("precheck failed: could not get current kubelet version: %w", err)
	}
	currentKubeletVerStr := strings.TrimSpace(string(stdout))
	currentKubeletVer, err := version.ParseGeneric(currentKubeletVerStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse current kubelet version '%s': %w", currentKubeletVerStr, err)
	}

	if currentKubeletVer.AtLeast(targetVer) {
		logger.Infof("Kubelet version (%s) is already at or above the target version (%s). Step is done.", currentKubeletVerStr, targetVer.String())
		return true, nil
	}

	logger.Infof("Precheck passed: Kubelet version %s needs upgrade to %s.", currentKubeletVerStr, targetVer.String())
	return false, nil
}

func (s *KubeadmUpgradeNodeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Running 'kubeadm upgrade node' to sync kubelet and other node configurations...")

	upgradeNodeCmd := "kubeadm upgrade node"

	stdout, err := runner.Run(ctx.GoContext(), conn, upgradeNodeCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("'kubeadm upgrade node' failed. Output:\n%s\nError: %w", string(stdout), err)
	}

	output := string(stdout)
	logger.Infof("`kubeadm upgrade node` output:\n%s", output)

	re := regexp.MustCompile(`backed up to (/etc/kubernetes/tmp/kubeadm-backup-etcd-[a-z0-9-]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		backupPath := matches[1]
		cacheKey := fmt.Sprintf(common.CacheKeyKubeadmBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
		ctx.GetTaskCache().Set(cacheKey, backupPath)
		logger.Infof("Detected and saved kubeadm backup path to cache: %s", backupPath)
	}

	if !strings.Contains(output, "[upgrade/successful] SUCCESS!") {
		return fmt.Errorf("could not find success message in 'kubeadm upgrade node' output")
	}

	logger.Info("Node configuration upgraded successfully.")
	return nil
}

func (s *KubeadmUpgradeNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for 'kubeadm upgrade node' is not a direct command.")
	logger.Warn("It requires restoring the backup created by kubeadm, which should be handled by a dedicated 'RestoreKubeadmBackupStep'.")
	cacheKey := fmt.Sprintf(common.CacheKeyKubeadmBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	if backupPath, ok := ctx.GetTaskCache().Get(cacheKey); ok {
		logger.Warnf("If manual rollback is needed, restore the backup from: %s", backupPath)
	}

	return nil
}

var _ step.Step = (*KubeadmUpgradeNodeStep)(nil)
