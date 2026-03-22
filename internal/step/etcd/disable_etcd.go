package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type DisableEtcdStep struct {
	step.Base
	ServiceName string
}

type DisableEtcdStepBuilder struct {
	step.Builder[DisableEtcdStepBuilder, *DisableEtcdStep]
}

func NewDisableEtcdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableEtcdStepBuilder {
	s := &DisableEtcdStep{
		ServiceName: "etcd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable etcd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableEtcdStepBuilder).Init(s)
	return b
}

func (b *DisableEtcdStepBuilder) WithServiceName(name string) *DisableEtcdStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (s *DisableEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to get host facts for precheck: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if etcd service is enabled, proceeding with run phase.", "error", err)
		return false, nil
	}

	if !enabled {
		logger.Info("Etcd service is already disabled. Step is done.", "service", s.ServiceName)
		return true, nil
	}

	logger.Info("Etcd service is still enabled. Step needs to run.", "service", s.ServiceName)
	return false, nil
}

func (s *DisableEtcdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		err = fmt.Errorf("failed to get host facts to disable service: %w", err)
		result.MarkFailed(err, "Failed to get host facts")
		return result, err
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get connector")
		return result, err
	}

	logger.Info("Disabling etcd service from starting on boot...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		err = fmt.Errorf("failed to disable etcd service %s: %w", s.ServiceName, err)
		result.MarkFailed(err, "Failed to disable service")
		return result, err
	}

	logger.Info("Etcd service disabled successfully.", "service", s.ServiceName)
	result.MarkCompleted("Service disabled successfully")
	return result, nil
}

func (s *DisableEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		logger.Error(err, "Failed to get host facts for rollback")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	logger.Warn("Rolling back by re-enabling etcd service...", "service", s.ServiceName)
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Error(err, "Failed to re-enable etcd service during rollback")
	}

	return nil
}

var _ step.Step = (*DisableEtcdStep)(nil)
