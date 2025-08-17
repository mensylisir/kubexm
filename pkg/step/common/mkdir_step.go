package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"os"
	"time"

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type MkdirStep struct {
	step.Base
	Path        string
	Permissions os.FileMode
}

type MkdirStepBuilder struct {
	step.Builder[MkdirStepBuilder, *MkdirStep]
}

func NewMkdirStepBuilder(ctx runtime.ExecutionContext, instanceName, path string) *MkdirStepBuilder {
	cs := &MkdirStep{
		Path:        path,
		Permissions: 0755,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Mkdir [%s]", instanceName, path)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(MkdirStepBuilder).Init(cs)
}

func (b *MkdirStepBuilder) WithPermissions(permissions os.FileMode) *MkdirStepBuilder {
	b.Step.Permissions = permissions
	return b
}

func (s *MkdirStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *MkdirStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	fi, err := runner.Stat(ctx.GoContext(), conn, s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Directory does not exist. Step needs to run.", "path", s.Path)
			return false, nil
		}
		logger.Warn("Error checking remote path. Step will attempt to run.", "path", s.Path, "error", err)
		return false, nil
	}

	if !fi.IsDir() {
		logger.Warn("Path exists but is not a directory. Step will attempt to run.", "path", s.Path)
		return false, nil
	}

	if fi.Mode().Perm() != s.Permissions {
		logger.Info("Directory exists, but permissions are incorrect. Step needs to run.",
			"path", s.Path, "current", fi.Mode().Perm(), "target", s.Permissions)
		return false, nil
	}

	logger.Info("Directory already exists with correct permissions.", "path", s.Path)
	return true, nil
}

func (s *MkdirStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	permStr := fmt.Sprintf("%o", s.Permissions)
	logger.Info("Creating/ensuring directory.", "path", s.Path, "permissions", permStr, "sudo", s.Sudo)

	err = runner.Mkdirp(ctx.GoContext(), conn, s.Path, permStr, s.Sudo)
	if err != nil {
		logger.Error(err, "Failed to create directory.", "path", s.Path)
		return fmt.Errorf("failed to create directory %s: %w", s.Path, err)
	}

	// Also ensure the permissions are set, as Mkdirp might not update them if the dir exists.
	err = runner.Chmod(ctx.GoContext(), conn, s.Path, permStr, s.Sudo)
	if err != nil {
		logger.Error(err, "Failed to set permissions on directory.", "path", s.Path, "permissions", permStr)
		return fmt.Errorf("failed to set permissions on directory %s: %w", s.Path, err)
	}

	logger.Info("Directory created/ensured successfully.", "path", s.Path)
	return nil
}

func (s *MkdirStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Attempting to remove directory for rollback.", "path", s.Path)
	err = runner.Remove(ctx.GoContext(), conn, s.Path, s.Sudo, true)
	if err != nil {
		logger.Error(err, "Failed to remove directory during rollback.", "path", s.Path)
		return fmt.Errorf("rollback: failed to remove directory %s: %w", s.Path, err)
	}
	logger.Info("Directory removed or was not present for rollback.", "path", s.Path)
	return nil
}

var _ step.Step = (*MkdirStep)(nil)
