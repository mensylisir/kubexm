package containerd

import (
	"fmt"
	"io/fs"
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

type InstallCNIPluginsStep struct {
	step.Base
	RemoteCNIBinDir string
	Permission      string
}

type InstallCNIPluginsStepBuilder struct {
	step.Builder[InstallCNIPluginsStepBuilder, *InstallCNIPluginsStep]
}

func NewInstallCNIPluginsStepBuilder(ctx runtime.Context, instanceName string) *InstallCNIPluginsStepBuilder {
	s := &InstallCNIPluginsStep{
		RemoteCNIBinDir: common.DefaultCNIBin,
		Permission:      "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCNIPluginsStepBuilder).Init(s)
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithRemoteCNIBinDir(remoteCNIBinDir string) *InstallCNIPluginsStepBuilder {
	if remoteCNIBinDir != "" {
		b.Step.RemoteCNIBinDir = remoteCNIBinDir
	}
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithPermission(permission string) *InstallCNIPluginsStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallCNIPluginsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCNIPluginsStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get CNI plugins binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("CNI plugins are unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tgz")
	return filepath.Join(ctx.GetExtractDir(), destDirName), nil
}

func (s *InstallCNIPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	keyPluginPath := filepath.Join(s.RemoteCNIBinDir, "bridge")
	pluginExists, err := runner.Exists(ctx.GoContext(), conn, keyPluginPath)
	if err != nil {
		return false, err
	}
	if !pluginExists {
		logger.Infof("Key CNI plugin '%s' does not exist in '%s'. Installation is required.", keyPluginPath, s.RemoteCNIBinDir)
		return false, nil
	}

	logger.Info("Key CNI plugin already exists. Step is done.")
	return true, nil
}

func (s *InstallCNIPluginsStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("local CNI extracted path '%s' not found, ensure extract step ran successfully", localExtractedPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteCNIBinDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote CNI bin directory '%s': %w", s.RemoteCNIBinDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("cni-plugins-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	err = filepath.WalkDir(localExtractedPath, func(localPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		remoteTempPath := filepath.Join(remoteUploadTmpDir, fileName)
		remoteTargetPath := filepath.Join(s.RemoteCNIBinDir, fileName)

		logger.Infof("Uploading CNI plugin %s to %s:%s", fileName, ctx.GetHost().GetName(), remoteTempPath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remoteTempPath, false); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localPath, err)
		}

		moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, remoteTargetPath)
		logger.Infof("Moving CNI plugin to %s on remote host", remoteTargetPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move CNI plugin '%s': %w", fileName, err)
		}

		logger.Infof("Setting permissions for %s to %s", remoteTargetPath, s.Permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remoteTargetPath, s.Permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on CNI plugin '%s': %w", fileName, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error during CNI plugin installation walk: %w", err)
	}

	logger.Info("All CNI plugins have been installed successfully.")
	return nil
}

func (s *InstallCNIPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for CNI plugins installation is complex and not automatically performed to avoid deleting other CNI files. Manual cleanup of '/opt/cni/bin' may be required if installation failed partially.")
	return nil
}

var _ step.Step = (*InstallCNIPluginsStep)(nil)
