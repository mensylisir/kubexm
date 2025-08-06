package packages

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DistributePackagesStep struct {
	step.Base
	LocalPackagesDir  string
	localFile         string
	remotePackagePath string
}
type DistributePackagesStepBuilder struct {
	step.Builder[DistributePackagesStepBuilder, *DistributePackagesStep]
	localPackagesDir string
}

func NewDistributePackagesStepBuilder(ctx runtime.Context, instanceName string) *DistributePackagesStepBuilder {
	s := &DistributePackagesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute and install offline packages", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute
	s.remotePackagePath = filepath.Join(ctx.GetUploadDir(), "packages.tar.gz")
	b := new(DistributePackagesStepBuilder).Init(s)
	return b
}

func (b *DistributePackagesStepBuilder) WithLocalPackagesDir(dir string) *DistributePackagesStepBuilder {
	b.Step.LocalPackagesDir = dir
	return b
}

func (s *DistributePackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributePackagesStep) getPathsForPackages(ctx runtime.ExecutionContext) (string, error) {
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return "", fmt.Errorf("failed to gather facts to determine offline package: %w", err)
	}
	localPackageDir := filepath.Join(ctx.GetRepositoryDir(), facts.OS.ID, facts.OS.VersionID, facts.OS.Arch)
	localTarballName := fmt.Sprintf("packages-%s-%s-%s.tar.gz", facts.OS.ID, facts.OS.VersionID, facts.OS.Arch)
	localPackagePath := filepath.Join(localPackageDir, localTarballName)
	s.LocalPackagesDir = localPackageDir
	s.localFile = localTarballName
	return localPackagePath, nil
}

func (s *DistributePackagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.remotePackagePath)
	if err != nil {
		return false, fmt.Errorf("failed to check remote directory: %w", err)
	}
	if exists {
		logger.Info("Offline package seems to be already exists. Skipping offline upload.")
		return true, nil
	}
	return false, nil
}

func (s *DistributePackagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localPackagePath, err := s.getPathsForPackages(ctx)
	if err != nil {
		logger.Errorf("Failed to get paths for packages: %w", err)
		return err
	}

	logger.Infof("Uploading offline package '%s' to host...", filepath.Base(localPackagePath))

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.remotePackagePath)
	if err != nil {
		return fmt.Errorf("failed to check remote directory: %w", err)
	}
	if !exists {
		err := runnerSvc.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.remotePackagePath), "0777", s.Sudo)
		if err != nil {
			return fmt.Errorf("failed to create remote temp directory: %w", err)
		}
	}

	if err := runnerSvc.Upload(ctx.GoContext(), conn, localPackagePath, s.remotePackagePath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload offline package: %w", err)
	}
	logger.Info("All required packages sources have been setting successfully.")
	return nil
}

func (s *DistributePackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing temporary directory: %s", s.remotePackagePath)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", s.remotePackagePath), s.Sudo); err != nil {
		logger.Errorf("Failed to remove temporary directory '%s' during rollback: %v", s.remotePackagePath, err)
	}

	return nil
}

var _ step.Step = (*DistributePackagesStep)(nil)
