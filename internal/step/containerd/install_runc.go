package containerd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type InstallRuncStep struct {
	step.Base
	RemoteRuncTargetPath string
	Permission           string
}

type InstallRuncStepBuilder struct {
	step.Builder[InstallRuncStepBuilder, *InstallRuncStep]
}

func NewInstallRuncStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallRuncStepBuilder {
	s := &InstallRuncStep{
		RemoteRuncTargetPath: filepath.Join(common.DefaultLocalSBinDir, "runc"),
		Permission:           "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install runc binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallRuncStepBuilder).Init(s)
	return b
}

func (b *InstallRuncStepBuilder) WithRemoteRuncTargetPath(remoteRuncTargetPath string) *InstallRuncStepBuilder {
	if remoteRuncTargetPath != "" {
		b.Step.RemoteRuncTargetPath = remoteRuncTargetPath
	}
	return b
}

func (b *InstallRuncStepBuilder) WithPermission(permission string) *InstallRuncStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallRuncStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallRuncStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentRunc, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get runc binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("runc is disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install runc binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	return binaryInfo.FilePath(), nil
}

func (s *InstallRuncStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Info("runc not required for this host, skipping.", "arch", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, s.RemoteRuncTargetPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", s.RemoteRuncTargetPath, err)
	}

	if isDone {
		logger.Info("Target file already exists and is up-to-date. Step is done.", "path", s.RemoteRuncTargetPath)
	} else {
		logger.Info("Target file is missing or outdated. Installation is required.", "path", s.RemoteRuncTargetPath)
	}

	return isDone, nil
}

func (s *InstallRuncStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Info("runc not required for this host, skipping.", "arch", ctx.GetHost().GetArch())
			result.MarkCompleted("skipping - runc not required for this host")
			return result, nil
		}
		result.MarkFailed(err, "failed to get local source path")
		return result, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		result.MarkFailed(err, "local source file not found")
		return result, fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	installDir := filepath.Dir(s.RemoteRuncTargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, installDir, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to create remote install directory")
		return result, fmt.Errorf("failed to create remote install directory '%s': %w", installDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("runc-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		result.MarkFailed(err, "failed to create remote upload directory")
		return result, fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "runc")
	logger.Debug("Uploading runc.", "to", remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		result.MarkFailed(err, "failed to upload runc")
		return result, fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, s.RemoteRuncTargetPath)
	logger.Debug("Moving file.", "to", s.RemoteRuncTargetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to move file")
		return result, fmt.Errorf("failed to move file to '%s': %w", s.RemoteRuncTargetPath, err)
	}

	logger.Debug("Setting permissions.", "path", s.RemoteRuncTargetPath, "permissions", s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, s.RemoteRuncTargetPath, s.Permission, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to set permission")
		return result, fmt.Errorf("failed to set permission on '%s': %w", s.RemoteRuncTargetPath, err)
	}

	logger.Info("runc binary has been installed successfully.")
	result.MarkCompleted("runc installed successfully")
	return result, nil
}

func (s *InstallRuncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing.", "path", s.RemoteRuncTargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteRuncTargetPath, s.Sudo, false); err != nil {
		logger.Error(err, "Failed to remove path during rollback.", "path", s.RemoteRuncTargetPath)
	}

	return nil
}

var _ step.Step = (*InstallRuncStep)(nil)
