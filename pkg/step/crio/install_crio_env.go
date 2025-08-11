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

type InstallCrioEnvStep struct {
	step.Base
}

type InstallCrioEnvStepBuilder struct {
	step.Builder[InstallCrioEnvStepBuilder, *InstallCrioEnvStep]
}

func NewInstallCrioEnvStepBuilder(ctx runtime.Context, instanceName string) *InstallCrioEnvStepBuilder {
	s := &InstallCrioEnvStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CRI-O environment file for systemd", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(InstallCrioEnvStepBuilder).Init(s)
	return b
}

func (s *InstallCrioEnvStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCrioEnvStep) getExtractedSourcePath(ctx runtime.ExecutionContext) (string, error) {
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
	return filepath.Join(filepath.Dir(sourcePath), "cri-o", "etc", "crio"), nil
}

func (s *InstallCrioEnvStep) getTargetEnvPath(ctx runtime.ExecutionContext) (string, error) {
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return "", fmt.Errorf("failed to get host facts: %w", err)
	}

	for _, distro := range common.DebianFamilyDistributions {
		if facts.OS.ID == distro {
			return "/etc/default/crio", nil
		}
	}
	for _, distro := range common.RedHatFamilyDistributions {
		if facts.OS.ID == distro {
			return "/etc/sysconfig/crio", nil
		}
	}

	ctx.GetLogger().Warnf("Unsupported OS distribution '%s' for CRI-O environment file, defaulting to /etc/sysconfig/crio.", facts.OS.ID)
	return "/etc/sysconfig/crio", nil
}

func (s *InstallCrioEnvStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath, err := s.getTargetEnvPath(ctx)
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for env file '%s': %w", targetPath, err)
	}
	return exists, nil
}

func (s *InstallCrioEnvStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	sourceFile, err := s.getExtractedSourcePath(ctx)
	if err != nil {
		return err
	}
	targetFile, err := s.getTargetEnvPath(ctx)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(targetFile)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory '%s': %w", targetDir, err)
	}

	err = helpers.WriteContentToRemote(ctx, conn, string(content), targetFile, "0644", s.Sudo)
	if err != nil {
		return err
	}
	logger.Infof("CRI-O environment file installed successfully to %s.", targetFile)
	return nil
}

func (s *InstallCrioEnvStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	targetFile, err := s.getTargetEnvPath(ctx)
	if err != nil {
		logger.Warnf("Could not determine target path for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", targetFile)
	if err := runner.Remove(ctx.GoContext(), conn, targetFile, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s': %v", targetFile, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallCrioEnvStep)(nil)
