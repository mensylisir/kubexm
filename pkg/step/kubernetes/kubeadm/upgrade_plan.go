package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"k8s.io/apimachinery/pkg/util/version"
)

type KubeadmUpgradePlanStep struct {
	step.Base
	TargetVersion string
}

type KubeadmUpgradePlanStepBuilder struct {
	step.Builder[KubeadmUpgradePlanStepBuilder, *KubeadmUpgradePlanStep]
}

func NewKubeadmUpgradePlanStepBuilder(ctx runtime.Context, instanceName string) *KubeadmUpgradePlanStepBuilder {
	s := &KubeadmUpgradePlanStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check for available Kubernetes upgrades using 'kubeadm upgrade plan'"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmUpgradePlanStepBuilder).Init(s)
	return b
}

func (b *KubeadmUpgradePlanStepBuilder) WithTargetVersion(v string) *KubeadmUpgradePlanStepBuilder {
	b.Step.TargetVersion = v
	return b
}

func (s *KubeadmUpgradePlanStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmUpgradePlanStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for upgrade plan...")

	if s.TargetVersion == "" {
		return false, fmt.Errorf("precheck failed: TargetVersion is not set for the upgrade plan step")
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	getVerCmd := "kubectl --kubeconfig /etc/kubernetes/admin.conf version -o json | jq -r '.serverVersion.gitVersion'"
	shellCmd := fmt.Sprintf("bash -c \"%s\"", getVerCmd)

	stdout, err := runner.Run(ctx.GoContext(), conn, shellCmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("precheck failed: could not get current Kubernetes server version: %w", err)
	}
	currentVerStr := strings.TrimSpace(string(stdout))

	currentVer, err := version.ParseGeneric(currentVerStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse current version '%s': %w", currentVerStr, err)
	}

	targetVer, err := version.ParseGeneric(s.TargetVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse target version '%s': %w", s.TargetVersion, err)
	}

	if currentVer.AtLeast(targetVer) {
		logger.Infof("Current cluster version (%s) is already at or above the target version (%s). Step is done.", currentVerStr, s.TargetVersion)
		return true, nil
	}

	logger.Infof("Precheck passed: Current version %s is less than target %s. Planning is required.", currentVerStr, s.TargetVersion)
	return false, nil
}

func (s *KubeadmUpgradePlanStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Running 'kubeadm upgrade plan' for version %s...", s.TargetVersion)

	planCmd := fmt.Sprintf("kubeadm upgrade plan %s --config /etc/kubernetes/kubeadm-config.yaml", s.TargetVersion)

	stdout, err := runner.Run(ctx.GoContext(), conn, planCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("'kubeadm upgrade plan' failed. This indicates the cluster is not ready for upgrade or the target version is invalid. Output:\n%s\nError: %w", string(stdout), err)
	}

	output := string(stdout)
	logger.Infof("`kubeadm upgrade plan` output:\n%s", output)

	if !strings.Contains(output, "You can now apply the upgrade by executing the following command") {
		return fmt.Errorf("could not find success message in 'kubeadm upgrade plan' output. The plan might have warnings that need attention")
	}
	ctx.GetTaskCache().Set(
		fmt.Sprintf(common.CacheKeyTargetVersion, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName()),
		s.TargetVersion,
	)
	logger.Infof("Successfully verified upgrade plan. Target version '%s' saved to cache.", s.TargetVersion)

	return nil
}

func (s *KubeadmUpgradePlanStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a read-only planning step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmUpgradePlanStep)(nil)
