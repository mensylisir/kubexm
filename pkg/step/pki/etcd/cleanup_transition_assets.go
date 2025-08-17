package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanupTransitionAssetsStep struct {
	step.Base
	filesToRemove []string
	dirsToRemove  []string
}

type CleanupTransitionAssetsStepBuilder struct {
	step.Builder[CleanupTransitionAssetsStepBuilder, *CleanupTransitionAssetsStep]
}

func NewCleanupTransitionAssetsStepBuilder(ctx runtime.Context, instanceName string) *CleanupTransitionAssetsStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()

	s := &CleanupTransitionAssetsStep{
		filesToRemove: []string{
			filepath.Join(fmt.Sprintf("%s/%s-%s", localCertsDir, "certs", "old"), "ca.pem"),
			filepath.Join(fmt.Sprintf("%s/%s-%s", localCertsDir, "certs", "new"), "ca.pem"),
			filepath.Join(localCertsDir, "ca-bundle.pem"),
		},
		dirsToRemove: []string{
			fmt.Sprintf("%s/%s-%s", localCertsDir, "certs", "old"),
			fmt.Sprintf("%s/%s-%s", localCertsDir, "certs", "new"),
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up intermediate files from the CA transition process"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(CleanupTransitionAssetsStepBuilder).Init(s)
	return b
}

func (s *CleanupTransitionAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanupTransitionAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if cleanup is necessary...")
	return false, nil
}

func (s *CleanupTransitionAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Cleaning up CA transition assets from the local workspace...")

	for _, file := range s.filesToRemove {
		logger.Debugf("Removing file: %s", file)
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to remove intermediate file '%s': %v", file, err)
		}
	}

	for _, dir := range s.dirsToRemove {
		logger.Debugf("Removing directory: %s", dir)
		if err := os.RemoveAll(dir); err != nil {
			logger.Warnf("Failed to remove intermediate directory '%s': %v", dir, err)
		}
	}

	logger.Info("Cleanup of transition assets completed.")
	return nil
}

func (s *CleanupTransitionAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Infof("No rollback required for cleanup of transition assets.")
	return nil
}

var _ step.Step = (*CleanupTransitionAssetsStep)(nil)
