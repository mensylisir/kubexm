package containerd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanupContainerdStep struct {
	step.Base
	InstallPath     string
	RemoteCNIBinDir string
	PurgeData       bool
}

type CleanupContainerdStepBuilder struct {
	step.Builder[CleanupContainerdStepBuilder, *CleanupContainerdStep]
}

func NewCleanupContainerdStepBuilder(ctx runtime.Context, instanceName string) *CleanupContainerdStepBuilder {
	s := &CleanupContainerdStep{
		InstallPath:     common.DefaultBinDir,
		RemoteCNIBinDir: common.DefaultCNIBin,
		PurgeData:       false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup containerd and related components", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanupContainerdStepBuilder).Init(s)
	return b
}

func (b *CleanupContainerdStepBuilder) WithInstallPath(installPath string) *CleanupContainerdStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *CleanupContainerdStepBuilder) WithRemoteCNIBinDir(remoteCNIBinDir string) *CleanupContainerdStepBuilder {
	b.Step.RemoteCNIBinDir = remoteCNIBinDir
	return b
}

func (b *CleanupContainerdStepBuilder) WithPurgeData(purge bool) *CleanupContainerdStepBuilder {
	b.Step.PurgeData = purge
	return b
}

func (s *CleanupContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanupContainerdStep) filesAndDirsToRemove() []string {
	paths := []string{
		filepath.Join(s.InstallPath, "containerd"),
		filepath.Join(s.InstallPath, "containerd-shim"),
		filepath.Join(s.InstallPath, "containerd-shim-runc-v1"),
		filepath.Join(s.InstallPath, "containerd-shim-runc-v2"),
		filepath.Join(s.InstallPath, "ctr"),
		filepath.Join(common.DefaultSBinDir, "runc"),
		filepath.Join(s.InstallPath, "crictl"),
		common.DefaultCNIConfDirTarget,
		common.DefaultCNIBin,
		DefaultContainerdServicePath,
		common.ContainerdDefaultDropInFile,
		filepath.Dir(common.ContainerdDefaultDropInFile),
		filepath.Dir(common.ContainerdDefaultConfigFile),
		common.CrictlDefaultConfigFile,
	}

	if s.PurgeData {
		paths = append(paths, common.ContainerdDefaultRoot)
	}

	return paths
}

func (s *CleanupContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	paths := s.filesAndDirsToRemove()
	for _, path := range paths {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, err
		}
		if exists {
			logger.Info("Path still exists. Cleanup is required.", "path", path)
			return false, nil
		}
	}

	logger.Info("All containerd related files and directories have been removed. Step is done.")
	return true, nil
}

func (s *CleanupContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	paths := s.filesAndDirsToRemove()

	for _, path := range paths {
		logger.Warn("Removing path.", "path", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Error(err, "Failed to remove path, manual cleanup may be required.", "path", path)
			}
		}
	}

	logger.Info("Reloading systemd daemon after cleanup")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			logger.Error(err, "Failed to reload systemd daemon during cleanup.")
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			logger.Error(err, "Failed to reload systemd daemon during cleanup.")
		}
	}

	return nil
}

func (s *CleanupContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanupContainerdStep)(nil)
