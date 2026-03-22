package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type RemoveHelmStep struct {
	step.Base
	InstallPath string
}

type RemoveHelmStepBuilder struct {
	step.Builder[RemoveHelmStepBuilder, *RemoveHelmStep]
}

func NewRemoveHelmStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveHelmStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHelm, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &RemoveHelmStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove helm binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveHelmStepBuilder).Init(s)
	return b
}

func (b *RemoveHelmStepBuilder) WithInstallPath(installPath string) *RemoveHelmStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (s *RemoveHelmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveHelmStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "helm")
}

func (s *RemoveHelmStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := s.getRemoteTargetPath()
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", targetPath, ctx.GetHost().GetName(), err)
	}

	if !exists {
		logger.Infof("Target file '%s' already removed. Step is done.", targetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' exists. Removal is required.", targetPath)
	return false, nil
}

func (s *RemoveHelmStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	targetPath := s.getRemoteTargetPath()
	logger.Infof("Removing helm binary from %s", targetPath)

	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, true); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("File '%s' was not found, assuming it was already removed.", targetPath)
			result.MarkCompleted("helm binary already removed")
			return result, nil
		}
		err := fmt.Errorf("failed to remove '%s': %w", targetPath, err)
		result.MarkFailed(err, "failed to remove helm binary")
		return result, err
	}

	logger.Infof("Successfully removed helm binary from %s.", targetPath)
	result.MarkCompleted("helm binary removed successfully")
	return result, nil
}

func (s *RemoveHelmStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a removal step is a no-op.")
	return nil
}

var _ step.Step = (*RemoveHelmStep)(nil)
