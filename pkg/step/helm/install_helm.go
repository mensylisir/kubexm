package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/pkg/errors"
)

type InstallHelmStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallHelmStepBuilder struct {
	step.Builder[InstallHelmStepBuilder, *InstallHelmStep]
}

func NewInstallHelmStepBuilder(ctx runtime.Context, instanceName string) *InstallHelmStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHelm, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallHelmStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install helm binary to system path", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallHelmStepBuilder).Init(s)
	return b
}

func (b *InstallHelmStepBuilder) WithInstallPath(installPath string) *InstallHelmStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallHelmStepBuilder) WithPermission(permission string) *InstallHelmStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallHelmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallHelmStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentHelm, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get helm binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("helm is unexpectedly disabled for arch %s", arch)
	}
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install helm binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	extractedDir := filepath.Dir(binaryInfo.FilePath())
	innerDir := fmt.Sprintf("linux-%s", arch)

	return filepath.Join(extractedDir, innerDir, "helm"), nil
}

func (s *InstallHelmStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "helm")
}

func (s *InstallHelmStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		logger.Infof("Helm is not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
		return true, nil
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file '%s' not found, ensure extract step ran successfully", localSourcePath)
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

func (s *InstallHelmStep) Run(ctx runtime.ExecutionContext) error {
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

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return errors.Wrapf(err, "failed to create remote install directory '%s'", s.InstallPath)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("helm-install-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil { // No sudo for tmp dir
		return errors.Wrapf(err, "failed to create remote upload directory '%s'", remoteUploadTmpDir)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "helm")
	logger.Infof("Uploading helm to temporary path %s:%s", ctx.GetHost().GetName(), remoteTempPath)
	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		return errors.Wrapf(err, "failed to upload '%s' to '%s'", localSourcePath, remoteTempPath)
	}

	targetPath := s.getRemoteTargetPath()
	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetPath)
	logger.Infof("Moving file to final destination %s on remote host", targetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return errors.Wrapf(err, "failed to move file to '%s'", targetPath)
	}

	logger.Infof("Setting permissions for %s to %s", targetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, targetPath, s.Permission, s.Sudo); err != nil {
		return errors.Wrapf(err, "failed to set permission on '%s'", targetPath)
	}

	logger.Infof("Successfully installed helm to %s", targetPath)
	return nil
}

func (s *InstallHelmStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*InstallHelmStep)(nil)
