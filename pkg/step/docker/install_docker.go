package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallDockerStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
	InstallPath string
	Permission  string
}

type InstallDockerStepBuilder struct {
	step.Builder[InstallDockerStepBuilder, *InstallDockerStep]
}

func NewInstallDockerStepBuilder(ctx runtime.Context, instanceName string) *InstallDockerStepBuilder {
	s := &InstallDockerStep{
		Version:     common.DefaultDockerVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        util.GetZone(),
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Docker binaries for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallDockerStepBuilder).Init(s)
	return b
}

func (b *InstallDockerStepBuilder) WithVersion(version string) *InstallDockerStepBuilder {
	if version != "" {
		b.Step.Version = version
		b.Step.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Docker binaries for version %s", b.Step.Base.Meta.Name, b.Step.Version)
	}
	return b
}

func (b *InstallDockerStepBuilder) WithArch(arch string) *InstallDockerStepBuilder {
	if arch != "" {
		b.Step.Arch = arch
	}
	return b
}

func (b *InstallDockerStepBuilder) WithInstallPath(installPath string) *InstallDockerStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *InstallDockerStepBuilder) WithPermission(permission string) *InstallDockerStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *InstallDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallDockerStep) getExtractedPathOnControlNode() (string, error) {
	provider := util.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		util.ComponentDocker,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get docker binary info: %w", err)
	}

	destDirName := strings.TrimSuffix(binaryInfo.FileName, ".tgz")
	destPath := filepath.Join(common.DefaultExtractTmpDir, destDirName, "docker")
	return destPath, nil
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
	for _, fileName := range files {
		targetPath := filepath.Join(s.InstallPath, fileName)
		exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
		if err != nil {
			return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", targetPath, ctx.GetHost().GetName(), err)
		}
		if !exists {
			logger.Infof("Target file '%s' does not exist. Installation is required.", targetPath)
			return false, nil
		}
	}

	logger.Info("All required docker files already exist on remote host. Step is done.")
	return true, nil
}

func (s *InstallDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourceDir, err := s.getExtractedPathOnControlNode()
	if err != nil {
		return fmt.Errorf("could not determine local extracted path: %w", err)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	files := s.filesToInstall()
	for _, fileName := range files {
		localSourcePath := filepath.Join(localSourceDir, fileName)
		targetPath := filepath.Join(s.InstallPath, fileName)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			return fmt.Errorf("local source file '%s' not found, please ensure the extract step ran correctly", localSourcePath)
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, fileName)
		logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

		if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Base.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
		}

		installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.Permission, remoteTempPath, targetPath)
		logger.Infof("Installing file to %s on remote host", targetPath)

		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
			runner.Remove(ctx.GoContext(), conn, remoteTempPath, s.Sudo, true)
			return fmt.Errorf("failed to install file '%s' on remote host: %w", targetPath, err)
		}
		runner.Remove(ctx.GoContext(), conn, remoteTempPath, s.Sudo, true)
	}

	logger.Info("Successfully installed all Docker binaries.")
	return nil
}

func (s *InstallDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove files: %v", err)
		return nil
	}

	files := s.filesToInstall()
	for _, fileName := range files {
		targetPath := filepath.Join(s.InstallPath, fileName)
		logger.Warnf("Rolling back by removing: %s", targetPath)
		if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, true); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
		}
	}
	return nil
}
