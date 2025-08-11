package crio

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

type InstallCrioServiceStep struct {
	step.Base
}

type InstallCrioServiceStepBuilder struct {
	step.Builder[InstallCrioServiceStepBuilder, *InstallCrioServiceStep]
}

func NewInstallCrioServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallCrioServiceStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(ComponentCrio, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil

	}

	s := &InstallCrioServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCrioServiceStepBuilder).Init(s)
	return b
}

func (s *InstallCrioServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCrioServiceStep) getExtractedSourcePath(ctx runtime.ExecutionContext) (string, error) {
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
	return filepath.Join(filepath.Dir(sourcePath), "cri-o", "contrib", "crio.service"), nil
}

func (s *InstallCrioServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := common.CRIODefaultSystemdFile
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for systemd service file '%s': %w", targetPath, err)
	}

	if exists {
		logger.Infof("CRI-O systemd service file already exists at %s.", targetPath)
		return true, nil
	}
	logger.Info("CRI-O systemd service file not found. Installation is required.")
	return false, nil
}

func (s *InstallCrioServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}

	sourceFile, err := s.getExtractedSourcePath(ctx)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	targetFile := common.CRIODefaultSystemdFile

	logger.Infof("Writing systemd service file to %s", targetFile)
	err = helpers.WriteContentToRemote(ctx, conn, string(content), targetFile, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write containerd service file: %w", err)
	}

	logger.Info("Reloading systemd daemon")
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	logger.Info("CRI-O systemd service installed and daemon reloaded successfully.")
	return nil
}

func (s *InstallCrioServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetFile := common.CRIODefaultSystemdFile
	logger.Warnf("Rolling back by removing: %s", targetFile)
	if err := runner.Remove(ctx.GoContext(), conn, targetFile, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s' during rollback: %v", targetFile, err)
		}
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}

	logger.Info("Reloading systemd daemon after rollback")
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	return nil
}

var _ step.Step = (*InstallCrioServiceStep)(nil)
