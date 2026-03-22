package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartEtcdStep struct {
	step.Base
	ServiceName string
}

type RestartEtcdStepBuilder struct {
	step.Builder[RestartEtcdStepBuilder, *RestartEtcdStep]
}

func NewRestartEtcdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartEtcdStepBuilder {
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

func (s *RestartEtcdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		err = fmt.Errorf("failed to get host facts to restart service: %w", err)
		result.MarkFailed(err, "Failed to get host facts")
		return result, err
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get connector")
		return result, err
	}

	logger.Info("Restarting etcd service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		err = fmt.Errorf("failed to restart etcd service %s: %w", s.ServiceName, err)
		result.MarkFailed(err, "Failed to restart service")
		return result, err
	}

	time.Sleep(3 * time.Second)

	logger.Info("Etcd service restarted successfully.", "service", s.ServiceName)
	result.MarkCompleted("Service restarted successfully")
	return result, nil
}

func (s *RestartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for a restart step is typically a no-op.")
	return nil
}

var _ step.Step = (*RestartEtcdStep)(nil)
