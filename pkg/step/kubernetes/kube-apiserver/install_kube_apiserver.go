package kube_apiserver

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
)

type InstallKubeApiServerStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
	InstallPath string
	Permission  string
}

type InstallKubeApiServerStepBuilder struct {
	step.Builder[InstallKubeApiServerStepBuilder, *InstallKubeApiServerStep]
}

func NewInstallKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeApiServerStepBuilder {
	s := &InstallKubeApiServerStep{
		Version:     common.DefaultKubernetesVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        helpers.GetZone(),
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-apiserver binaries for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeApiServerStepBuilder).Init(s)
	return b
}

func (b *InstallKubeApiServerStepBuilder) WithInstallPath(installPath string) *InstallKubeApiServerStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *InstallKubeApiServerStepBuilder) WithPermission(permission string) *InstallKubeApiServerStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *InstallKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeApiServerStep) getExtractedPathOnControlNode() (string, error) {
	provider := helpers.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		helpers.ComponentKubeApiServer,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get kubernetes binary info: %w", err)
	}
	return binaryInfo.FilePath, nil
}

func (s *InstallKubeApiServerStep) filesToInstall() map[string]struct {
	Target string
	Perms  string
} {
	return map[string]struct {
		Target string
		Perms  string
	}{
		"kube-apiserver": {Target: filepath.Join(s.InstallPath, "kube-apiserver"), Perms: s.Permission},
	}
}

func (s *InstallKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	logger.Info("All required kube-apiserver files already exist on remote host. Step is done.")
	return true, nil
}

func (s *InstallKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
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

	for _, details := range files {
		if _, err := os.Stat(localExtractedPath); os.IsNotExist(err) {
			logger.Warnf("Local source file '%s' not found, skipping its installation.", localExtractedPath)
			continue
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, filepath.Base(localExtractedPath))
		logger.Infof("Uploading %s to %s:%s", localExtractedPath, ctx.GetHost().GetName(), remoteTempPath)

		if err := runner.Upload(ctx.GoContext(), conn, localExtractedPath, remoteTempPath, s.Base.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s' to '%s': %w", localExtractedPath, remoteTempPath, err)
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

func (s *InstallKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
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
