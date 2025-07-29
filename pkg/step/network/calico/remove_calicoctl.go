package calico

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type RemoveCalicoctlStep struct {
	step.Base
	InstallPath string
}

type RemoveCalicoctlStepBuilder struct {
	step.Builder[RemoveCalicoctlStepBuilder, *RemoveCalicoctlStep]
}

func NewRemoveCalicoctlStepBuilder(ctx runtime.Context, instanceName string) *RemoveCalicoctlStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCalicoCtl, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &RemoveCalicoctlStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove calicoctl binary", s.Base.Meta.Name)
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

func (s *RemoveCalicoctlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetPath := s.getRemoteTargetPath()
	logger.Infof("Removing calicoctl binary from %s", targetPath)

	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, true); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("File '%s' was not found, assuming it was already removed.", targetPath)
			return nil
		}
		return fmt.Errorf("failed to remove '%s': %w", targetPath, err)
	}

	logger.Infof("Successfully removed calicoctl binary from %s.", targetPath)
	return nil
}

func (s *RemoveCalicoctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a removal step is a no-op.")
	return nil
}

var _ step.Step = (*RemoveCalicoctlStep)(nil)
