package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckTimeSyncStep struct {
	step.Base
}

type CheckTimeSyncStepBuilder struct {
	step.Builder[CheckTimeSyncStepBuilder, *CheckTimeSyncStep]
}

func NewCheckTimeSyncStepBuilder(ctx runtime.Context, instanceName string) *CheckTimeSyncStepBuilder {
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

func (s *CheckTimeSyncStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking NTP time synchronization status...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	chronyCmd := "chronyc tracking"
	if _, err := runner.Run(ctx.GoContext(), conn, chronyCmd, s.Sudo); err == nil {
		stdout, err := runner.Run(ctx.GoContext(), conn, chronyCmd, s.Sudo)
		if err != nil {
			return fmt.Errorf("failed to execute 'chronyc tracking': %w", err)
		}

		output := string(stdout)
		if !strings.Contains(output, "Leap status     : Normal") {
			logger.Errorf("chrony reports abnormal leap status. Full output:\n%s", output)
			return fmt.Errorf("NTP time synchronization is not healthy according to chrony")
		}
		logger.Info("Time is synchronized according to 'chronyc'.")
		return nil
	}

	logger.Info("'chronyc' not found or failed, falling back to 'ntpstat'...")
	ntpstatCmd := "ntpstat"
	if _, err := runner.Run(ctx.GoContext(), conn, ntpstatCmd, s.Sudo); err == nil {

		logger.Info("Time is synchronized according to 'ntpstat'.")
		return nil
	} else {
		stdout, _ := runner.Run(ctx.GoContext(), conn, ntpstatCmd, s.Sudo)
		logger.Errorf("ntpstat check failed. Full output:\n%s", string(stdout))
		return fmt.Errorf("NTP time synchronization check failed with ntpstat: %w", err)
	}
}

func (s *CheckTimeSyncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}

var _ step.Step = (*CheckTimeSyncStep)(nil)
