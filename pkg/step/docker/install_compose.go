package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallDockerComposeStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
	InstallPath string
	Permission  string
}

type InstallDockerComposeStepBuilder struct {
	step.Builder[InstallDockerComposeStepBuilder, *InstallDockerComposeStep]
}

func NewInstallDockerComposeStepBuilder(ctx runtime.Context, instanceName string) *InstallDockerComposeStepBuilder {

	s := &InstallDockerComposeStep{
		Version:     common.DefaultDockerComposeVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        util.GetZone(),
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker-compose binary for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallDockerComposeStepBuilder).Init(s)
	return b
}

func (b *InstallDockerComposeStepBuilder) WithVersion(version string) *InstallDockerComposeStepBuilder {
	if version != "" {
		b.Step.Version = version
		b.Step.Base.Meta.Description = fmt.Sprintf("[%s]>>Install docker-compose binary for version %s", b.Step.Base.Meta.Name, b.Step.Version)
	}
	return b
}

func (b *InstallDockerComposeStepBuilder) WithArch(arch string) *InstallDockerComposeStepBuilder {
	if arch != "" {
		b.Step.Arch = arch
	}
	return b
}

func (b *InstallDockerComposeStepBuilder) WithInstallPath(installPath string) *InstallDockerComposeStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (s *InstallDockerComposeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallDockerComposeStep) getLocalSourcePath() (string, error) {
	provider := util.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(util.ComponentCompose, s.Version, s.Arch, s.Zone, s.WorkDir, s.ClusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get docker-compose binary info: %w", err)
	}
	return binaryInfo.FilePath, nil
}

func (s *InstallDockerComposeStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "docker-compose")
}

func (s *InstallDockerComposeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
	if !exists {
		return false, nil
	}
	return true, nil
}

func (s *InstallDockerComposeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath()
	if err != nil {
		return fmt.Errorf("could not determine local source path: %w", err)
	}
	targetPath := s.getRemoteTargetPath()

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, please run download step first", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "docker-compose")
	logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}
	defer runner.Remove(ctx.GoContext(), conn, remoteTempPath, s.Sudo, true)

	installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.Permission, remoteTempPath, targetPath)
	logger.Infof("Installing file to %s on remote host", targetPath)

	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to install file '%s' on remote host: %w", targetPath, err)
	}

	logger.Infof("Successfully installed docker-compose to %s", targetPath)
	return nil
}

func (s *InstallDockerComposeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetPath := s.getRemoteTargetPath()
	logger.Warnf("Rolling back by removing: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}
	return nil
}

var _ step.Step = (*InstallDockerComposeStep)(nil)
