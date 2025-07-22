package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopEtcdStep struct {
	step.Base
	ServiceName string
}

type StopEtcdStepBuilder struct {
	step.Builder[StopEtcdStepBuilder, *StopEtcdStep]
}

func NewStopEtcdStepBuilder(ctx runtime.Context, instanceName string) *StopEtcdStepBuilder {
	s := &StopEtcdStep{
		ServiceName: "etcd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop etcd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopEtcdStepBuilder).Init(s)
	return b
}

func (b *StopEtcdStepBuilder) WithServiceName(name string) *StopEtcdStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (s *StopEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if etcd service is active, proceeding with run phase.", "error", err)
		return false, nil
	}

	if !active {
		logger.Info("Etcd service is already inactive. Step is done.", "service", s.ServiceName)
		return true, nil
	}

	logger.Info("Etcd service is still active. Step needs to run.", "service", s.ServiceName)
	return false, nil
}

func (s *StopEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts to stop service: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Stopping etcd service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to stop etcd service %s: %w", s.ServiceName, err)
	}

	logger.Info("Etcd service stopped successfully.", "service", s.ServiceName)
	return nil
}

func (s *StopEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warn("Rolling back by restarting etcd service...", "service", s.ServiceName)
	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Error(err, "Failed to restart etcd service during rollback")
	}

	return nil
}

var _ step.Step = (*StopEtcdStep)(nil)
