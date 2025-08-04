package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallBuildxStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallBuildxStepBuilder struct {
	step.Builder[InstallBuildxStepBuilder, *InstallBuildxStep]
}

func NewInstallBuildxStepBuilder(ctx runtime.Context, instanceName string) *InstallBuildxStepBuilder {
	s := &InstallBuildxStep{
		InstallPath: filepath.Join(common.DockerPluginsDir, "docker-buildx"),
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker buildx binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallBuildxStepBuilder).Init(s)
	return b
}

func (b *InstallBuildxStepBuilder) WithInstallPath(installPath string) *InstallBuildxStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallBuildxStepBuilder) WithPermission(permission string) *InstallBuildxStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallBuildxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallBuildxStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentBuildx, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get buildx binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("buildx is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker buildx binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	return binaryInfo.FilePath(), nil
}

func (s *InstallBuildxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.InstallPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.InstallPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists. Step is done.", s.InstallPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Installation is required.", s.InstallPath)
	return false, nil
}

func (s *InstallBuildxStep) Run(ctx runtime.ExecutionContext) error {
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

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, ensure download step ran successfully", localSourcePath)
	}

	installDir := filepath.Dir(s.InstallPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, installDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", installDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("buildx-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "docker-buildx")
	logger.Infof("Uploading buildx to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, s.InstallPath)
	logger.Infof("Moving file to %s on remote host", s.InstallPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move file to '%s': %w", s.InstallPath, err)
	}

	logger.Infof("Setting permissions for %s to %s", s.InstallPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, s.InstallPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to set permission on '%s': %w", s.InstallPath, err)
	}

	logger.Infof("Successfully installed docker-buildx to %s", s.InstallPath)
	return nil
}

func (s *InstallBuildxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.InstallPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.InstallPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.InstallPath, err)
	}

	return nil
}

var _ step.Step = (*InstallBuildxStep)(nil)
