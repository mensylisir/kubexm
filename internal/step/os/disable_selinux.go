package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/pkg/errors"
)

var _ step.Step = (*DisableSelinuxStep)(nil)

type DisableSelinuxStep struct {
	step.Base
	originalSelinuxConfigContent string
	originalEnforceStatus        string
}

type DisableSelinuxStepBuilder struct {
	step.Builder[DisableSelinuxStepBuilder, *DisableSelinuxStep]
}

func NewDisableSelinuxStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableSelinuxStepBuilder {
	s := &DisableSelinuxStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Disable SELinux", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DisableSelinuxStepBuilder).Init(s)
	return b
}

func (s *DisableSelinuxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableSelinuxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	if !*ctx.GetClusterConfig().Spec.Preflight.DisableSelinux {
		return true, nil
	}
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	getenforceCmd := "getenforce"
	runResult, err := runner.Run(ctx.GoContext(), conn, getenforceCmd, s.Sudo)
	if err != nil {
		if strings.Contains(err.Error(), "command not found") {
			logger.Info("SELinux command 'getenforce' not found. Assuming SELinux is not installed/active.")
			return true, nil
		}
		return false, errors.Wrap(err, "failed to check SELinux status with 'getenforce'")
	}

	selinuxStatus := strings.TrimSpace(strings.ToLower(runResult.Stdout))
	if selinuxStatus == "disabled" || selinuxStatus == "permissive" {
		logger.Infof("SELinux is already in '%s' mode.", selinuxStatus)
		return true, nil
	}

	logger.Infof("SELinux is in '%s' mode and needs to be disabled.", selinuxStatus)
	return false, nil
}

func (s *DisableSelinuxStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "step failed"); return result, err
	}

	logger.Info("Saving current SELinux state for potential rollback...")
	enforceStatusCmd := "getenforce"
	enforceResult, _ := runner.Run(ctx.GoContext(), conn, enforceStatusCmd, s.Sudo)
	s.originalEnforceStatus = strings.TrimSpace(enforceResult.Stdout)

	configPath := "/etc/selinux/config"
	configBytes, err := runner.ReadFile(ctx.GoContext(), conn, configPath)
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "No such file or directory") {
		result.MarkFailed(err, "failed to read SELinux config at '%s'"); return result, err
	}
	s.originalSelinuxConfigContent = string(configBytes)

	logger.Info("Setting SELinux to permissive mode with 'setenforce 0'...")
	if _, err := runner.Run(ctx.GoContext(), conn, "setenforce 0", s.Sudo); err != nil {
		logger.Warnf("Command 'setenforce 0' failed, which may be expected. Error: %v", err)
	}

	logger.Info("Disabling SELinux permanently in /etc/selinux/config...")
	if s.originalSelinuxConfigContent == "" {
		logger.Warnf("SELinux config file '%s' not found. Skipping permanent disablement.", configPath)
	result.MarkCompleted("step completed successfully"); return result, nil
	}

	sedCmd := "sed -i 's/SELINUX=enforcing/SELINUX=disabled/g; s/SELINUX=permissive/SELINUX=disabled/g' /etc/selinux/config"
	if _, err := runner.Run(ctx.GoContext(), conn, sedCmd, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to update /etc/selinux/config"); return result, err
	}

	logger.Info("/etc/selinux/config updated.")
	logger.Warn("A reboot is required for the permanent SELinux disablement to take full effect.")
	logger.Info("SELinux disable step completed.")
	result.MarkCompleted("step completed successfully"); return result, nil
}

func (s *DisableSelinuxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Attempting to roll back SELinux changes...")

	if s.originalSelinuxConfigContent != "" {
		logger.Info("Restoring original /etc/selinux/config...")
		configPath := "/etc/selinux/config"
		err := helpers.WriteContentToRemote(ctx, conn, s.originalSelinuxConfigContent, configPath, "0644", s.Sudo)
		if err != nil {
			return errors.Wrapf(err, "failed to restore '%s'", configPath)
		}
		logger.Info("'%s' restored.", configPath)
	}

	if strings.ToLower(s.originalEnforceStatus) == "enforcing" {
		logger.Info("Restoring SELinux to enforcing mode with 'setenforce 1'...")
		if _, err := runner.Run(ctx.GoContext(), conn, "setenforce 1", s.Sudo); err != nil {
			logger.Warnf("Failed to set SELinux back to enforcing mode. Error: %v", err)
		}
	}

	logger.Info("SELinux state has been rolled back.")
	return nil
}
