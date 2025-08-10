package containerd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallRuncStep struct {
	step.Base
	RemoteRuncTargetPath string
	Permission           string
}

type InstallRuncStepBuilder struct {
	step.Builder[InstallRuncStepBuilder, *InstallRuncStep]
}

func NewInstallRuncStepBuilder(ctx runtime.Context, instanceName string) *InstallRuncStepBuilder {
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
			logger.Infof("runc not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure download step ran successfully", localSourcePath)
	}

	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, s.RemoteRuncTargetPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", s.RemoteRuncTargetPath, err)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", s.RemoteRuncTargetPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Installation is required.", s.RemoteRuncTargetPath)
	}

	return isDone, nil
}

func (s *InstallRuncStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("runc not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return nil
		}
		return err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, ensure download step ran successfully", localSourcePath)
	}

	installDir := filepath.Dir(s.RemoteRuncTargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, installDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", installDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("runc-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "runc")
	logger.Debugf("Uploading runc to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, s.RemoteRuncTargetPath)
	logger.Debugf("Moving file to %s on remote host", s.RemoteRuncTargetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move file to '%s': %w", s.RemoteRuncTargetPath, err)
	}

	logger.Debugf("Setting permissions for %s to %s", s.RemoteRuncTargetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, s.RemoteRuncTargetPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to set permission on '%s': %w", s.RemoteRuncTargetPath, err)
	}

	logger.Info("runc binary has been installed successfully.")
	return nil
}

func (s *InstallRuncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.RemoteRuncTargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteRuncTargetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteRuncTargetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallRuncStep)(nil)
