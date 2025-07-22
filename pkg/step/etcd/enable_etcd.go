package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableEtcdStep struct {
	step.Base
	ServiceName string
}

type EnableEtcdStepBuilder struct {
	step.Builder[EnableEtcdStepBuilder, *EnableEtcdStep]
}

func NewEnableEtcdStepBuilder(ctx runtime.Context, instanceName string) *EnableEtcdStepBuilder {
	s := &EnableEtcdStep{
		ServiceName: "etcd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable etcd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableEtcdStepBuilder).Init(s)
	return b
}

func (b *EnableEtcdStepBuilder) WithServiceName(name string) *EnableEtcdStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (s *EnableEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if etcd service is enabled, proceeding with run phase.", "error", err)
		return false, nil
	}

	if enabled {
		logger.Info("Etcd service is already enabled. Step is done.", "service", s.ServiceName)
		return true, nil
	}

	logger.Info("Etcd service is not enabled. Step needs to run.", "service", s.ServiceName)
	return false, nil
}

func (s *EnableEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts to enable service: %w", err)
	}

	logger.Info("Enabling etcd service to start on boot...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to enable etcd service %s: %w", s.ServiceName, err)
	}

	logger.Info("Etcd service enabled successfully.", "service", s.ServiceName)
	return nil
}

func (s *EnableEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warn("Rolling back by disabling etcd service...", "service", s.ServiceName)
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Error(err, "Failed to disable etcd service during rollback")
	}

	return nil
}

var _ step.Step = (*EnableEtcdStep)(nil)
