package os

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type RemoveSysctlStep struct {
	step.Base
	removedFileContent []byte
}

type RemoveSysctlStepBuilder struct {
	step.Builder[RemoveSysctlStepBuilder, *RemoveSysctlStep]
}

func NewRemoveSysctlStepBuilder(ctx runtime.Context, instanceName string) *RemoveSysctlStepBuilder {
	s := &RemoveSysctlStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Remove Kubernetes sysctl configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveSysctlStepBuilder).Init(s)
	return b
}

func (s *RemoveSysctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveSysctlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	filePath := common.KubernetesSysctlConfFileTarget
	fileExists, err := runner.Exists(ctx.GoContext(), conn, filePath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check for existence of '%s'", filePath)
	}

	if !fileExists {
		logger.Infof("Sysctl config file '%s' already removed. Nothing to do.", filePath)
		return true, nil
	}

	logger.Infof("Sysctl config file '%s' found, needs to be removed.", filePath)
	return false, nil
}

func (s *RemoveSysctlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	filePath := common.KubernetesSysctlConfFileTarget

	logger.Infof("Saving content of '%s' for potential rollback...", filePath)
	contentBytes, err := runner.ReadFile(ctx.GoContext(), conn, filePath)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Warnf("Sysctl config file '%s' not found during run. Assuming already removed.", filePath)
			s.removedFileContent = nil
			return nil
		}
		return errors.Wrapf(err, "failed to read file '%s' before removing", filePath)
	}
	s.removedFileContent = contentBytes

	logger.Infof("Removing sysctl config file '%s'...", filePath)
	if err := runner.Remove(ctx.GoContext(), conn, filePath, s.Sudo, false); err != nil {
		return errors.Wrapf(err, "failed to remove sysctl config file '%s'", filePath)
	}

	logger.Info("Re-applying system default sysctl settings to unload removed configuration...")
	if _, err := runner.Run(ctx.GoContext(), conn, "sysctl --system", s.Sudo); err != nil {
		logger.Warnf("Failed to re-apply sysctl settings after removal. A reboot may be needed to fully revert. Error: %v", err)
	}

	logger.Info("Kubernetes sysctl configuration removed successfully.")
	return nil
}

func (s *RemoveSysctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.removedFileContent == nil {
		logger.Info("Nothing to roll back as no sysctl config was removed in the run step.")
		return nil
	}

	filePath := common.KubernetesSysctlConfFileTarget
	logger.Infof("Rolling back by restoring sysctl config file '%s'...", filePath)

	permissions := fmt.Sprintf("0%o", common.DefaultConfigFilePermission)
	err = runner.WriteFile(ctx.GoContext(), conn, s.removedFileContent, filePath, permissions, s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to restore sysctl config file '%s' during rollback", filePath)
	}

	logger.Info("Re-applying restored sysctl settings...")
	if _, err := runner.Run(ctx.GoContext(), conn, "sysctl --system", s.Sudo); err != nil {
		logger.Warnf("Failed to re-apply sysctl settings during rollback. Error: %v", err)
	}

	logger.Infof("Sysctl configuration rolled back successfully.")
	return nil
}
