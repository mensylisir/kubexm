package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RemoveCrictlStep struct {
	step.Base
	InstallPath string
}

type RemoveCrictlStepBuilder struct {
	step.Builder[RemoveCrictlStepBuilder, *RemoveCrictlStep]
}

func NewRemoveCrictlStepBuilder(ctx runtime.Context, instanceName string) *RemoveCrictlStepBuilder {
	s := &RemoveCrictlStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove crictl binary and related configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveCrictlStepBuilder).Init(s)
	return b
}

func (b *RemoveCrictlStepBuilder) WithInstallPath(installPath string) *RemoveCrictlStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (s *RemoveCrictlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveCrictlStep) filesToRemove() []string {
	return []string{
		filepath.Join(s.InstallPath, "crictl"),
		common.CrictlDefaultConfigFile,
	}
}

func (s *RemoveCrictlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	paths := s.filesToRemove()
	for _, path := range paths {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, fmt.Errorf("failed to check for path '%s': %w", path, err)
		}
		if exists {
			logger.Infof("Path '%s' still exists. Cleanup is required.", path)
			return false, nil
		}
	}

	logger.Info("All crictl related files have been removed. Step is done.")
	return true, nil
}

func (s *RemoveCrictlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	var removalErrors []error
	paths := s.filesToRemove()

	logger.Info("Removing crictl files...", "files", paths)

	for _, path := range paths {
		logger.Infof("Removing path: %s", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file or directory") {
				err := fmt.Errorf("failed to remove '%s': %w", path, err)
				logger.Error(err.Error())
				removalErrors = append(removalErrors, err)
			}
		}
	}

	if len(removalErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) during crictl cleanup, manual review may be required", len(removalErrors))
	}

	logger.Info("crictl and related configuration removed successfully.")
	return nil
}

func (s *RemoveCrictlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step (RemoveCrictlStep) has no rollback action.")
	return nil
}
