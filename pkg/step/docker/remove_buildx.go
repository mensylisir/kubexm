package docker

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

type RemoveBuildxStep struct {
	step.Base
	InstallPath string
}

type RemoveBuildxStepBuilder struct {
	step.Builder[RemoveBuildxStepBuilder, *RemoveBuildxStep]
}

func NewRemoveBuildxStepBuilder(ctx runtime.Context, instanceName string) *RemoveBuildxStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentBuildx, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &RemoveBuildxStep{
		InstallPath: filepath.Join(common.DockerPluginsDir, "docker-buildx"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove docker buildx binary", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveBuildxStepBuilder).Init(s)
	return b
}

func (b *RemoveBuildxStepBuilder) WithInstallPath(installPath string) *RemoveBuildxStepBuilder {
	if installPath != "" {
		b.Step.InstallPath = installPath
	}
	return b
}

func (s *RemoveBuildxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveBuildxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.InstallPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.InstallPath, ctx.GetHost().GetName(), err)
	}

	if !exists {
		logger.Infof("Target file '%s' already removed. Step is done.", s.InstallPath)
		return true, nil
	}

	logger.Infof("Target file '%s' exists. Removal is required.", s.InstallPath)
	return false, nil
}

func (s *RemoveBuildxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Removing docker-buildx binary from %s", s.InstallPath)

	if err := runner.Remove(ctx.GoContext(), conn, s.InstallPath, s.Sudo, true); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("File '%s' was not found, assuming it was already removed.", s.InstallPath)
			return nil
		}
		return fmt.Errorf("failed to remove '%s': %w", s.InstallPath, err)
	}

	logger.Infof("Successfully removed docker-buildx binary from %s.", s.InstallPath)
	return nil
}

func (s *RemoveBuildxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a removal step is a no-op.")
	return nil
}

var _ step.Step = (*RemoveBuildxStep)(nil)
