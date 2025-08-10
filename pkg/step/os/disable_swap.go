package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type DisableSwapStep struct {
	step.Base
	originalFstabContent string
}

type DisableSwapStepBuilder struct {
	step.Builder[DisableSwapStepBuilder, *DisableSwapStep]
}

func NewDisableSwapStepBuilder(ctx runtime.Context, instanceName string) *DisableSwapStepBuilder {
	s := &DisableSwapStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Disable swap", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableSwapStepBuilder).Init(s)
	return b
}

func (s *DisableSwapStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableSwapStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	if !*ctx.GetClusterConfig().Spec.Preflight.DisableSwap {
		return true, nil
	}
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	swapStatus, _ := runner.Run(ctx.GoContext(), conn, "swapon --show", s.Sudo)

	if strings.TrimSpace(swapStatus) == "" {
		logger.Info("Swap is already disabled.")
		return true, nil
	}

	logger.Info("Swap is enabled and needs to be disabled.")
	return false, nil
}

func (s *DisableSwapStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Turning off swap with 'swapoff -a'...")
	if _, err := runner.Run(ctx.GoContext(), conn, "swapoff -a", s.Sudo); err != nil {
		return errors.Wrap(err, "failed to execute 'swapoff -a'")
	}

	fstabPath := "/etc/fstab"
	logger.Info("Disabling swap permanently in /etc/fstab...")

	fstabBytes, err := runner.ReadFile(ctx.GoContext(), conn, fstabPath)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Warn("/etc/fstab not found, skipping permanent disablement.")
			s.originalFstabContent = ""
			return nil
		}
		return errors.Wrap(err, "failed to read /etc/fstab")
	}
	s.originalFstabContent = string(fstabBytes)

	lines := strings.Split(s.originalFstabContent, "\n")
	var newLines []string
	changed := false
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, "swap") && !strings.HasPrefix(trimmedLine, "#") {
			newLines = append(newLines, "#"+line)
			changed = true
			logger.Infof("Commenting out fstab line: '%s'", trimmedLine)
		} else {
			newLines = append(newLines, line)
		}
	}

	if changed {
		newFstabContent := strings.Join(newLines, "\n")
		err = helpers.WriteContentToRemote(ctx, conn, newFstabContent, fstabPath, "0644", s.Sudo)
		if err != nil {
			return errors.Wrap(err, "failed to write updated /etc/fstab")
		}
		logger.Info("/etc/fstab updated to disable swap on boot.")
	} else {
		logger.Info("/etc/fstab already has swap entries commented out. No changes made.")
	}

	logger.Info("Swap disabled successfully.")
	return nil
}

func (s *DisableSwapStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.originalFstabContent == "" {
		logger.Warn("No original /etc/fstab content was saved, skipping fstab rollback.")
	} else {
		logger.Info("Restoring original /etc/fstab...")
		err = helpers.WriteContentToRemote(ctx, conn, s.originalFstabContent, "/etc/fstab", "0644", s.Sudo)
		if err != nil {
			return errors.Wrap(err, "failed to restore /etc/fstab")
		}
		logger.Info("/etc/fstab restored.")
	}

	logger.Info("Attempting to turn swap back on with 'swapon -a'...")
	if _, err := runner.Run(ctx.GoContext(), conn, "swapon -a", s.Sudo); err != nil {
		logger.Warnf("Command 'swapon -a' failed during rollback, this might be expected. Error: %v", err)
	} else {
		logger.Info("Swap has been re-enabled.")
	}

	return nil
}
