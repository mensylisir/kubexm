package docker

import (
	"errors" // Import the errors package
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

type RemoveCriDockerdStep struct {
	step.Base
	InstallPath string
}

type RemoveCriDockerdStepBuilder struct {
	step.Builder[RemoveCriDockerdStepBuilder, *RemoveCriDockerdStep]
}

func NewRemoveCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *RemoveCriDockerdStepBuilder {
	s := &RemoveCriDockerdStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove cri-dockerd and related components", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(RemoveCriDockerdStepBuilder).Init(s)
	return b
}

func (b *RemoveCriDockerdStepBuilder) WithInstallPath(installPath string) *RemoveCriDockerdStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (s *RemoveCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveCriDockerdStep) filesAndDirsToRemove() []string {
	return []string{
		filepath.Join(s.InstallPath, "cri-dockerd"),
		common.CniDockerdSystemdFile,
		common.CniDockerdSystemdDropinFile,
		filepath.Dir(common.CniDockerdSystemdDropinFile),
	}
}

func (s *RemoveCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err == nil {
		isActive, _ := runner.IsServiceActive(ctx.GoContext(), conn, facts, common.DefaultCRIDockerServiceName)
		if isActive {
			logger.Info("cri-dockerd service is still active. Cleanup is required.")
			return false, nil
		}
	}

	paths := s.filesAndDirsToRemove()
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

	logger.Info("All cri-dockerd related files and directories have been removed. Step is done.")
	return true, nil
}

func (s *RemoveCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Could not gather host facts, proceeding with cleanup but might not be able to interact with services gracefully: %v", err)
	}

	if facts != nil {
		logger.Info("Stopping and disabling cri-dockerd service...")
		_ = runner.StopService(ctx.GoContext(), conn, facts, common.DefaultCRIDockerServiceName)
		_ = runner.DisableService(ctx.GoContext(), conn, facts, common.DefaultCRIDockerServiceName)
	}

	var removalErrors []error
	paths := s.filesAndDirsToRemove()

	for _, path := range paths {
		logger.Warnf("Removing path: %s", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file or directory") {
				err := fmt.Errorf("failed to remove '%s': %w", path, err)
				logger.Error(err.Error())
				removalErrors = append(removalErrors, err)
			}
		}
	}

	logger.Info("Reloading systemd daemon after cleanup")
	if facts != nil {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			err = fmt.Errorf("failed to reload systemd daemon: %w", err)
			logger.Error(err.Error())
			removalErrors = append(removalErrors, err)
		}
	} else {
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			err = fmt.Errorf("failed to reload systemd daemon with fallback command: %w", err)
			logger.Error(err.Error())
			removalErrors = append(removalErrors, err)
		}
	}
	if len(removalErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) during cri-dockerd cleanup, manual review may be required", len(removalErrors))
	}

	return nil
}

func (s *RemoveCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}
