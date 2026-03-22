package etcd

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

type CleanupEtcdStep struct {
	step.Base
	PathsToRemove []string
}

type CleanupEtcdStepBuilder struct {
	step.Builder[CleanupEtcdStepBuilder, *CleanupEtcdStep]
}

func NewCleanupEtcdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanupEtcdStepBuilder {
	s := &CleanupEtcdStep{
		PathsToRemove: []string{
			common.EtcdDefaultDataDirTarget,
			common.EtcdDefaultConfDirTarget,
			common.EtcdSystemdFile,
			filepath.Dir(common.EtcdDropInFile),
			filepath.Join(common.DefaultBinDir, "etcd"),
			filepath.Join(common.DefaultBinDir, "etcdctl"),
		},
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup all etcd files and directories on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = true
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanupEtcdStepBuilder).Init(s)
	return b
}

func (b *CleanupEtcdStepBuilder) WithPathsToRemove(paths []string) *CleanupEtcdStepBuilder {
	b.Step.PathsToRemove = paths
	return b
}

func (s *CleanupEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanupEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, path := range s.PathsToRemove {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, fmt.Errorf("failed to check existence of %s: %w", path, err)
		}
		if exists {
			return false, nil
		}
	}

	return true, nil
}

func (s *CleanupEtcdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get connector")
		return result, err
	}

	for _, path := range s.PathsToRemove {
		logger.Warn("Removing path", "path", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			logger.Error(err, "Failed to remove path, but ignoring due to step configuration", "path", path)
		}
	}

	logger.Info("Etcd cleanup completed.")
	result.MarkCompleted("Cleanup completed successfully")
	return result, nil
}

func (s *CleanupEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for a cleanup step is not possible.")
	return nil
}

var _ step.Step = (*CleanupEtcdStep)(nil)
