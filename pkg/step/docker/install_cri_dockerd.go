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
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type InstallCriDockerdStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
	InstallPath string
	Permission  string
}

type InstallCriDockerdStepBuilder struct {
	step.Builder[InstallCriDockerdStepBuilder, *InstallCriDockerdStep]
}

func NewInstallCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *InstallCriDockerdStepBuilder {
	s := &InstallCriDockerdStep{
		Version:     common.DefaultCriDockerdVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        helpers.GetZone(),
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install cri-dockerd binaries for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCriDockerdStepBuilder).Init(s)
	return b
}

func (b *InstallCriDockerdStepBuilder) WithInstallPath(installPath string) *InstallCriDockerdStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *InstallCriDockerdStepBuilder) WithPermission(permission string) *InstallCriDockerdStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *InstallCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCriDockerdStep) getExtractedPathOnControlNode() (string, error) {
	provider := helpers.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		helpers.ComponentCriDockerd,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get cri-dockerd binary info: %w", err)
	}

	destDirName := strings.TrimSuffix(binaryInfo.FileName, ".tar.gz")
	destPath := filepath.Join(common.DefaultExtractTmpDir, destDirName)
	return destPath, nil
}

func (s *InstallCriDockerdStep) filesToInstall() map[string]struct {
	Target string
	Perms  string
} {
	return map[string]struct {
		Target string
		Perms  string
	}{
		"cri-dockerd": {Target: filepath.Join(s.InstallPath, "cri-dockerd"), Perms: s.Permission},
	}
}

func (s *InstallCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	files := s.filesToInstall()
	for _, details := range files {
		exists, err := runner.Exists(ctx.GoContext(), conn, details.Target)
		if err != nil {
			return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", details.Target, ctx.GetHost().GetName(), err)
		}
		if !exists {
			logger.Infof("Target file '%s' does not exist. Installation is required.", details.Target)
			return false, nil
		}
	}

	logger.Info("All required cri-dockerd files already exist on remote host. Step is done.")
	return true, nil
}

func (s *InstallCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getExtractedPathOnControlNode()
	if err != nil {
		return fmt.Errorf("could not determine local extracted path: %w", err)
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	files := s.filesToInstall()

	for sourceRelPath, details := range files {
		localSourcePath := filepath.Join(localExtractedPath, sourceRelPath)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			logger.Warnf("Local source file '%s' not found, skipping its installation.", localSourcePath)
			continue
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, filepath.Base(localSourcePath))
		logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

		if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Base.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
		}
		installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", details.Perms, remoteTempPath, details.Target)
		logger.Infof("Installing file to %s on remote host", details.Target)

		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
			runner.Remove(ctx.GoContext(), conn, remoteTempPath, false, false)
			return fmt.Errorf("failed to install file '%s' on remote host: %w", details.Target, err)
		}
	}

	return nil
}

func (s *InstallCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove files: %v", err)
		return nil
	}

	files := s.filesToInstall()
	for _, details := range files {
		if details.Target == "" {
			continue
		}
		logger.Warnf("Rolling back by removing: %s", details.Target)
		if err := runner.Remove(ctx.GoContext(), conn, details.Target, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", details.Target, err)
		}
	}
	return nil
}
