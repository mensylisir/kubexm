package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/pkg/util"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallCNIPluginsStep struct {
	step.Base
	Version         string
	Arch            string
	WorkDir         string
	ClusterName     string
	Zone            string
	RemoteCNIBinDir string
	Permission      string
}

type InstallCNIPluginsStepBuilder struct {
	step.Builder[InstallCNIPluginsStepBuilder, *InstallCNIPluginsStep]
}

func NewInstallCNIPluginsStepBuilder(ctx runtime.Context, instanceName string) *InstallCNIPluginsStepBuilder {
	s := &InstallCNIPluginsStep{
		Version:         common.DefaultCNIPluginsVersion,
		Arch:            "",
		WorkDir:         ctx.GetGlobalWorkDir(),
		ClusterName:     ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:            util.GetZone(),
		RemoteCNIBinDir: common.DefaultCNIBin,
		Permission:      "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCNIPluginsStepBuilder).Init(s)
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithVersion(version string) *InstallCNIPluginsStepBuilder {
	b.Step.Version = version
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithRemoteCNIBinDir(remoteCNIBinDir string) *InstallCNIPluginsStepBuilder {
	b.Step.RemoteCNIBinDir = remoteCNIBinDir
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithPermission(permission string) *InstallCNIPluginsStepBuilder {
	b.Step.Permission = permission
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
	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tgz")
	return filepath.Join(common.DefaultExtractTmpDir, destDirName), nil
}

func (s *InstallCNIPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	dirExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteCNIBinDir)
	if err != nil {
		return false, err
	}
	if !dirExists {
		logger.Infof("Target directory '%s' does not exist. Installation is required.", s.RemoteCNIBinDir)
		return false, nil
	}

	keyPluginPath := filepath.Join(s.RemoteCNIBinDir, "bridge")
	pluginExists, err := runner.Exists(ctx.GoContext(), conn, keyPluginPath)
	if err != nil {
		return false, err
	}
	if !pluginExists {
		logger.Infof("Key CNI plugin '%s' does not exist. Installation is required.", keyPluginPath)
		return false, nil
	}

	logger.Info("CNI plugins directory and key plugin already exist. Step is done.")
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
		return fmt.Errorf("local CNI extracted path '%s' not found, please run extract step first", localExtractedPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteCNIBinDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote CNI bin directory '%s': %w", s.RemoteCNIBinDir, err)
	}
	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

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
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remoteTempPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localPath, err)
		}

		installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.Permission, remoteTempPath, remoteTargetPath)
		logger.Infof("Installing CNI plugin to %s on remote host", remoteTargetPath)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
			runner.Remove(ctx.GoContext(), conn, remoteTempPath, false, false)
			return fmt.Errorf("failed to install CNI plugin '%s': %w", fileName, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error during CNI plugin installation walk: %w", err)
	}

	return nil
}

func (s *InstallCNIPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing directory: %s", s.RemoteCNIBinDir)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteCNIBinDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteCNIBinDir, err)
	}

	return nil
}

var _ step.Step = (*InstallCNIPluginsStep)(nil)
