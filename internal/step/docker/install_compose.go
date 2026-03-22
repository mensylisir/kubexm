package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type InstallDockerComposeStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallDockerComposeStepBuilder struct {
	step.Builder[InstallDockerComposeStepBuilder, *InstallDockerComposeStep]
}

func NewInstallDockerComposeStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallDockerComposeStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCompose, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallDockerComposeStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker-compose binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallDockerComposeStepBuilder).Init(s)
	return b
}

func (b *InstallDockerComposeStepBuilder) WithInstallPath(installPath string) *InstallDockerComposeStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallDockerComposeStepBuilder) WithPermission(permission string) *InstallDockerComposeStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallDockerComposeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallDockerComposeStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentCompose, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get docker-compose binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("docker-compose is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker-compose binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	return binaryInfo.FilePath(), nil
}

func (s *InstallDockerComposeStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "docker-compose")
}

func (s *InstallDockerComposeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("docker-compose not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	targetPath := s.getRemoteTargetPath()
	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, targetPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", targetPath, err)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", targetPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Installation is required.", targetPath)
	}

	return isDone, nil
}

func (s *InstallDockerComposeStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		result.MarkFailed(err, "failed to get local source path")
		return result, err
	}
	targetPath := s.getRemoteTargetPath()

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		result.MarkFailed(err, "local source file not found")
		return result, fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to create remote install directory")
		return result, fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("docker-compose-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		result.MarkFailed(err, "failed to create remote upload directory")
		return result, fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "docker-compose")
	logger.Infof("Uploading docker-compose to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		result.MarkFailed(err, "failed to upload docker-compose")
		return result, fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetPath)
	logger.Infof("Moving file to %s on remote host", targetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to move docker-compose")
		return result, fmt.Errorf("failed to move file to '%s': %w", targetPath, err)
	}

	logger.Infof("Setting permissions for %s to %s", targetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, targetPath, s.Permission, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to set permission")
		return result, fmt.Errorf("failed to set permission on '%s': %w", targetPath, err)
	}

	logger.Infof("Successfully installed docker-compose to %s", targetPath)
	result.MarkCompleted("docker-compose installed successfully")
	return result, nil
}

func (s *InstallDockerComposeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	targetPath := s.getRemoteTargetPath()
	logger.Warnf("Rolling back by removing: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallDockerComposeStep)(nil)
