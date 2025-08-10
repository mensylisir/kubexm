package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

type InstallEtcdStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallEtcdStepBuilder struct {
	step.Builder[InstallEtcdStepBuilder, *InstallEtcdStep]
}

func NewInstallEtcdStepBuilder(ctx runtime.Context, instanceName string) *InstallEtcdStepBuilder {
	s := &InstallEtcdStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install etcd binaries", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(InstallEtcdStepBuilder).Init(s)
	return b
}

func (b *InstallEtcdStepBuilder) WithInstallPath(installPath string) *InstallEtcdStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallEtcdStepBuilder) WithPermission(permission string) *InstallEtcdStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallEtcdStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentEtcd, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get etcd binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("etcd is disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install etcd binaries (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	extractedDir := filepath.Dir(binaryInfo.FilePath())
	innerDir := "etcd"
	return filepath.Join(extractedDir, innerDir), nil
}

func (s *InstallEtcdStep) filesToInstall() map[string]string {
	return map[string]string{
		"etcd":    filepath.Join(s.InstallPath, "etcd"),
		"etcdctl": filepath.Join(s.InstallPath, "etcdctl"),
		"etcdutl": filepath.Join(s.InstallPath, "etcdutl"),
	}
}

func (s *InstallEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourceDir, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("etcd not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	files := s.filesToInstall()
	allDone := true
	for sourceRelPath, remoteTargetPath := range files {
		localSourcePath := filepath.Join(localSourceDir, sourceRelPath)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			return false, fmt.Errorf("local source file '%s' not found, ensure extract step ran successfully", localSourcePath)
		}

		isDone, err := helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, remoteTargetPath, s.Sudo)
		if err != nil {
			return false, fmt.Errorf("failed to check remote file integrity for %s: %w", remoteTargetPath, err)
		}

		if !isDone {
			logger.Infof("Target file '%s' is missing or outdated. Installation is required.", remoteTargetPath)
			allDone = false
			break
		}
	}

	if allDone {
		logger.Info("All required etcd binaries already exist and are up-to-date. Step is done.")
	}

	return allDone, nil
}

func (s *InstallEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(localExtractedPath); os.IsNotExist(err) {
		return fmt.Errorf("local etcd extracted path '%s' not found, ensure extract step ran successfully", localExtractedPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("etcd-%d", time.Now().UnixNano()))
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

		remoteTempPath := filepath.Join(remoteUploadTmpDir, sourceRelPath)
		logger.Infof("Uploading %s to %s:%s", sourceRelPath, ctx.GetHost().GetName(), remoteTempPath)
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

	logger.Info("Successfully installed all etcd binaries.")
	return nil
}

func (s *InstallEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
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

var _ step.Step = (*InstallEtcdStep)(nil)
