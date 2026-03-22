package kubevip

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanKubeVipManifestStep struct {
	step.Base
}

type CleanKubeVipManifestStepBuilder struct {
	step.Builder[CleanKubeVipManifestStepBuilder, *CleanKubeVipManifestStep]
}

func NewCleanKubeVipManifestStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanKubeVipManifestStepBuilder {
	s := &CleanKubeVipManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean kube-vip static pod manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(CleanKubeVipManifestStepBuilder).Init(s)
	return b
}

func (s *CleanKubeVipManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanKubeVipManifestStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil {
		return false, err
	}
	if !exists {
		ctx.GetLogger().Infof("kube-vip static pod manifest does not exist. Cleanup is done.")
		return true, nil
	}
	ctx.GetLogger().Infof("kube-vip static pod manifest found. Cleanup is required.")
	return false, nil
}

func (s *CleanKubeVipManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")
	logger.Infof("Removing kube-vip static pod manifest: %s", remoteManifestPath)

	if err := runner.Remove(ctx.GoContext(), conn, remoteManifestPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove kube-vip manifest (may not exist): %v", err)
	}

	logger.Info("kube-vip static pod manifest cleaned up successfully.")
	result.MarkCompleted("kube-vip static pod manifest cleaned up successfully")
	return result, nil
}

func (s *CleanKubeVipManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanKubeVipManifestStep)(nil)
