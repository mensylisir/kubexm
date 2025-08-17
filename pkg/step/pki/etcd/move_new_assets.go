package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type MoveNewAssetsStep struct {
	step.Base
	localCertsDir    string
	localNewCertsDir string
	movedFiles       map[string]string
}

type MoveNewAssetsStepBuilder struct {
	step.Builder[MoveNewAssetsStepBuilder, *MoveNewAssetsStep]
}

func NewMoveNewAssetsStepBuilder(ctx runtime.Context, instanceName string) *MoveNewAssetsStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &MoveNewAssetsStep{
		localCertsDir:    localCertsDir,
		localNewCertsDir: filepath.Join(localCertsDir, "certs-new"),
		movedFiles:       make(map[string]string),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Move new assets from certs-new to the main certs directory"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(MoveNewAssetsStepBuilder).Init(s)
	return b
}

func (s *MoveNewAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *MoveNewAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if !helpers.IsDirExist(s.localNewCertsDir) {
		return false, fmt.Errorf("source directory 'certs-new' ('%s') not found. Ensure previous steps ran successfully", s.localNewCertsDir)
	}
	entries, err := os.ReadDir(s.localNewCertsDir)
	if err != nil {
		return false, fmt.Errorf("failed to read certs-new directory: %w", err)
	}
	if len(entries) == 0 {
		logger.Warn("certs-new directory is empty, nothing to move.")
		return true, nil
	}

	return false, nil
}

func (s *MoveNewAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Infof("Moving files from '%s' to '%s'...", s.localNewCertsDir, s.localCertsDir)

	files, err := filepath.Glob(filepath.Join(s.localNewCertsDir, "*.pem"))
	if err != nil {
		return fmt.Errorf("failed to list files in certs-new directory: %w", err)
	}

	for _, srcPath := range files {
		fileName := filepath.Base(srcPath)
		dstPath := filepath.Join(s.localCertsDir, fileName)

		logger.Debugf("Moving '%s' to '%s'", srcPath, dstPath)

		if err := os.Rename(srcPath, dstPath); err != nil {
			s.rollbackInternal(ctx)
			return fmt.Errorf("failed to move file '%s' to '%s': %w", srcPath, dstPath, err)
		}

		s.movedFiles[dstPath] = srcPath
	}

	logger.Info("All new assets have been moved successfully.")
	return nil
}

func (s *MoveNewAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
	s.rollbackInternal(ctx)
	return nil
}

func (s *MoveNewAssetsStep) rollbackInternal(ctx runtime.ExecutionContext) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if len(s.movedFiles) == 0 {
		return
	}

	logger.Warnf("Rolling back by moving %d files back to '%s'...", len(s.movedFiles), s.localNewCertsDir)

	for newPath, oldPath := range s.movedFiles {
		if err := os.Rename(newPath, oldPath); err != nil {
			logger.Errorf("CRITICAL: Failed to move file '%s' back to '%s' during rollback. Manual intervention may be required. Error: %v", newPath, oldPath, err)
		}
	}

	s.movedFiles = make(map[string]string)
}

var _ step.Step = (*MoveNewAssetsStep)(nil)
