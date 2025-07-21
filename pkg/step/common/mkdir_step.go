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
		return false, fmt.Errorf("precheck MkdirStep: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	fi, err := runner.Stat(ctx.GoContext(), conn, s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Infof("Directory %s does not exist on host %s. Step needs to run.", s.Path, ctx.GetHost().GetName())
			return false, nil
		}
		logger.Warnf("Error checking remote path %s with runner: %v. Step will attempt to run.", s.Path, err)
		return false, nil
	}

	if !fi.IsDir() {
		logger.Warnf("Path %s exists on host %s but is not a directory. Step will attempt to run.", s.Path, ctx.GetHost().GetName())
		return false, nil
	}

	if fi.Mode().Perm() != s.Permissions {
		logger.Infof("Directory %s exists on host %s, but permissions are incorrect (current: %o, target: %o). Step needs to run.", s.Path, ctx.GetHost().GetName(), fi.Mode().Perm(), s.Permissions)
		return false, nil
	}

	logger.Infof("Directory %s already exists on host %s with correct permissions.", s.Path, ctx.GetHost().GetName())
	return true, nil
}

func (s *MkdirStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run MkdirStep: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	permStr := fmt.Sprintf("%o", s.Permissions)
	logger.Infof("Creating directory %s on host %s with permissions %s (sudo: %t).", s.Path, ctx.GetHost().GetName(), permStr, s.Sudo)

	err = runner.Mkdirp(ctx.GoContext(), conn, s.Path, permStr, s.Sudo)
	if err != nil {
		logger.Errorf("Failed to create directory %s on host %s: %v", s.Path, ctx.GetHost().GetName(), err)
		return fmt.Errorf("failed to create directory %s on host %s: %w", s.Path, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Directory %s created/ensured on host %s.", s.Path, ctx.GetHost().GetName())
	return nil
}

func (s *MkdirStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback MkdirStep: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	err = runner.Remove(ctx.GoContext(), conn, s.Path, s.Sudo, true)
	if err != nil {
		logger.Errorf("Failed to remove directory %s on host %s during rollback: %v", s.Path, ctx.GetHost().GetName(), err)
		return fmt.Errorf("rollback: failed to remove directory %s on host %s: %w", s.Path, ctx.GetHost().GetName(), err)
	}
	logger.Infof("Directory %s removed or was not present on host %s for rollback.", s.Path, ctx.GetHost().GetName())
	return nil
}

var _ step.Step = (*MkdirStep)(nil)
