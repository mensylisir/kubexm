package harbor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type RemoveHarborArtifactsStep struct {
	step.Base
	RemoteInstallDir string
}

type RemoveHarborArtifactsStepBuilder struct {
	step.Builder[RemoveHarborArtifactsStepBuilder, *RemoveHarborArtifactsStep]
}

func NewRemoveHarborArtifactsStepBuilder(ctx runtime.Context, instanceName string) *RemoveHarborArtifactsStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment

	if localCfg == nil || localCfg.Type != "harbor" {
		return nil
	}

	installRoot := "/opt"
	if localCfg.DataRoot != "" {
		installRoot = localCfg.DataRoot
	}

	s := &RemoveHarborArtifactsStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Harbor installation artifacts from the registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveHarborArtifactsStepBuilder).Init(s)
	return b
}

func (s *RemoveHarborArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveHarborArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteInstallDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for directory '%s' on host %s: %w", s.RemoteInstallDir, ctx.GetHost().GetName(), err)
	}

	if !exists {
		logger.Infof("Harbor installation directory '%s' already removed. Step is done.", s.RemoteInstallDir)
		return true, nil
	}

	logger.Infof("Harbor installation directory '%s' exists. Removal is required.", s.RemoteInstallDir)
	return false, nil
}

func (s *RemoveHarborArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Removing Harbor installation directory: %s", s.RemoteInstallDir)

	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteInstallDir, s.Sudo, true); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Directory '%s' was not found, assuming it was already removed.", s.RemoteInstallDir)
			return nil
		}
		return fmt.Errorf("failed to remove directory '%s': %w", s.RemoteInstallDir, err)
	}

	logger.Infof("Successfully removed Harbor installation directory: %s", s.RemoteInstallDir)
	return nil
}

func (s *RemoveHarborArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a removal step is a no-op.")
	return nil
}

var _ step.Step = (*RemoveHarborArtifactsStep)(nil)
