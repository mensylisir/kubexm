package kube_proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type InstallKubeProxyStep struct {
	step.Base
	InstallPath string
	Permission  string
}

type InstallKubeProxyStepBuilder struct {
	step.Builder[InstallKubeProxyStepBuilder, *InstallKubeProxyStep]
}

func NewInstallKubeProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallKubeProxyStepBuilder {
	s := &InstallKubeProxyStep{
		InstallPath: common.DefaultBinDir,
		Permission:  "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-proxy binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeProxyStepBuilder).Init(s)
	return b
}

func (b *InstallKubeProxyStepBuilder) WithInstallPath(installPath string) *InstallKubeProxyStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (b *InstallKubeProxyStepBuilder) WithPermission(permission string) *InstallKubeProxyStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallKubeProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeProxyStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentKubeProxy, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get kube-proxy binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("kube-proxy is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-proxy binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	return binaryInfo.FilePath(), nil
}

func (s *InstallKubeProxyStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "kube-proxy")
}

func (s *InstallKubeProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("kube-proxy not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
	}

	targetPath := s.getRemoteTargetPath()
	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, targetPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", targetPath, err)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", targetPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Installation is required.", targetPath)
	}

	return isDone, nil
}

func (s *InstallKubeProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to get local source path")
		return result, err
	}
	targetPath := s.getRemoteTargetPath()

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		err = fmt.Errorf("local source file '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localSourcePath)
		result.MarkFailed(err, "local source file not found")
		return result, err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.InstallPath, "0755", s.Sudo); err != nil {
		err = fmt.Errorf("failed to create remote install directory '%s': %w", s.InstallPath, err)
		result.MarkFailed(err, "failed to create remote install directory")
		return result, err
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("kube-proxy-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		err = fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
		result.MarkFailed(err, "failed to create remote upload directory")
		return result, err
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "kube-proxy")
	logger.Infof("Uploading kube-proxy to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		err = fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
		result.MarkFailed(err, "failed to upload kube-proxy")
		return result, err
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetPath)
	logger.Infof("Moving file to %s on remote host", targetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		err = fmt.Errorf("failed to move file to '%s': %w", targetPath, err)
		result.MarkFailed(err, "failed to move file to target path")
		return result, err
	}

	logger.Infof("Setting permissions for %s to %s", targetPath, s.Permission)
	if err := runner.Chmod(ctx.GoContext(), conn, targetPath, s.Permission, s.Sudo); err != nil {
		err = fmt.Errorf("failed to set permission on '%s': %w", targetPath, err)
		result.MarkFailed(err, "failed to set permission")
		return result, err
	}

	logger.Infof("Successfully installed kube-proxy to %s", targetPath)
	result.MarkCompleted("kube-proxy installed successfully")
	return result, nil
}

func (s *InstallKubeProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	targetPath := s.getRemoteTargetPath()
	logger.Warnf("Rolling back by removing: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallKubeProxyStep)(nil)
