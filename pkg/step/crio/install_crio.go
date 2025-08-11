package crio

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

var filesToInstall = map[string]string{
	"crio":     common.DefaultBinPath,
	"pinns":    common.DefaultBinPath,
	"conmon":   common.CRIORuntimePath,
	"conmonrs": common.CRIORuntimePath,
	"runc":     common.CRIORuntimePath,
	"crun":     common.CRIORuntimePath,
}

type InstallCrioStep struct {
	step.Base
}

type InstallCrioStepBuilder struct {
	step.Builder[InstallCrioStepBuilder, *InstallCrioStep]
}

func NewInstallCrioStepBuilder(ctx runtime.Context, instanceName string) *InstallCrioStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(ComponentCrio, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CRI-O core binaries", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCrioStepBuilder).Init(s)
	return b
}

func (s *InstallCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCrioStep) getExtractedSourceDir(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(ComponentCrio, arch)
	if err != nil {
		return "", err
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("CRI-O binary info not found for arch %s", arch)
	}

	sourcePath := binaryInfo.FilePath()
	destPath := filepath.Join(filepath.Dir(sourcePath), "cri-o", "bin")
	return destPath, nil
}

func (s *InstallCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	allDone := true
	for file, targetDir := range filesToInstall {
		targetPath := filepath.Join(targetDir, file)
		exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
		if err != nil {
			return false, fmt.Errorf("failed to check for file '%s': %w", targetPath, err)
		}
		if !exists {
			logger.Infof("Binary '%s' not found on target host. Installation is required.", targetPath)
			allDone = false
			break
		}
	}

	if allDone {
		logger.Info("All required CRI-O core binaries are already installed.")
	}
	return allDone, nil
}

func (s *InstallCrioStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	sourceDir, err := s.getExtractedSourceDir(ctx)
	if err != nil {
		return err
	}

	targetDirs := map[string]bool{}
	for _, dir := range filesToInstall {
		targetDirs[dir] = true
	}
	for dir := range targetDirs {
		if err := runner.Mkdirp(ctx.GoContext(), conn, dir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create target directory '%s': %w", dir, err)
		}
	}

	for file, targetDir := range filesToInstall {
		sourceFile := filepath.Join(sourceDir, file)
		targetFile := filepath.Join(targetDir, file)

		logger.Infof("Copying %s to %s", sourceFile, targetFile)

		remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("%s-%d", file, time.Now().UnixNano()))
		if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
			return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
		}
		defer func() {
			_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
		}()
		remoteTempPath := filepath.Join(remoteUploadTmpDir, filepath.Base(sourceFile))
		logger.Debugf("Uploading %s to %s:%s", filepath.Base(sourceFile), ctx.GetHost().GetName(), remoteTempPath)
		if err := runner.Upload(ctx.GoContext(), conn, sourceFile, remoteTempPath, false); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", sourceFile, err)
		}

		moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetFile)
		logger.Debugf("Moving file to %s on remote host", targetFile)
		if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move file to '%s': %w", targetFile, err)
		}

		logger.Debugf("Setting permissions for %s to %s", targetFile, "0755")
		if err := runner.Chmod(ctx.GoContext(), conn, targetFile, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on '%s': %w", targetFile, err)
		}
	}

	logger.Info("CRI-O core binaries installed successfully.")
	return nil
}

func (s *InstallCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	for file, targetDir := range filesToInstall {
		targetPath := filepath.Join(targetDir, file)
		logger.Warnf("Rolling back by removing: %s", targetPath)
		if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
			}
		}
	}

	return nil
}

var _ step.Step = (*InstallCrioStep)(nil)
