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

type CleanDockerStep struct {
	step.Base
	InstallPath string
	PurgeData   bool
}

type CleanDockerStepBuilder struct {
	step.Builder[CleanDockerStepBuilder, *CleanDockerStep]
}

func NewCleanDockerStepBuilder(ctx runtime.Context, instanceName string) *CleanDockerStepBuilder {
	s := &CleanDockerStep{
		InstallPath: common.DefaultBinDir,
		PurgeData:   false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup docker, containerd and related components", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(CleanDockerStepBuilder).Init(s)
	return b
}

func (b *CleanDockerStepBuilder) WithInstallPath(installPath string) *CleanDockerStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *CleanDockerStepBuilder) WithPurgeData(purge bool) *CleanDockerStepBuilder {
	b.Step.PurgeData = purge
	if purge {
		b.Step.Base.Meta.Description = fmt.Sprintf("[%s]>>Purge (remove with all data) docker, containerd and related components", b.Step.Meta().Name)
	}
	return b
}

func (s *CleanDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanDockerStep) servicesToManage() []string {
	return []string{
		common.DefaultDockerServiceName,
		common.ContainerdDefaultServiceName,
	}
}

func (s *CleanDockerStep) filesAndDirsToRemove() []string {
	paths := []string{
		filepath.Join(s.InstallPath, "docker"),
		filepath.Join(s.InstallPath, "dockerd"),
		filepath.Join(s.InstallPath, "docker-proxy"),
		filepath.Join(s.InstallPath, "docker-init"),
		filepath.Join(s.InstallPath, "containerd"),
		filepath.Join(s.InstallPath, "containerd-shim"),
		filepath.Join(s.InstallPath, "containerd-shim-runc-v1"),
		filepath.Join(s.InstallPath, "containerd-shim-runc-v2"),
		filepath.Join(s.InstallPath, "ctr"),
		filepath.Join(common.DefaultSBinDir, "runc"),
		filepath.Join(s.InstallPath, "crictl"),

		common.DefaultCNIConfDirTarget,
		common.DefaultCNIBin,
		common.DockerDefaultSystemdFile,
		common.DockerDefaultDropInFile,
		filepath.Dir(common.DockerDefaultDropInFile),
		common.ContainerdDefaultSystemdFile,
		filepath.Dir(common.ContainerdDefaultDropInFile),
		common.DockerDefaultConfDirTarget,
		common.ContainerdDefaultConfDir,
		common.CrictlDefaultConfigFile,
	}

	if s.PurgeData {
		paths = append(paths,
			common.DockerDefaultDataRoot,
			common.ContainerdDefaultRoot,
		)
	}

	return paths
}

func (s *CleanDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)

	if err == nil {
		for _, service := range s.servicesToManage() {
			isActive, _ := runner.IsServiceActive(ctx.GoContext(), conn, facts, service)
			if isActive {
				logger.Infof("Service '%s' is still active. Cleanup is required.", service)
				return false, nil
			}
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

	logger.Info("All docker related files and directories have been removed. Step is done.")
	return true, nil
}

func (s *CleanDockerStep) Run(ctx runtime.ExecutionContext) error {
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
		for _, service := range s.servicesToManage() {
			logger.Infof("Stopping and disabling service: %s", service)
			_ = runner.StopService(ctx.GoContext(), conn, facts, service)
			_ = runner.DisableService(ctx.GoContext(), conn, facts, service)
		}
	}

	var removalErrors []error
	paths := s.filesAndDirsToRemove()
	logger.Info("Removing all docker and containerd related files...", "purge", s.PurgeData)

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

	logger.Info("Reloading systemd daemon after cleanup")
	if facts != nil {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			err = fmt.Errorf("failed to reload systemd daemon: %w", err)
			logger.Error(err.Error())
			removalErrors = append(removalErrors, err)
		}
	}

	if len(removalErrors) > 0 {
		return fmt.Errorf("encountered %d error(s) during Docker cleanup, manual review may be required", len(removalErrors))
	}

	logger.Info("Docker and containerd cleanup completed successfully.")
	return nil
}

func (s *CleanDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}
