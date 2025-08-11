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
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallContainerdStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallContainerdStepBuilder struct {
	step.Builder[InstallContainerdStepBuilder, *InstallContainerdStep]
}

func NewInstallContainerdStepBuilder(ctx runtime.Context, instanceName string) *InstallContainerdStepBuilder {
	s := &InstallContainerdStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install containerd binaries", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallContainerdStepBuilder).Init(s)
	return b
}

func (b *InstallContainerdStepBuilder) WithInstallPath(installPath string) *InstallContainerdStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallContainerdStepBuilder) WithPermission(permission string) *InstallContainerdStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallContainerdStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get containerd binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("containerd is disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install containerd binaries (version %s)", s.Base.Meta.Name, binaryInfo.Version)
	sourcePath := binaryInfo.FilePath()
	innerPath := "containerd"
	destPath := filepath.Join(filepath.Dir(sourcePath), innerPath)
	return destPath, nil
}

func (s *InstallContainerdStep) filesToInstall() map[string]string {
	return map[string]string{
		"bin/containerd":              filepath.Join(s.InstallPath, "containerd"),
		"bin/containerd-shim-runc-v2": filepath.Join(s.InstallPath, "containerd-shim-runc-v2"),
		"bin/containerd-stress":       filepath.Join(s.InstallPath, "containerd-stress"),
		"bin/ctr":                     filepath.Join(s.InstallPath, "ctr"),
	}
}

func (s *InstallContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// 检查本地源文件是否存在
	localExtractedPath, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("Containerd not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}
	if _, err := os.Stat(filepath.Join(localExtractedPath, "bin", "containerd")); os.IsNotExist(err) {
		return false, fmt.Errorf("local containerd extracted path '%s' or key file 'bin/containerd' not found, ensure extract step ran successfully", localExtractedPath)
	}

	files := s.filesToInstall()
	allExist := true
	for _, targetPath := range files {
		exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
		if err != nil {
			return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", targetPath, ctx.GetHost().GetName(), err)
		}
		if !exists {
			logger.Infof("Target file '%s' does not exist. Installation is required.", targetPath)
			allExist = false
			break
		}
	}

	if allExist {
		logger.Info("All required containerd files already exist on remote host. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *InstallContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("Containerd not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return nil
		}
		return err
	}
	if _, err := os.Stat(localExtractedPath); os.IsNotExist(err) {
		return fmt.Errorf("local containerd extracted path '%s' not found, ensure extract step ran successfully", localExtractedPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("containerd-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	files := s.filesToInstall()
	for sourceRelPath, remoteTargetPath := range files {
		localSourcePath := filepath.Join(localExtractedPath, sourceRelPath)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			logger.Warnf("Local source file '%s' not found, skipping its installation.", localSourcePath)
			continue
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, filepath.Base(localSourcePath))
		logger.Debugf("Uploading %s to %s:%s", filepath.Base(localSourcePath), ctx.GetHost().GetName(), remoteTempPath)
		if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localSourcePath, err)
		}

		moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, remoteTargetPath)
		logger.Debugf("Moving file to %s on remote host", remoteTargetPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move file to '%s': %w", remoteTargetPath, err)
		}

		logger.Debugf("Setting permissions for %s to %s", remoteTargetPath, s.Permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remoteTargetPath, s.Permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on '%s': %w", remoteTargetPath, err)
		}
	}

	logger.Info("All containerd binaries have been installed successfully.")
	return nil
}

func (s *InstallContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	files := s.filesToInstall()
	for _, targetPath := range files {
		logger.Warnf("Rolling back by removing: %s", targetPath)
		if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallContainerdStep)(nil)
