package preflight

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

var _ step.Step = (*CheckCPUStep)(nil)

type CheckCPUStep struct {
	step.Base
	MinCores *int32
}

type CheckCPUStepBuilder struct {
	step.Builder[CheckCPUStepBuilder, *CheckCPUStep]
}

func NewCheckCPUStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CheckCPUStepBuilder {
	s := &CheckCPUStep{
		MinCores: ctx.GetClusterConfig().Spec.Preflight.MinCPUCores,
	}

	var minCores int32 = 0
	if s.MinCores != nil {
		minCores = *s.MinCores
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Check for minimum of %d CPU cores", s.Base.Meta.Name, minCores)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckCPUStepBuilder).Init(s)
	return b
}

func (s *CheckCPUStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckCPUStep) checkRequirement(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	skipChecks := ctx.GetClusterConfig().Spec.Preflight.SkipChecks
	const checkName = "cpu_cores"
	for _, checkToSkip := range skipChecks {
		if checkToSkip == checkName {
			logger.Infof("Skipping CPU check because '%s' is in skipChecks list.", checkName)
			return nil
		}
	}

	if s.MinCores == nil {
		logger.Info("Minimum CPU core requirement not configured, skipping check.")
		return nil
	}
	minCores := *s.MinCores

	actualCores, err := s.getActualCores(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine the actual number of CPU cores")
	}

	logger.Infof("Checking CPU cores: Required >= %d, Actual = %d.", minCores, actualCores)

	if int32(actualCores) < minCores {
		return errors.Errorf("CPU core requirement not met: Required >= %d, but found only %d cores", minCores, actualCores)
	}

	logger.Info("CPU core requirement is met.")
	return nil
}

func (s *CheckCPUStep) getActualCores(ctx runtime.ExecutionContext) (int, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get host connector")
	}

	facts, err := ctx.GetRunner().GatherHostFacts(ctx.GoContext(), conn)
	if err == nil && facts != nil && facts.CPU != nil && facts.CPU.LogicalCount > 0 {
		logger.Debugf("Determined CPU core count from host facts: %d", facts.CPU.LogicalCount)
		return facts.CPU.LogicalCount, nil
	}
	if err != nil {
		logger.Warnf("Could not gather host facts to determine CPU count, will fall back to command. Error: %v", err)
	}

	runner := ctx.GetRunner()

	logger.Info("Attempting to determine CPU count via 'nproc' command.")
	runResult, err := runner.Run(ctx.GoContext(), conn, "nproc", s.Sudo)
	if err == nil {
		coresStr := strings.TrimSpace(runResult.Stdout)
		cores, parseErr := strconv.Atoi(coresStr)
		if parseErr == nil {
			logger.Debugf("Determined CPU core count from command 'nproc': %d", cores)
			return cores, nil
		}
		logger.Warnf("Failed to parse 'nproc' output, will try fallback. Output: '%s', Error: %v", coresStr, parseErr)
	} else {
		logger.Warnf("Failed to execute 'nproc', will try fallback. Error: %v", err)
	}

	logger.Info("Fallback: Attempting to determine CPU count via '/proc/cpuinfo'.")
	cmd := "grep -c ^processor /proc/cpuinfo"
	runResult, err = runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute 'grep' on /proc/cpuinfo to get CPU count")
	}

	coresStr := strings.TrimSpace(runResult.Stdout)
	cores, err := strconv.Atoi(coresStr)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse CPU count from 'grep' output: %s", coresStr)
	}

	logger.Debugf("Determined CPU core count from command '%s': %d", cmd, cores)
	return cores, nil
}

func (s *CheckCPUStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	checkErr := s.checkRequirement(ctx)
	if checkErr == nil {
		return true, nil
	}
	return false, nil
}

func (s *CheckCPUStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	if err := s.checkRequirement(ctx); err != nil {
		result.MarkFailed(err, "CPU check failed")
		return result, err
	}
	result.MarkCompleted("CPU check passed")
	return result, nil
}

func (s *CheckCPUStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}
