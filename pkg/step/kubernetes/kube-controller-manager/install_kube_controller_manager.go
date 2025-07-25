package kube_controller_manager

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

type InstallKubeControllerManagerStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	InstallPath string
	FileName    string
	Permission  string
}

type InstallKubeControllerManagerStepBuilder struct {
	step.Builder[InstallKubeControllerManagerStepBuilder, *InstallKubeControllerManagerStep]
}

func NewInstallKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeControllerManagerStepBuilder {
	s := &InstallKubeControllerManagerStep{
		Version:     ctx.GetClusterConfig().Spec.Kubernetes.Version,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		InstallPath: common.DefaultBinDir,
		FileName:    "kube-controller-manager",
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-controller-manager binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (b *InstallKubeControllerManagerStepBuilder) WithInstallPath(path string) *InstallKubeControllerManagerStepBuilder {
	b.Step.InstallPath = path
	return b
}

func (s *InstallKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeControllerManagerStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := helpers.NewBinaryProvider()

	host := ctx.GetHost()
	arch := s.Arch
	if arch == "" {
		arch = host.GetArch()
	}

	info, err := provider.GetBinaryInfo(helpers.ComponentKubeControllerManager, s.Version, arch, "", s.WorkDir, s.ClusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get binary info for kube-controller-manager: %w", err)
	}
	return info.FilePath, nil
}

func (s *InstallKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := filepath.Join(s.InstallPath, s.FileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", targetPath, ctx.GetHost().GetName(), err)
	}
	if !exists {
		logger.Infof("Target file '%s' does not exist. Installation is required.", targetPath)
		return false, nil
	}

	logger.Infof("%s binary already exists on remote host. Step is done.", s.FileName)
	return true, nil
}

func (s *InstallKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, please ensure the download step ran successfully", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
	}

	targetPath := filepath.Join(s.InstallPath, s.FileName)
	remoteTempPath := filepath.Join(common.DefaultUploadTmpDir, s.FileName)

	logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)
	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Base.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.Permission, remoteTempPath, targetPath)
	logger.Infof("Installing file to %s on remote host", targetPath)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		_ = runner.Remove(ctx.GoContext(), conn, remoteTempPath, s.Sudo, false)
		return fmt.Errorf("failed to install file '%s' on remote host: %w", targetPath, err)
	}
	_ = runner.Remove(ctx.GoContext(), conn, remoteTempPath, s.Sudo, false)

	logger.Infof("%s binary installed successfully.", s.FileName)
	return nil
}

func (s *InstallKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetPath := filepath.Join(s.InstallPath, s.FileName)
	logger.Warnf("Rolling back by removing: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}
	return nil
}

var _ step.Step = (*InstallKubeControllerManagerStep)(nil)
