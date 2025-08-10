package preflight

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type CheckMemoryStep struct {
	step.Base
	MinMemoryMB *uint64
}

type CheckMemoryStepBuilder struct {
	step.Builder[CheckMemoryStepBuilder, *CheckMemoryStep]
}

func NewCheckMemoryStepBuilder(ctx runtime.Context, instanceName string) *CheckMemoryStepBuilder {
	s := &CheckMemoryStep{
		MinMemoryMB: ctx.GetClusterConfig().Spec.Preflight.MinMemoryMB,
	}
	var minMB uint64 = 0
	if s.MinMemoryMB != nil {
		minMB = *s.MinMemoryMB
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Check for minimum of %d MB memory", s.Base.Meta.Name, minMB)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckMemoryStepBuilder).Init(s)
	return b
}

func (s *CheckMemoryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckMemoryStep) checkRequirement(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	skipChecks := ctx.GetClusterConfig().Spec.Preflight.SkipChecks
	const checkName = "memory"
	for _, checkToSkip := range skipChecks {
		if checkToSkip == checkName {
			logger.Infof("Skipping memory check because '%s' is in skipChecks list.", checkName)
			return nil
		}
	}

	if s.MinMemoryMB == nil {
		logger.Info("Minimum memory requirement not configured, skipping check.")
		return nil
	}

	minMB := *s.MinMemoryMB

	actualMemoryMB, err := s.getActualMemoryMB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine the actual memory size")
	}

	logger.Infof("Checking memory size: Required >= %d MB, Actual = %d MB.", minMB, actualMemoryMB)

	if actualMemoryMB < minMB {
		return errors.Errorf("Memory requirement not met: Required >= %d MB, but found only %d MB", minMB, actualMemoryMB)
	}

	logger.Info("Memory requirement is met.")
	return nil
}

func (s *CheckMemoryStep) getActualMemoryMB(ctx runtime.ExecutionContext) (uint64, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get host connector")
	}
	facts, err := ctx.GetRunner().GatherHostFacts(ctx.GoContext(), conn)
	if err == nil && facts != nil && facts.Memory != nil && !facts.Memory.Total.IsZero() {
		memoryBytes := uint64(facts.Memory.Total.Value())
		memoryMB := memoryBytes / (1024 * 1024)
		logger.Debugf("Determined memory size from host facts: %d MB", memoryMB)
		return memoryMB, nil
	}
	if err != nil {
		logger.Warnf("Could not gather host facts to determine memory size, will fall back to command. Error: %v", err)
	}
	logger.Info("Host facts unavailable or incomplete, determining memory size via command.")
	runner := ctx.GetRunner()
	var cmd string
	var isKb, isBytes bool
	osID := "linux"
	if facts != nil && facts.OS != nil {
		osID = strings.ToLower(facts.OS.ID)
	}
	if osID == "darwin" {
		cmd = "sysctl -n hw.memsize"
		isBytes = true
	} else {
		cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
		isKb = true
	}
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to execute '%s' to get memory info", cmd)
	}
	memStr := strings.TrimSpace(output)
	memVal, err := strconv.ParseUint(memStr, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse memory size from command output: %s", memStr)
	}
	var calculatedMemoryMB uint64
	if isKb {
		calculatedMemoryMB = memVal / 1024
	} else if isBytes {
		calculatedMemoryMB = memVal / (1024 * 1024)
	} else {
		calculatedMemoryMB = memVal
	}
	logger.Debugf("Determined memory size from command '%s': %d MB", cmd, calculatedMemoryMB)
	return calculatedMemoryMB, nil
}

func (s *CheckMemoryStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	checkErr := s.checkRequirement(ctx)

	if checkErr == nil {
		return true, nil
	}

	return false, nil
}

func (s *CheckMemoryStep) Run(ctx runtime.ExecutionContext) error {
	return s.checkRequirement(ctx)
}

func (s *CheckMemoryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}
