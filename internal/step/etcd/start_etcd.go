package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StartEtcdStep struct {
	step.Base
	ServiceName string
}

type StartEtcdStepBuilder struct {
	step.Builder[StartEtcdStepBuilder, *StartEtcdStep]
}

func NewStartEtcdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StartEtcdStepBuilder {
	s := &StartEtcdStep{
		ServiceName: "etcd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start etcd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StartEtcdStepBuilder).Init(s)
	return b
}

func (b *StartEtcdStepBuilder) WithServiceName(name string) *StartEtcdStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (s *StartEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to get host facts for precheck: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if etcd service is active, proceeding with run phase.", "error", err)
		return false, nil
	}

	if active {
		logger.Info("Etcd service is already active. Step is done.", "service", s.ServiceName)
		return true, nil
	}

	logger.Info("Etcd service is not active. Step needs to run.", "service", s.ServiceName)
	return false, nil
}

func (s *StartEtcdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get connector")
		return result, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		err = fmt.Errorf("failed to get host facts to start service: %w", err)
		result.MarkFailed(err, "Failed to get host facts")
		return result, err
	}

	logger.Info("Starting etcd service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		err = fmt.Errorf("failed to start etcd service %s: %w", s.ServiceName, err)
		result.MarkFailed(err, "Failed to start service")
		return result, err
	}
	time.Sleep(5 * time.Second)

	logger.Info("Etcd service started successfully.", "service", s.ServiceName)
	result.MarkCompleted("Service started successfully")
	return result, nil
}

func (s *StartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		logger.Error(err, "Failed to get host facts for rollback")
		return nil
	}

	logger.Warn("Rolling back by stopping etcd service...", "service", s.ServiceName)
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Error(err, "Failed to stop etcd service during rollback")
	}

	return nil
}

var _ step.Step = (*StartEtcdStep)(nil)
