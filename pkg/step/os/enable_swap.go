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

type EnableSwapStep struct {
	step.Base
	originalFstabContent string
}

type EnableSwapStepBuilder struct {
	step.Builder[EnableSwapStepBuilder, *EnableSwapStep]
}

func NewEnableSwapStepBuilder(ctx runtime.ExecutionContext, instanceName string) *EnableSwapStepBuilder {
	s := &EnableSwapStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Enable swap", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableSwapStepBuilder).Init(s)
	return b
}

func (s *EnableSwapStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableSwapStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	swapStatus, _ := runner.Run(ctx.GoContext(), conn, "swapon --show", s.Sudo)

	if strings.TrimSpace(swapStatus) != "" {
		logger.Info("Swap is already enabled.")
		return true, nil
	}

	logger.Info("Swap is disabled and needs to be enabled.")
	return false, nil
}

func (s *EnableSwapStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	fstabPath := "/etc/fstab"
	logger.Info("Enabling swap permanently in /etc/fstab...")

	fstabBytes, err := runner.ReadFile(ctx.GoContext(), conn, fstabPath)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Warn("/etc/fstab not found, cannot permanently enable swap.")
			s.originalFstabContent = ""
		} else {
			return errors.Wrap(err, "failed to read /etc/fstab")
		}
	} else {
		s.originalFstabContent = string(fstabBytes)

		lines := strings.Split(s.originalFstabContent, "\n")
		var newLines []string
		changed := false
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "#") && strings.Contains(trimmedLine, "swap") {
				newLines = append(newLines, strings.TrimPrefix(line, "#"))
				changed = true
				logger.Infof("Uncommenting fstab line: '%s'", trimmedLine)
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
			logger.Info("/etc/fstab updated to enable swap on boot.")
		} else {
			logger.Info("/etc/fstab does not contain any commented out swap entries. No changes made.")
		}
	}

	logger.Info("Turning on swap with 'swapon -a'...")
	if _, err := runner.Run(ctx.GoContext(), conn, "swapon -a", s.Sudo); err != nil {
		logger.Warnf("Command 'swapon -a' did not succeed, which may be expected if no swap partitions are defined. Error: %v", err)
	}

	logger.Info("Swap enable step completed.")
	return nil
}

func (s *EnableSwapStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Rolling back swap enablement...")

	if s.originalFstabContent != "" {
		logger.Info("Restoring original /etc/fstab...")
		err = helpers.WriteContentToRemote(ctx, conn, s.originalFstabContent, "/etc/fstab", "0644", s.Sudo)
		if err != nil {
			return errors.Wrap(err, "failed to restore /etc/fstab during rollback")
		}
	}

	logger.Info("Turning off swap with 'swapoff -a'...")
	if _, err := runner.Run(ctx.GoContext(), conn, "swapoff -a", s.Sudo); err != nil {
		logger.Warnf("Command 'swapoff -a' failed during rollback. Error: %v", err)
	}

	logger.Info("Swap disabled as part of rollback.")
	return nil
}
