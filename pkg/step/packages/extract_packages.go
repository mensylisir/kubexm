package packages

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ExtractPackagesStep struct {
	step.Base
	LocalPackagesPath string
}
type ExtractPackagesStepBuilder struct {
	step.Builder[ExtractPackagesStepBuilder, *ExtractPackagesStep]
}

func NewExtractPackagesStepBuilder(ctx runtime.Context, instanceName string) *ExtractPackagesStepBuilder {
	s := &ExtractPackagesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute and install offline packages", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute
	s.LocalPackagesPath = filepath.Join(ctx.GetUploadDir(), "packages.tar.gz")
	b := new(ExtractPackagesStepBuilder).Init(s)
	return b
}

func (s *ExtractPackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractPackagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, strings.TrimSuffix(s.LocalPackagesPath, ".tar.gz"))
	if err != nil {
		return false, fmt.Errorf("failed to check remote directory: %w", err)
	}
	if exists {
		logger.Info("Offline package seems to be already exists. Skipping offline extract.")
		return true, nil
	}
	return false, nil
}

func (s *ExtractPackagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.LocalPackagesPath)
	if err != nil {
		return fmt.Errorf("failed to check for remote tarball '%s': %w", s.LocalPackagesPath, err)
	}
	if !exists {
		return fmt.Errorf("remote tarball '%s' not found, 'DistributePackagesStep' may have failed", s.LocalPackagesPath)
	}

	extractDir := strings.TrimSuffix(s.LocalPackagesPath, ".tar.gz")
	logger.Infof("Creating extraction directory: %s", extractDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, extractDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote extraction directory '%s': %w", extractDir, err)
	}

	logger.Info("Extracting offline packages on host...")
	extractCmd := fmt.Sprintf("tar -zxf %s -C %s", s.LocalPackagesPath, extractDir)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, extractCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract offline package '%s': %w", s.LocalPackagesPath, err)
	}

	logger.Info("All required packages have been extracted successfully.")
	return nil
}

func (s *ExtractPackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing temporary directory: %s", strings.TrimSuffix(s.LocalPackagesPath, ".tar.gz"))
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", strings.TrimSuffix(s.LocalPackagesPath, ".tar.gz")), s.Sudo); err != nil {
		logger.Errorf("Failed to remove temporary directory '%s' during rollback: %v", strings.TrimSuffix(s.LocalPackagesPath, ".tar.gz"), err)
	}

	return nil
}

var _ step.Step = (*ExtractPackagesStep)(nil)
