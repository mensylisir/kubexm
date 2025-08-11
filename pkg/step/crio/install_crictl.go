package crio

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallCrictlStep struct {
	step.Base
}

type InstallCrictlStepBuilder struct {
	step.Builder[InstallCrictlStepBuilder, *InstallCrictlStep]
}

func NewInstallCrictlStepBuilder(ctx runtime.Context, instanceName string) *InstallCrictlStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(ComponentCrio, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallCrictlStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install crictl CLI from CRI-O package", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCrictlStepBuilder).Init(s)
	return b
}

func (s *InstallCrictlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCrictlStep) getExtractedSourceDir(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(ComponentCrio, arch)
	if err != nil {
		return "", err
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("CRI-O binary info not found for arch %s", arch)
	}

	sourcePath := binaryInfo.FilePath()
	destPath := filepath.Join(filepath.Dir(sourcePath), "cri-o", "bin")
	return destPath, nil
}

func (s *InstallCrictlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := filepath.Join(common.DefaultBinPath, "crictl")
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for crictl: %w", err)
	}

	if exists {
		logger.Infof("crictl binary already exists at %s.", targetPath)
		return true, nil
	}

	logger.Info("crictl binary not found. Installation is required.")
	return false, nil
}

func (s *InstallCrictlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	sourceDir, err := s.getExtractedSourceDir(ctx)
	if err != nil {
		return err
	}

	sourceFile := filepath.Join(sourceDir, "crictl")
	targetFile := filepath.Join(common.DefaultBinPath, "crictl")

	targetDir := filepath.Dir(targetFile)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory '%s': %w", targetDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("crictl-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "crictl")
	logger.Debugf("Uploading crictl to %s:%s", ctx.GetHost().GetName(), remoteTempPath)
	if err := runner.Upload(ctx.GoContext(), conn, sourceFile, remoteTempPath, false); err != nil {
		return fmt.Errorf("failed to upload '%s': %w", sourceFile, err)
	}

	moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, targetFile)
	logger.Debugf("Moving crictl to %s", targetFile)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move file to '%s': %w", targetFile, err)
	}

	logger.Debugf("Setting permissions for %s to 0755", targetFile)
	if err := runner.Chmod(ctx.GoContext(), conn, targetFile, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to set permission on '%s': %w", targetFile, err)
	}

	logger.Info("crictl installed successfully.")
	return nil
}

func (s *InstallCrictlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetFile := filepath.Join(common.DefaultBinPath, "crictl")
	logger.Warnf("Rolling back by removing: %s", targetFile)
	if err := runner.Remove(ctx.GoContext(), conn, targetFile, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s' during rollback: %v", targetFile, err)
		}
	}

	return nil
}

var _ step.Step = (*InstallCrictlStep)(nil)
