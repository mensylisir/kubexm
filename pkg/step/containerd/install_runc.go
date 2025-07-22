package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"path/filepath"
	"time"
)

type InstallRuncStep struct {
	step.Base
	Version              string
	Arch                 string
	WorkDir              string
	ClusterName          string
	Zone                 string
	RemoteRuncTargetPath string
	Permission           string
}

type InstallRuncStepBuilder struct {
	step.Builder[InstallRuncStepBuilder, *InstallRuncStep]
}

func NewInstallRuncStepBuilder(ctx runtime.Context, instanceName string) *InstallRuncStepBuilder {
	s := &InstallRuncStep{
		Version:              common.DefaultRuncVersion,
		Arch:                 "",
		WorkDir:              ctx.GetGlobalWorkDir(),
		ClusterName:          ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:                 helpers.GetZone(),
		RemoteRuncTargetPath: filepath.Join(common.DefaultBinDir, "runc"),
		Permission:           "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install runc binary for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallRuncStepBuilder).Init(s)
	return b
}

func (b *InstallRuncStepBuilder) WithVersion(version string) *InstallRuncStepBuilder {
	b.Step.Version = version
	return b
}

func (b *InstallRuncStepBuilder) WithRemoteRuncTargetPath(remoteRuncTargetPath string) *InstallRuncStepBuilder {
	b.Step.RemoteRuncTargetPath = remoteRuncTargetPath
	return b
}

func (b *InstallRuncStepBuilder) WithPermission(permission string) *InstallRuncStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *InstallRuncStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallRuncStep) getLocalSourcePath() (string, error) {
	provider := helpers.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		helpers.ComponentRunc,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get runc binary info: %w", err)
	}
	return binaryInfo.FilePath, nil
}

func (s *InstallRuncStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteRuncTargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.RemoteRuncTargetPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists. Step is done.", s.RemoteRuncTargetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Installation is required.", s.RemoteRuncTargetPath)
	return false, nil
}

func (s *InstallRuncStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, please run download step first", localSourcePath)
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "runc")
	logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.Permission, remoteTempPath, s.RemoteRuncTargetPath)
	logger.Infof("Installing file to %s on remote host", s.RemoteRuncTargetPath)

	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		runner.Remove(ctx.GoContext(), conn, remoteTempPath, false, false)
		return fmt.Errorf("failed to install file '%s' on remote host: %w", s.RemoteRuncTargetPath, err)
	}

	return nil
}

func (s *InstallRuncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.RemoteRuncTargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteRuncTargetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteRuncTargetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallRuncStep)(nil)
