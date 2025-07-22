package etcd

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

type InstallEtcdStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
	InstallPath string
	Permission  string
}

type InstallEtcdStepBuilder struct {
	step.Builder[InstallEtcdStepBuilder, *InstallEtcdStep]
}

func NewInstallEtcdStepBuilder(ctx runtime.Context, instanceName string) *InstallEtcdStepBuilder {
	etcdVersion := common.DefaultEtcdVersion
	if ctx.GetClusterConfig().Spec.Etcd != nil && ctx.GetClusterConfig().Spec.Etcd.Version != "" {
		etcdVersion = ctx.GetClusterConfig().Spec.Etcd.Version
	}

	s := &InstallEtcdStep{
		Version:     etcdVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        helpers.GetZone(),
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install etcd binaries for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(InstallEtcdStepBuilder).Init(s)
	return b
}

func (b *InstallEtcdStepBuilder) WithInstallPath(installPath string) *InstallEtcdStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *InstallEtcdStepBuilder) WithPermission(permission string) *InstallEtcdStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *InstallEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallEtcdStep) getLocalExtractedPath() (string, error) {
	provider := helpers.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		helpers.ComponentEtcd,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get etcd binary info: %w", err)
	}
	destDirName := strings.TrimSuffix(binaryInfo.FileName, ".tar.gz")
	return filepath.Join(common.DefaultExtractTmpDir, destDirName), nil
}

func (s *InstallEtcdStep) filesToInstall() map[string]struct {
	Target string
	Perms  string
} {
	return map[string]struct {
		Target string
		Perms  string
	}{
		"etcd":    {Target: filepath.Join(s.InstallPath, "etcd"), Perms: s.Permission},
		"etcdctl": {Target: filepath.Join(s.InstallPath, "etcdctl"), Perms: s.Permission},
	}
}

func (s *InstallEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			return false, fmt.Errorf("failed to check for file '%s': %w", details.Target, err)
		}
		if !exists {
			logger.Infof("Target file '%s' does not exist. Installation is required.", details.Target)
			return false, nil
		}
	}

	logger.Info("All required etcd binaries already exist on remote host. Step is done.")
	return true, nil
}

func (s *InstallEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getLocalExtractedPath()
	if err != nil {
		return err
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	files := s.filesToInstall()
	for sourceRelPath, details := range files {
		localSourcePath := filepath.Join(localExtractedPath, sourceRelPath)

		if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
			return fmt.Errorf("local source file '%s' not found, did extract step run correctly?", localSourcePath)
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, sourceRelPath)
		logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

		if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Sudo); err != nil {
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

func (s *InstallEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	files := s.filesToInstall()
	for _, details := range files {
		logger.Warnf("Rolling back by removing: %s", details.Target)
		if err := runner.Remove(ctx.GoContext(), conn, details.Target, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", details.Target, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallEtcdStep)(nil)
