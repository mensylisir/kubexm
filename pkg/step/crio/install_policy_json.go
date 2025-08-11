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

type InstallPolicyJsonStep struct {
	step.Base
}

type InstallPolicyJsonStepBuilder struct {
	step.Builder[InstallPolicyJsonStepBuilder, *InstallPolicyJsonStep]
}

func NewInstallPolicyJsonStepBuilder(ctx runtime.Context, instanceName string) *InstallPolicyJsonStepBuilder {
	s := &InstallPolicyJsonStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CRI-O signature policy file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(InstallPolicyJsonStepBuilder).Init(s)
	return b
}

func (s *InstallPolicyJsonStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallPolicyJsonStep) getExtractedSourcePath(ctx runtime.ExecutionContext) (string, error) {
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
	return filepath.Join(filepath.Dir(sourcePath), "cri-o", "contrib", "policy.json"), nil
}

func (s *InstallPolicyJsonStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := common.SignaturePolicyPath
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for policy.json: %w", err)
	}
	return exists, nil
}

func (s *InstallPolicyJsonStep) Run(ctx runtime.ExecutionContext) error {
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
	targetFile := common.SignaturePolicyPath

	targetDir := filepath.Dir(targetFile)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory '%s': %w", targetDir, err)
	}

	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}
	err = helpers.WriteContentToRemote(ctx, conn, string(content), targetFile, "0644", s.Sudo)
	if err != nil {
		return err
	}
	logger.Info("CRI-O signature policy file installed successfully.")
	return nil
}

func (s *InstallPolicyJsonStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}
	targetFile := common.SignaturePolicyPath
	logger.Warnf("Rolling back by removing: %s", targetFile)
	if err := runner.Remove(ctx.GoContext(), conn, targetFile, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s': %v", targetFile, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallPolicyJsonStep)(nil)
