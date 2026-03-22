package calico

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type RemoveCalicoctlStep struct {
	step.Base
	InstallPath string
}

type RemoveCalicoctlStepBuilder struct {
	step.Builder[RemoveCalicoctlStepBuilder, *RemoveCalicoctlStep]
}

func NewRemoveCalicoctlStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveCalicoctlStepBuilder {
	provider := binary.NewBinaryProvider(ctx)

	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCalicoCtl, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &RemoveCalicoctlStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove calicoctl binary from system path", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveCalicoctlStepBuilder).Init(s)
	return b
}

func (b *RemoveCalicoctlStepBuilder) WithInstallPath(installPath string) *RemoveCalicoctlStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (s *RemoveCalicoctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveCalicoctlStep) getRemoteTargetPath() string {
	return filepath.Join(s.InstallPath, "calicoctl")
}

func (s *RemoveCalicoctlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, errors.Wrap(err, "failed to get connector for precheck")
	}

	targetPath := s.getRemoteTargetPath()
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check for file '%s' on host %s", targetPath, ctx.GetHost().GetName())
	}

	if !exists {
		logger.Infof("Target file '%s' already removed. Step is done.", targetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' exists. Removal is required.", targetPath)
	return false, nil
}

func (s *RemoveCalicoctlStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	targetPath := s.getRemoteTargetPath()
	logger.Infof("Removing calicoctl binary from %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to remove '%s'", targetPath))
		return result, err
	}

	logger.Infof("Successfully removed calicoctl binary from %s.", targetPath)
	result.MarkCompleted("calicoctl binary removed successfully")
	return result, nil
}

func (s *RemoveCalicoctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a removal step is a no-op, as re-installing the binary is not the desired behavior.")
	return nil
}

var _ step.Step = (*RemoveCalicoctlStep)(nil)
