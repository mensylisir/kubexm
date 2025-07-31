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

type CheckCPUStep struct {
	step.Base
	MinCores *int32
}

type CheckCPUStepBuilder struct {
	step.Builder[CheckCPUStepBuilder, *CheckCPUStep]
}

func NewCheckCPUStepBuilder(ctx runtime.Context, instanceName string) *CheckCPUStepBuilder {
	s := &CheckCPUStep{
		MinCores: ctx.GetClusterConfig().Spec.Preflight.MinCPUCores,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Check for minimum of %d CPU cores", s.Base.Meta.Name, s.MinCores)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckCPUStepBuilder).Init(s)
	return b
}

func (s *CheckCPUStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
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

	logger.Info("Host facts unavailable or incomplete, determining CPU count via 'nproc' command.")
	runner := ctx.GetRunner()
	cmd := "nproc"
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute 'nproc' to get CPU count")
	}

	coresStr := strings.TrimSpace(output)
	cores, err := strconv.Atoi(coresStr)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse CPU count from 'nproc' output: %s", coresStr)
	}

	logger.Debugf("Determined CPU core count from command '%s': %d", cmd, cores)
	return cores, nil
}

func (s *CheckCPUStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	actualCores, err := s.getActualCores(ctx)
	if err != nil {
		return false, nil
	}

	return actualCores >= int(*s.MinCores), nil
}

func (s *CheckCPUStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	actualCores, err := s.getActualCores(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine the actual number of CPU cores")
	}

	logger.Infof("Checking CPU cores: Required >= %d, Actual = %d.", s.MinCores, actualCores)

	if actualCores < int(*s.MinCores) {
		return errors.Errorf("CPU core requirement not met: Required >= %d, but found only %d cores", s.MinCores, actualCores)
	}

	logger.Info("CPU core requirement is met.")
	return nil
}

func (s *CheckCPUStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}
