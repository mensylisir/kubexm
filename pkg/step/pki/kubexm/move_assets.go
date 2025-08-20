package kubexm

import (
	"fmt"
	"io/fs"
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
	localKubeCertsDir string
	localNewCertsDir  string
	movedFiles        map[string]string
}

type MoveNewAssetsStepBuilder struct {
	step.Builder[MoveNewAssetsStepBuilder, *MoveNewAssetsStep]
}

func NewMoveNewAssetsStepBuilder(ctx runtime.Context, instanceName string) *MoveNewAssetsStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	s := &MoveNewAssetsStep{
		localKubeCertsDir: localCertsDir,
		localNewCertsDir:  filepath.Join(localCertsDir, "certs-new"),
		movedFiles:        make(map[string]string),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Activate new assets by moving them from 'certs-new' to the main pki directory"
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
		logger.Info("Source directory 'certs-new' not found. Assuming no renewal was performed, step is done.")
		return true, nil
	}

	entries, err := os.ReadDir(s.localNewCertsDir)
	if err != nil {
		return false, fmt.Errorf("failed to read 'certs-new' directory: %w", err)
	}

	if len(entries) == 0 {
		logger.Info("'certs-new' directory is empty, nothing to move. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: 'certs-new' directory contains assets to be moved.")
	return false, nil
}

func (s *MoveNewAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Infof("Moving all assets from '%s' to '%s'...", s.localNewCertsDir, s.localKubeCertsDir)

	walkErr := filepath.WalkDir(s.localNewCertsDir, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.localNewCertsDir, srcPath)
		if err != nil {
			return fmt.Errorf("could not determine relative path for '%s': %w", srcPath, err)
		}
		dstPath := filepath.Join(s.localKubeCertsDir, relPath)

		if srcPath == s.localNewCertsDir {
			return nil
		}

		log := logger.With("path", relPath)

		if d.IsDir() {
			log.Debugf("Ensuring destination directory exists: '%s'", dstPath)
			if err := os.MkdirAll(dstPath, d.Type().Perm()); err != nil {
				return fmt.Errorf("failed to create destination directory '%s': %w", dstPath, err)
			}
		} else {
			log.Debugf("Moving file to '%s'", dstPath)
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to move file '%s': %w", relPath, err)
			}
			s.movedFiles[dstPath] = srcPath
		}
		return nil
	})

	if walkErr != nil {
		logger.Errorf("An error occurred while moving assets: %v. Triggering rollback.", walkErr)
		s.rollbackInternal(ctx)
		return walkErr
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
		logger.Info("No files were moved, nothing to roll back.")
		return
	}

	logger.Warnf("Rolling back by moving %d file(s) back to '%s'...", len(s.movedFiles), s.localNewCertsDir)

	for newPath, oldPath := range s.movedFiles {
		oldParentDir := filepath.Dir(oldPath)
		if err := os.MkdirAll(oldParentDir, 0755); err != nil {
			logger.Errorf("CRITICAL: Failed to create directory '%s' for rollback. File '%s' cannot be restored. MANUAL INTERVENTION REQUIRED. Error: %v", oldParentDir, newPath, err)
			continue
		}

		if err := os.Rename(newPath, oldPath); err != nil {
			logger.Errorf("CRITICAL: Failed to move file '%s' back to '%s' during rollback. The pki directory is in an inconsistent state. MANUAL INTERVENTION REQUIRED. Error: %v", newPath, oldPath, err)
		}
	}

	s.movedFiles = make(map[string]string)
	logger.Info("Rollback attempt finished.")
}

var _ step.Step = (*MoveNewAssetsStep)(nil)
