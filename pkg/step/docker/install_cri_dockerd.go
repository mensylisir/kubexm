package docker

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
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallCriDockerdStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallCriDockerdStepBuilder struct {
	step.Builder[InstallCriDockerdStepBuilder, *InstallCriDockerdStep]
}

func NewInstallCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *InstallCriDockerdStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCriDockerd, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallCriDockerdStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install cri-dockerd binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCriDockerdStepBuilder).Init(s)
	return b
}

func (b *InstallCriDockerdStepBuilder) WithInstallPath(installPath string) *InstallCriDockerdStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallCriDockerdStepBuilder) WithPermission(permission string) *InstallCriDockerdStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCriDockerdStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentCriDockerd, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get cri-dockerd binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("cri-dockerd is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install cri-dockerd binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tgz")
	sourcePath := filepath.Join(common.DefaultExtractTmpDir, destDirName, "cri-dockerd", "cri-dockerd")
	return sourcePath, nil
}

func (s *InstallCriDockerdStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "cri-dockerd")
}

func (s *InstallCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := s.getRemoteTargetPath()
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", targetPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists. Step is done.", targetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Installation is required.", targetPath)
	return false, nil
}

func (s *InstallCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		return err
	}
	targetPath := s.getRemoteTargetPath()

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, ensure extract step ran successfully", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(common.DefaultUploadTmpDir, fmt.Sprintf("cri-dockerd-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "cri-dockerd")
	logger.Infof("Uploading cri-dockerd to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetPath)
	logger.Infof("Moving file to %s on remote host", targetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move file to '%s': %w", targetPath, err)
	}

	logger.Infof("Setting permissions for %s to %s", targetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, targetPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to set permission on '%s': %w", targetPath, err)
	}

	logger.Infof("Successfully installed cri-dockerd to %s", targetPath)
	return nil
}

func (s *InstallCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*InstallCriDockerdStep)(nil)
