package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CheckTimeSyncStep struct {
	step.Base
}

type CheckTimeSyncStepBuilder struct {
	step.Builder[CheckTimeSyncStepBuilder, *CheckTimeSyncStep]
}

func NewCheckTimeSyncStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CheckTimeSyncStepBuilder {
	s := &CheckTimeSyncStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check if the node's time is synchronized with an NTP server"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckTimeSyncStepBuilder).Init(s)
	return b
}

func (s *CheckTimeSyncStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckTimeSyncStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if node's time is synchronized with an NTP server...")
	return false, nil
}

func (s *CheckTimeSyncStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking NTP time synchronization status...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get host connector")
		return result, err
	}

	chronyCmd := "chronyc tracking"
	if _, err := runner.Run(ctx.GoContext(), conn, chronyCmd, s.Sudo); err == nil {
		stdout, err := runner.Run(ctx.GoContext(), conn, chronyCmd, s.Sudo)
		if err != nil {
			err = fmt.Errorf("failed to execute 'chronyc tracking': %w", err)
			result.MarkFailed(err, "Failed to execute chronyc tracking")
			return result, err
		}

		output := string(stdout)
		if !strings.Contains(output, "Leap status     : Normal") {
			logger.Errorf("chrony reports abnormal leap status. Full output:\n%s", output)
			err = fmt.Errorf("NTP time synchronization is not healthy according to chrony")
			result.MarkFailed(err, "NTP time synchronization is not healthy")
			return result, err
		}
		logger.Info("Time is synchronized according to 'chronyc'.")
		result.MarkCompleted("Time is synchronized")
		return result, nil
	}

	logger.Info("'chronyc' not found or failed, falling back to 'ntpstat'...")
	ntpstatCmd := "ntpstat"
	if _, err := runner.Run(ctx.GoContext(), conn, ntpstatCmd, s.Sudo); err == nil {

		logger.Info("Time is synchronized according to 'ntpstat'.")
		result.MarkCompleted("Time is synchronized")
		return result, nil
	} else {
		stdout, _ := runner.Run(ctx.GoContext(), conn, ntpstatCmd, s.Sudo)
		logger.Errorf("ntpstat check failed. Full output:\n%s", string(stdout))
		err = fmt.Errorf("NTP time synchronization check failed with ntpstat: %w", err)
		result.MarkFailed(err, "NTP time synchronization check failed")
		return result, err
	}
}

func (s *CheckTimeSyncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}

var _ step.Step = (*CheckTimeSyncStep)(nil)
