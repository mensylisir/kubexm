package docker

import (
	"errors" // 需要引入 errors
	"fmt"
	"os" // 需要引入 os
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RemoveDockerStep struct {
	step.Base
	FilesToRemove []string
	InstallPath   string
	Purge         bool
}

type RemoveDockerStepBuilder struct {
	step.Builder[RemoveDockerStepBuilder, *RemoveDockerStep]
}

func NewRemoveDockerStepBuilder(ctx runtime.Context, instanceName string) *RemoveDockerStepBuilder {
	s := &RemoveDockerStep{
		FilesToRemove: []string{
			"containerd",
			"containerd-shim-runc-v2",
			"ctr",
			"docker",
			"docker-init",
			"docker-proxy",
			"dockerd",
			"runc",
		},
		InstallPath: common.DefaultBinDir,
		Purge:       false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Docker components and configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(RemoveDockerStepBuilder).Init(s)
	return b
}

func (b *RemoveDockerStepBuilder) WithFiles(files []string) *RemoveDockerStepBuilder {
	b.Step.FilesToRemove = files
	return b
}

func (b *RemoveDockerStepBuilder) WithInstallPath(path string) *RemoveDockerStepBuilder {
	b.Step.InstallPath = path
	return b
}

func (b *RemoveDockerStepBuilder) WithPurge(purge bool) *RemoveDockerStepBuilder {
	b.Step.Purge = purge
	if purge {
		b.Step.Base.Meta.Description = fmt.Sprintf("[%s]>>Purge (remove with all data) Docker components", b.Step.Base.Meta.Name)
	}
	return b
}

func (s *RemoveDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveDockerStep) getAllPathsToRemove() []string {
	paths := make([]string, 0, len(s.FilesToRemove)+5)
	for _, file := range s.FilesToRemove {
		paths = append(paths, filepath.Join(s.InstallPath, file))
	}

	paths = append(paths,
		common.DockerDefaultSystemdFile,
		common.DockerDefaultDropInFile,
		filepath.Dir(common.DockerDefaultDropInFile),
		common.DockerDefaultConfDirTarget,
	)

	if s.Purge {
		paths = append(paths, common.DockerDefaultDataRoot)
	}

	return paths
}

func (s *RemoveDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err == nil {
		isActive, _ := runner.IsServiceActive(ctx.GoContext(), conn, facts, common.DefaultDockerServiceName)
		if isActive {
			logger.Info("Docker service is still active. Cleanup is required.")
			return false, nil
		}
	}

	paths := s.getAllPathsToRemove()
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

	logger.Info("All Docker related components have been removed. Step is done.")
	return true, nil
}

func (s *RemoveDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Could not gather host facts, will proceed with cleanup but might not be able to interact with services gracefully: %v", err)
	}

	if facts != nil {
		logger.Info("Stopping and disabling docker service...")
		_ = runner.StopService(ctx.GoContext(), conn, facts, common.DefaultDockerServiceName)
		_ = runner.DisableService(ctx.GoContext(), conn, facts, common.DefaultDockerServiceName)
	}

	var removalErrors []error
	paths := s.getAllPathsToRemove()
	purgeMsg := ""
	if s.Purge {
		purgeMsg = " (including data directory)"
	}
	logger.Info("Removing Docker files and directories"+purgeMsg, "paths", paths)

	for _, path := range paths {
		if path == common.DockerDefaultDataRoot {
			logger.Warnf("PURGE MODE: Removing Docker data root: %s", path)
		} else {
			logger.Infof("Removing path: %s", path)
		}

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
	}

	if len(removalErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) during Docker cleanup", len(removalErrors))
	}

	logger.Info("Docker components removed successfully.")
	return nil
}

func (s *RemoveDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveDockerStep is not applicable (would mean reinstalling).")
	return nil
}
