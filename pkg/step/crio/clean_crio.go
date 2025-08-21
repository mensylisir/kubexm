package crio

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

type CleanCrioStep struct {
	step.Base
	PurgeData bool
}

type CleanCrioStepBuilder struct {
	step.Builder[CleanCrioStepBuilder, *CleanCrioStep]
}

func NewCleanCrioStepBuilder(ctx runtime.Context, instanceName string) *CleanCrioStepBuilder {
	s := &CleanCrioStep{
		PurgeData: false,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean all CRI-O related files and configurations", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanCrioStepBuilder).Init(s)
	return b
}

func (b *CleanCrioStepBuilder) WithPurgeData(purge bool) *CleanCrioStepBuilder {
	b.Step.PurgeData = purge
	return b
}

func (s *CleanCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanCrioStep) filesAndDirsToRemove() []string {
	paths := []string{
		filepath.Join(common.DefaultBinPath, "crio"),
		filepath.Join(common.DefaultBinPath, "pinns"),
		filepath.Join(common.DefaultBinPath, "crictl"),
		filepath.Join(common.CRIORuntimePath, "conmon"),
		filepath.Join(common.CRIORuntimePath, "conmonrs"),
		filepath.Join(common.CRIORuntimePath, "runc"),
		filepath.Join(common.CRIORuntimePath, "crun"),
		common.CRIODefaultSystemdFile,
		common.DefaultCNIConfDirTarget,
		common.DefaultCNIBin,
		common.CrictlDefaultConfigFile,
		common.CRIODefaultAuthFile,
		common.RegistriesDefaultConfigFile,
		common.CRIODefaultConfDir,
		"/etc/default/crio",
		"/etc/sysconfig/crio",
	}

	if s.PurgeData {
		paths = append(paths, common.CRIODefaultGraphRoot)
	}

	return paths
}

func (s *CleanCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	logger.Info("All cri-o related files and directories have been removed. Step is done.")
	return true, nil
}

func (s *CleanCrioStep) Run(ctx runtime.ExecutionContext) error {
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

func (s *CleanCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanCrioStep)(nil)
