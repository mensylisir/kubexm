package registry

import (
	"fmt"
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

type DistributeRegistryArtifactsStep struct {
	step.Base
	LocalConfigPath string
}

type DistributeRegistryArtifactsStepBuilder struct {
	step.Builder[DistributeRegistryArtifactsStepBuilder, *DistributeRegistryArtifactsStep]
}

func NewDistributeRegistryArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeRegistryArtifactsStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	if cfg.Registry.LocalDeployment == nil || cfg.Registry.LocalDeployment.Type != "registry" {
		return nil
	}

	s := &DistributeRegistryArtifactsStep{
		LocalConfigPath: filepath.Join(ctx.GetClusterArtifactsDir(), "registry", "config.yml"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute registry binary and configuration", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute
	b := new(DistributeRegistryArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeRegistryArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeRegistryArtifactsStep) getLocalBinaryPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, arch)
	if err != nil {
		return "", err
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("registry disabled for arch %s", arch)
	}

	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tar.gz")
	return filepath.Join(ctx.GetExtractDir(), destDirName, "registry"), nil
}

func (s *DistributeRegistryArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	remoteBinaryPath := filepath.Join(common.DefaultBinDir, "registry")
	remoteConfigPath := "/etc/docker/registry/config.yml"
	binaryExists, _ := runner.Exists(ctx.GoContext(), conn, remoteBinaryPath)
	configExists, _ := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	return binaryExists && configExists, nil
}

func (s *DistributeRegistryArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localBinaryPath, err := s.getLocalBinaryPath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(localBinaryPath); os.IsNotExist(err) {
		return fmt.Errorf("local binary '%s' not found", localBinaryPath)
	}
	if _, err := os.Stat(s.LocalConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("local config '%s' not found", s.LocalConfigPath)
	}

	remoteBinaryPath := filepath.Join(common.DefaultBinDir, "registry")
	remoteConfigPath := "/etc/docker/registry/config.yml"
	if err := runner.Mkdirp(ctx.GoContext(), conn, common.DefaultBinDir, "0755", s.Sudo); err != nil {
		return err
	}
	if err := runner.Mkdirp(ctx.GoContext(), conn, "/etc/docker/registry", "0755", s.Sudo); err != nil {
		return err
	}

	logger.Infof("Uploading registry binary to %s", remoteBinaryPath)
	if err := runner.Upload(ctx.GoContext(), conn, localBinaryPath, remoteBinaryPath, s.Sudo); err != nil {
		return err
	}
	if err := runner.Chmod(ctx.GoContext(), conn, remoteBinaryPath, "0755", s.Sudo); err != nil {
		return err
	}

	logger.Infof("Uploading registry config to %s", remoteConfigPath)
	if err := runner.Upload(ctx.GoContext(), conn, s.LocalConfigPath, remoteConfigPath, s.Sudo); err != nil {
		return err
	}
	if err := runner.Chmod(ctx.GoContext(), conn, remoteConfigPath, "0644", s.Sudo); err != nil {
		return err
	}

	logger.Info("Successfully distributed registry artifacts.")
	return nil
}

func (s *DistributeRegistryArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}
	_ = runner.Remove(ctx.GoContext(), conn, filepath.Join(common.DefaultBinDir, "registry"), s.Sudo, false)
	_ = runner.Remove(ctx.GoContext(), conn, "/etc/docker/registry", s.Sudo, true)
	logger.Info("Rollback for registry artifacts distribution completed.")
	return nil
}

var _ step.Step = (*DistributeRegistryArtifactsStep)(nil)
