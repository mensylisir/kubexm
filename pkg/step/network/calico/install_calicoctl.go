package calico

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

type InstallCalicoctlStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallCalicoctlStepBuilder struct {
	step.Builder[InstallCalicoctlStepBuilder, *InstallCalicoctlStep]
}

func NewInstallCalicoctlStepBuilder(ctx runtime.Context, instanceName string) *InstallCalicoctlStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
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
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install calicoctl binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCalicoctlStepBuilder).Init(s)
	return b
}

func (b *InstallCalicoctlStepBuilder) WithInstallPath(installPath string) *InstallCalicoctlStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallCalicoctlStepBuilder) WithPermission(permission string) *InstallCalicoctlStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
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

func (s *InstallCalicoctlStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("local source file '%s' not found, ensure download step ran successfully", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(common.DefaultUploadTmpDir, fmt.Sprintf("calicoctl-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "calicoctl")
	logger.Infof("Uploading calicoctl to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

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

	logger.Infof("Successfully installed calicoctl to %s", targetPath)
	return nil
}

func (s *InstallCalicoctlStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*InstallCalicoctlStep)(nil)
