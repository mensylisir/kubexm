package longhorn

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanLonghornStep struct {
	step.Base
	ReleaseName string
	Namespace   string
	PurgeData   bool
}

type CleanLonghornStepBuilder struct {
	step.Builder[CleanLonghornStepBuilder, *CleanLonghornStep]
}

func NewCleanLonghornStepBuilder(ctx runtime.Context, instanceName string) *CleanLonghornStepBuilder {
	s := &CleanLonghornStep{
		ReleaseName: "longhorn",
		Namespace:   "longhorn-system",
		PurgeData:   true,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Longhorn Helm release and cleanup all related data", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(CleanLonghornStepBuilder).Init(s)
	return b
}

func (b *CleanLonghornStepBuilder) WithPurgeData(purge bool) *CleanLonghornStepBuilder {
	b.Step.PurgeData = purge
	return b
}

func (s *CleanLonghornStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanLonghornStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkNsCmd := fmt.Sprintf("kubectl get namespace %s", s.Namespace)
	_, err = runner.Run(ctx.GoContext(), conn, checkNsCmd, s.Sudo)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") {
			logger.Info("Longhorn namespace not found. Step is done.")
			return true, nil
		}
	}

	logger.Info("Longhorn namespace found. Cleanup is required.")
	return false, nil
}

func (s *CleanLonghornStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if ctx.GetHost().IsRole(common.RoleMaster) {
		uninstallCmd := fmt.Sprintf("helm uninstall %s -n %s --wait --timeout 10m", s.ReleaseName, s.Namespace)
		logger.Infof("Uninstalling Longhorn Helm release with command: %s", uninstallCmd)
		if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "release: not found") {
				logger.Warnf("Helm uninstall command failed (this may be ok): %v", err)
			}
		}

		logger.Warn("Forcefully deleting Longhorn CRDs...")
		crdDeleteCmd := "kubectl get crd -o name | grep 'longhorn.io' | xargs -r kubectl delete"
		if _, err := runner.Run(ctx.GoContext(), conn, crdDeleteCmd, s.Sudo); err != nil {
			logger.Warnf("Failed to delete Longhorn CRDs (this may be ok if they were already gone): %v", err)
		}

		deleteNsCmd := fmt.Sprintf("kubectl delete namespace %s --ignore-not-found=true --force --grace-period=0", s.Namespace)
		logger.Infof("Forcefully deleting Longhorn namespace '%s'", s.Namespace)
		if _, err := runner.Run(ctx.GoContext(), conn, deleteNsCmd, s.Sudo); err != nil {
			logger.Warnf("Failed to delete Longhorn namespace: %v", err)
		}
	}

	if s.PurgeData {
		longhornDataPath := "/var/lib/longhorn"
		logger.Warnf("Purging Longhorn data from host directory: %s", longhornDataPath)
		if err := runner.Remove(ctx.GoContext(), conn, longhornDataPath, s.Sudo, true); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Warnf("Failed to remove Longhorn data directory '%s': %v", longhornDataPath, err)
			}
		}
	}

	logger.Info("Longhorn cleanup process on this node finished.")
	return nil
}

func (s *CleanLonghornStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CleanLonghornStep)(nil)
