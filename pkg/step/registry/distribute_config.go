package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type DistributeRegistryConfigStep struct {
	step.Base
	LocalConfigPath  string
	RemoteConfigPath string
	Permission       string
}

type DistributeRegistryConfigStepBuilder struct {
	step.Builder[DistributeRegistryConfigStepBuilder, *DistributeRegistryConfigStep]
}

func NewDistributeRegistryConfigStepBuilder(ctx runtime.Context, instanceName string) *DistributeRegistryConfigStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment
	if localCfg == nil || localCfg.Type != "registry" {
		return nil

	}

	dataRoot := "/var/lib/registry"
	if localCfg.DataRoot != "" {
		dataRoot = localCfg.DataRoot
	}

	s := &DistributeRegistryConfigStep{
		LocalConfigPath:  filepath.Join(ctx.GetGlobalWorkDir(), "registry", "config.yml"),
		RemoteConfigPath: filepath.Join(filepath.Dir(dataRoot), "config.yml"),
		Permission:       "0644",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute generated registry config.yml to registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DistributeRegistryConfigStepBuilder).Init(s)
	return b
}

func (s *DistributeRegistryConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeRegistryConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if _, err := os.Stat(s.LocalConfigPath); os.IsNotExist(err) {
		return false, fmt.Errorf("local source file '%s' not found, ensure generate config step ran successfully", s.LocalConfigPath)
	}

	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, s.LocalConfigPath, s.RemoteConfigPath, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to check remote file integrity for %s: %w", s.RemoteConfigPath, err)
	}

	if isDone {
		logger.Infof("Target file '%s' already exists and is up-to-date. Step is done.", s.RemoteConfigPath)
	} else {
		logger.Infof("Target file '%s' is missing or outdated. Distribution is required.", s.RemoteConfigPath)
	}

	return isDone, nil
}

func (s *DistributeRegistryConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	contentBytes, err := os.ReadFile(s.LocalConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read local source file %s: %w", s.LocalConfigPath, err)
	}

	remoteDir := filepath.Dir(s.RemoteConfigPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory '%s': %w", remoteDir, err)
	}

	logger.Infof("Writing registry config to %s", s.RemoteConfigPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(contentBytes), s.RemoteConfigPath, s.Permission, s.Sudo); err != nil {
		return fmt.Errorf("failed to write remote registry config: %w", err)
	}

	logger.Infof("Successfully distributed registry config.yml to %s", s.RemoteConfigPath)
	return nil
}

func (s *DistributeRegistryConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing distributed config file: %s", s.RemoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteConfigPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteConfigPath, err)
	}

	return nil
}

var _ step.Step = (*DistributeRegistryConfigStep)(nil)
