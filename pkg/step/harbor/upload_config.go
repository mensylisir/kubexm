package harbor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type DistributeHarborConfigStep struct {
	step.Base
	RemoteInstallDir string
	Permission       string
}

type DistributeHarborConfigStepBuilder struct {
	step.Builder[DistributeHarborConfigStepBuilder, *DistributeHarborConfigStep]
}

func NewDistributeHarborConfigStepBuilder(ctx runtime.Context, instanceName string) *DistributeHarborConfigStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment
	if localCfg == nil || localCfg.Type != "harbor" {
		return nil
	}

	installRoot := "/opt"
	if localCfg.DataRoot != "" {
		installRoot = localCfg.DataRoot
	}

	s := &DistributeHarborConfigStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
		Permission:       "0644",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute generated harbor.yml to registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DistributeHarborConfigStepBuilder).Init(s)
	return b
}

func (s *DistributeHarborConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeHarborConfigStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get Harbor binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("Harbor is disabled for arch %s", arch)
	}

	sourceDir := filepath.Dir(binaryInfo.FilePath())
	innerDir := "harbor"
	localExtractedPath := filepath.Join(sourceDir, innerDir)

	return filepath.Join(localExtractedPath, "harbor.yml"), nil
}

func (s *DistributeHarborConfigStep) getRemoteTargetPath() string {
	return filepath.Join(s.RemoteInstallDir, "harbor.yml")
}

func (s *DistributeHarborConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("Harbor not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure generate config step ran successfully", localSourcePath)
	}

	targetPath := s.getRemoteTargetPath()
	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, localSourcePath, targetPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", targetPath, err)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", targetPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Distribution is required.", targetPath)
	}

	return isDone, nil
}

func (s *DistributeHarborConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("Harbor not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return nil
		}
		return err
	}

	contentBytes, err := os.ReadFile(localSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read local source file %s: %w", localSourcePath, err)
	}

	targetPath := s.getRemoteTargetPath()

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteInstallDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install directory '%s': %w", s.RemoteInstallDir, err)
	}

	logger.Infof("Writing harbor config to %s", targetPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(contentBytes), targetPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to write remote harbor config: %w", err)
	}

	logger.Infof("Successfully distributed harbor.yml to %s", targetPath)
	return nil
}

func (s *DistributeHarborConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	targetPath := s.getRemoteTargetPath()
	logger.Warnf("Rolling back by removing distributed config file: %s", targetPath)
	if err := runner.Remove(ctx.GoContext(), conn, targetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", targetPath, err)
	}

	return nil
}

var _ step.Step = (*DistributeHarborConfigStep)(nil)
