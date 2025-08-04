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

type InstallDockerStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallDockerStepBuilder struct {
	step.Builder[InstallDockerStepBuilder, *InstallDockerStep]
}

func NewInstallDockerStepBuilder(ctx runtime.Context, instanceName string) *InstallDockerStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentDocker, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallDockerStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Docker binaries", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallDockerStepBuilder).Init(s)
	return b
}

func (b *InstallDockerStepBuilder) WithInstallPath(installPath string) *InstallDockerStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallDockerStepBuilder) WithPermission(permission string) *InstallDockerStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallDockerStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentDocker, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get Docker binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("Docker is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Docker binaries (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tgz")
	return filepath.Join(ctx.GetExtractDir(), destDirName, "docker"), nil
}

func (s *InstallDockerStep) filesToInstall() []string {
	return []string{
		"containerd",
		"containerd-shim-runc-v2",
		"ctr",
		"docker",
		"docker-init",
		"docker-proxy",
		"dockerd",
		"runc",
	}
}

func (s *InstallDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	files := s.filesToInstall()
	allExist := true
	for _, fileName := range files {
		targetPath := filepath.Join(s.InstallPath, fileName)
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
		logger.Info("All required Docker files already exist on remote host. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *InstallDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourceDir, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(localSourceDir); os.IsNotExist(err) {
		return fmt.Errorf("local Docker extracted path '%s' not found, ensure extract step ran successfully", localSourceDir)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("docker-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	files := s.filesToInstall()
	for _, fileName := range files {
		localSourcePath := filepath.Join(localSourceDir, fileName)
		remoteTargetPath := filepath.Join(s.InstallPath, fileName)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			logger.Warnf("Local source file '%s' not found, skipping its installation.", localSourcePath)
			continue
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, fileName)
		logger.Infof("Uploading %s to %s:%s", fileName, ctx.GetHost().GetName(), remoteTempPath)
		if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localSourcePath, err)
		}

		moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, remoteTargetPath)
		logger.Infof("Moving file to %s on remote host", remoteTargetPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move file to '%s': %w", remoteTargetPath, err)
		}

		logger.Infof("Setting permissions for %s to %s", remoteTargetPath, s.Permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remoteTargetPath, s.Permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on '%s': %w", remoteTargetPath, err)
		}
	}

	logger.Info("Successfully installed all Docker binaries.")
	return nil
}

func (s *InstallDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	files := s.filesToInstall()
	for _, fileName := range files {
		targetPath := filepath.Join(s.InstallPath, fileName)
		logger.Warnf("Rolling back by removing: %s", targetPath)
		if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallDockerStep)(nil)
