package calico

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type InstallCalicoctlStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallCalicoctlStepBuilder struct {
	step.Builder[InstallCalicoctlStepBuilder, *InstallCalicoctlStep]
}

func NewInstallCalicoctlStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallCalicoctlStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCalicoCtl, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallCalicoctlStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install calicoctl binary to system path", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCalicoctlStepBuilder).Init(s)
	return b
}

func (s *InstallCalicoctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCalicoctlStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentCalicoCtl, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get calicoctl binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("calicoctl is unexpectedly disabled for arch %s", arch)
	}
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install calicoctl binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)
	return binaryInfo.FilePath(), nil
}

func (s *InstallCalicoctlStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "calicoctl")
}

func (s *InstallCalicoctlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		logger.Infof("calicoctl is not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
		return true, nil
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	targetPath := s.getRemoteTargetPath()
	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, targetPath, s.Sudo)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check remote file integrity for %s", targetPath)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", targetPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Installation is required.", targetPath)
	}

	return isDone, nil
}

func (s *InstallCalicoctlStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector for Run phase")
		return result, err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		logger.Infof("calicoctl is not required for this host (arch: %s), skipping run phase.", ctx.GetHost().GetArch())
		result.MarkCompleted("calicoctl not required for this host, skipping")
		return result, nil
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create remote install directory '%s'", s.InstallPath))
		return result, err
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("calicoctl-install-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create remote upload directory '%s'", remoteUploadTmpDir))
		return result, err
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true) // no-sudo, recursive
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "calicoctl")
	logger.Infof("Uploading calicoctl to temporary path %s:%s", ctx.GetHost().GetName(), remoteTempPath)
	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to upload '%s' to '%s'", localSourcePath, remoteTempPath))
		return result, err
	}

	targetPath := s.getRemoteTargetPath()
	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetPath)
	logger.Infof("Moving file to final destination %s on remote host", targetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to move file to '%s'", targetPath))
		return result, err
	}

	logger.Infof("Setting permissions for %s to %s", targetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, targetPath, s.Permission, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to set permission on '%s'", targetPath))
		return result, err
	}

	logger.Infof("Successfully installed calicoctl to %s", targetPath)
	result.MarkCompleted("calicoctl installed successfully")
	return result, nil
}

func (s *InstallCalicoctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetPath := s.getRemoteTargetPath()
	logger.Warnf("Rolling back by removing: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallCalicoctlStep)(nil)
