package containerd

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
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallCriCtlStep struct {
	step.Base
	RemoteCriCtlTargetPath string
	CrictlPermissions      string
}

type InstallCriCtlStepBuilder struct {
	step.Builder[InstallCriCtlStepBuilder, *InstallCriCtlStep]
}

func NewInstallCriCtlStepBuilder(ctx runtime.Context, instanceName string) *InstallCriCtlStepBuilder {
	s := &InstallCriCtlStep{
		RemoteCriCtlTargetPath: filepath.Join(common.DefaultBinDir, "crictl"),
		CrictlPermissions:      "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install crictl binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCriCtlStepBuilder).Init(s)
	return b
}

func (b *InstallCriCtlStepBuilder) WithRemoteCriCtlTargetPath(remoteCriCtlTargetPath string) *InstallCriCtlStepBuilder {
	if remoteCriCtlTargetPath != "" {
		b.Step.RemoteCriCtlTargetPath = remoteCriCtlTargetPath
	}
	return b
}

func (b *InstallCriCtlStepBuilder) WithCrictlPermissions(permission string) *InstallCriCtlStepBuilder {
	if permission != "" {
		b.Step.CrictlPermissions = permission
	}
	return b
}

func (s *InstallCriCtlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCriCtlStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentCriCtl, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get crictl binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("crictl is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install crictl binary (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tar.gz")
	return filepath.Join(ctx.GetExtractDir(), destDirName), nil
}

func (s *InstallCriCtlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteCriCtlTargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.RemoteCriCtlTargetPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists. Step is done.", s.RemoteCriCtlTargetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Installation is required.", s.RemoteCriCtlTargetPath)
	return false, nil
}

func (s *InstallCriCtlStep) Run(ctx runtime.ExecutionContext) error {
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

	localSourcePath := filepath.Join(localExtractedPath, "crictl")

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, ensure extract step ran successfully", localSourcePath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, common.DefaultBinDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", common.DefaultBinDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("crictl-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "crictl")
	logger.Infof("Uploading crictl to %s:%s", ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, false); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, s.RemoteCriCtlTargetPath)
	logger.Infof("Moving file to %s on remote host", s.RemoteCriCtlTargetPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move file to '%s': %w", s.RemoteCriCtlTargetPath, err)
	}

	logger.Infof("Setting permissions for %s to %s", s.RemoteCriCtlTargetPath, s.CrictlPermissions)
	if err := runner.Chmod(ctx.GoContext(), conn, s.RemoteCriCtlTargetPath, s.CrictlPermissions, s.Sudo); err != nil {
		return fmt.Errorf("failed to set permission on '%s': %w", s.RemoteCriCtlTargetPath, err)
	}

	logger.Info("crictl binary has been installed successfully.")
	return nil
}

func (s *InstallCriCtlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.RemoteCriCtlTargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteCriCtlTargetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteCriCtlTargetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallCriCtlStep)(nil)
