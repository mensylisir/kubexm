package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartEtcdStep struct {
	step.Base
	ServiceName string
}

type RestartEtcdStepBuilder struct {
	step.Builder[RestartEtcdStepBuilder, *RestartEtcdStep]
}

func NewRestartEtcdStepBuilder(ctx runtime.Context, instanceName string) *RestartEtcdStepBuilder {
	s := &RestartEtcdStep{
		ServiceName: "etcd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart etcd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RestartEtcdStepBuilder).Init(s)
	return b
}

func (b *RestartEtcdStepBuilder) WithServiceName(name string) *RestartEtcdStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (s *RestartEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts to restart service: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Restarting etcd service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to restart etcd service %s: %w", s.ServiceName, err)
	}

	time.Sleep(3 * time.Second)

	logger.Info("Etcd service restarted successfully.", "service", s.ServiceName)
	return nil
}

func (s *RestartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for a restart step is typically a no-op.")
	return nil
}

var _ step.Step = (*RestartEtcdStep)(nil)
