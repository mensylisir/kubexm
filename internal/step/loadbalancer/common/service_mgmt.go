package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// StopLBServiceStep stops a systemd service on remote hosts.
type StopLBServiceStep struct {
	step.Base
	ServiceName string
}

type StopLBServiceStepBuilder struct {
	step.Builder[StopLBServiceStepBuilder, *StopLBServiceStep]
}

func NewStopLBServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *StopLBServiceStepBuilder {
	s := &StopLBServiceStep{ServiceName: serviceName}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop LB service %s", s.Base.Meta.Name, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(StopLBServiceStepBuilder).Init(s)
}

func (s *StopLBServiceStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *StopLBServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, fmt.Sprintf("/etc/systemd/system/%s.service", s.ServiceName))
	if err != nil || !exists {
		return true, nil // Service doesn't exist, nothing to stop
	}
	status, err := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-active %s 2>/dev/null || true", s.ServiceName), false)
	if err != nil {
		return false, err
	}
	return status.Stdout != "active\n" && status.Stdout != "activating\n", nil
}

func (s *StopLBServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector"); return result, err
	}

	cmd := fmt.Sprintf("systemctl stop %s 2>/dev/null || true", s.ServiceName)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to stop %s", s.ServiceName)); return result, err
	}

	logger.Infof("LB service %s stopped successfully.", s.ServiceName)
	result.MarkCompleted(fmt.Sprintf("LB service %s stopped", s.ServiceName))
	return result, nil
}

func (s *StopLBServiceStep) Rollback(ctx runtime.ExecutionContext) error { return nil }

var _ step.Step = (*StopLBServiceStep)(nil)

// DisableLBServiceStep disables a systemd service on remote hosts.
type DisableLBServiceStep struct {
	step.Base
	ServiceName string
}

type DisableLBServiceStepBuilder struct {
	step.Builder[DisableLBServiceStepBuilder, *DisableLBServiceStep]
}

func NewDisableLBServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *DisableLBServiceStepBuilder {
	s := &DisableLBServiceStep{ServiceName: serviceName}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable LB service %s", s.Base.Meta.Name, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(DisableLBServiceStepBuilder).Init(s)
}

func (s *DisableLBServiceStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *DisableLBServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, fmt.Sprintf("/etc/systemd/system/%s.service", s.ServiceName))
	if err != nil || !exists {
		return true, nil // Service doesn't exist, nothing to disable
	}
	status, err := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-enabled %s 2>/dev/null || true", s.ServiceName), false)
	if err != nil {
		return false, err
	}
	return status.Stdout != "enabled\n" && status.Stdout != "enabled-runtime\n", nil
}

func (s *DisableLBServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector"); return result, err
	}

	cmd := fmt.Sprintf("systemctl disable %s 2>/dev/null || true", s.ServiceName)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to disable %s", s.ServiceName)); return result, err
	}

	logger.Infof("LB service %s disabled successfully.", s.ServiceName)
	result.MarkCompleted(fmt.Sprintf("LB service %s disabled", s.ServiceName))
	return result, nil
}

func (s *DisableLBServiceStep) Rollback(ctx runtime.ExecutionContext) error { return nil }

var _ step.Step = (*DisableLBServiceStep)(nil)

// RemoveLBFileStep removes a file on remote hosts.
type RemoveLBFileStep struct {
	step.Base
	FilePath string
}

type RemoveLBFileStepBuilder struct {
	step.Builder[RemoveLBFileStepBuilder, *RemoveLBFileStep]
}

func NewRemoveLBFileStepBuilder(ctx runtime.ExecutionContext, instanceName, filePath string) *RemoveLBFileStepBuilder {
	s := &RemoveLBFileStep{FilePath: filePath}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove LB file %s", s.Base.Meta.Name, filePath)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RemoveLBFileStepBuilder).Init(s)
}

func (s *RemoveLBFileStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *RemoveLBFileStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, s.FilePath)
	return !exists, nil
}

func (s *RemoveLBFileStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector"); return result, err
	}

	cmd := fmt.Sprintf("rm -f %s", s.FilePath)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to remove %s", s.FilePath)); return result, err
	}

	logger.Infof("File %s removed successfully.", s.FilePath)
	result.MarkCompleted(fmt.Sprintf("File %s removed", s.FilePath))
	return result, nil
}

func (s *RemoveLBFileStep) Rollback(ctx runtime.ExecutionContext) error { return nil }

var _ step.Step = (*RemoveLBFileStep)(nil)
