package chrony

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifyTimeSyncStep struct {
	step.Base
	retryDelay time.Duration
}

type VerifyTimeSyncStepBuilder struct {
	step.Builder[VerifyTimeSyncStepBuilder, *VerifyTimeSyncStep]
}

func NewVerifyTimeSyncStepBuilder(ctx runtime.Context, instanceName string) *VerifyTimeSyncStepBuilder {
	s := &VerifyTimeSyncStep{
		retryDelay: 10 * time.Second,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Verify that the node's time is synchronized with an NTP server"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(VerifyTimeSyncStepBuilder).Init(s)
	return b
}

func (s *VerifyTimeSyncStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyTimeSyncStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for time synchronization verification...")

	synced, err := s.checkSyncStatus(ctx)
	if err != nil {
		logger.Infof("Precheck: Time synchronization status could not be determined. Step needs to run. (Error: %v)", err)
		return false, nil
	}
	if synced {
		logger.Info("Precheck: Time is already synchronized. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: Time is not yet synchronized.")
	return false, nil
}

func (s *VerifyTimeSyncStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Waiting for NTP time synchronization...")

	timeout := time.After(s.Base.Timeout)
	ticker := time.NewTicker(s.retryDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("time synchronization verification timed out after %v: %w", s.Base.Timeout, lastErr)
			}
			return fmt.Errorf("time synchronization verification timed out after %v", s.Base.Timeout)
		case <-ticker.C:
			synced, err := s.checkSyncStatus(ctx)
			if synced {
				logger.Info("Time synchronization successful!")
				return nil
			}
			if err != nil {
				lastErr = err
				logger.Debugf("Sync check attempt failed: %v", err)
			} else {
				lastErr = fmt.Errorf("time not synchronized yet")
				logger.Info("Time is not synchronized yet, retrying...")
			}
		}
	}
}

func (s *VerifyTimeSyncStep) checkSyncStatus(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	chronyCmd := "chronyc tracking"
	if _, err := runner.Run(ctx.GoContext(), conn, "command -v chronyc", s.Sudo); err == nil {
		stdout, err := runner.Run(ctx.GoContext(), conn, chronyCmd, s.Sudo)
		if err != nil {
			return false, fmt.Errorf("failed to execute 'chronyc tracking': %w", err)
		}
		if strings.Contains(string(stdout), "Leap status     : Normal") {
			return true, nil
		}
		return false, nil
	}

	ntpstatCmd := "ntpstat"
	if _, err := runner.Run(ctx.GoContext(), conn, "command -v ntpstat", s.Sudo); err == nil {
		if _, err := runner.Run(ctx.GoContext(), conn, ntpstatCmd, s.Sudo); err == nil {
			return true, nil
		}
		return false, nil
	}

	return false, fmt.Errorf("neither 'chronyc' nor 'ntpstat' command found")
}

func (s *VerifyTimeSyncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a verification step.")
	return nil
}

var _ step.Step = (*VerifyTimeSyncStep)(nil)
