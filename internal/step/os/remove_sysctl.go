package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/pkg/errors"
)

var _ step.Step = (*RemoveSysctlStep)(nil)

type RemoveSysctlStep struct {
	step.Base
	removedFileContent []byte
}

type RemoveSysctlStepBuilder struct {
	step.Builder[RemoveSysctlStepBuilder, *RemoveSysctlStep]
}

func NewRemoveSysctlStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveSysctlStepBuilder {
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

func (s *RemoveSysctlStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "step failed"); return result, err
	}

	filePath := common.KubernetesSysctlConfFileTarget

	logger.Infof("Saving content of '%s' for potential rollback...", filePath)
	contentBytes, err := runner.ReadFile(ctx.GoContext(), conn, filePath)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Warnf("Sysctl config file '%s' not found during run. Assuming already removed.", filePath)
			s.removedFileContent = nil
	result.MarkCompleted("step completed successfully"); return result, nil
		}
		result.MarkFailed(err, "failed to read file '%s' before removing"); return result, err
	}
	s.removedFileContent = contentBytes

	logger.Infof("Removing sysctl config file '%s'...", filePath)
	if err := runner.Remove(ctx.GoContext(), conn, filePath, s.Sudo, false); err != nil {
		result.MarkFailed(err, "failed to remove sysctl config file '%s'"); return result, err
	}

	logger.Info("Re-applying system default sysctl settings to unload removed configuration...")
	if _, err := runner.Run(ctx.GoContext(), conn, "sysctl --system", s.Sudo); err != nil {
		logger.Warnf("Failed to re-apply sysctl settings after removal. A reboot may be needed to fully revert. Error: %v", err)
	}

	logger.Info("Kubernetes sysctl configuration removed successfully.")
	result.MarkCompleted("step completed successfully"); return result, nil
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
	err = helpers.WriteContentToRemote(ctx, conn, string(s.removedFileContent), filePath, permissions, s.Sudo)
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
