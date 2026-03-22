package etcd

import (
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartEtcdStep struct {
	step.Base
}

type RestartEtcdStepBuilder struct {
	step.Builder[RestartEtcdStepBuilder, *RestartEtcdStep]
}

func NewRestartEtcdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartEtcdStepBuilder {
	s := &RestartEtcdStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart the etcd service on the current node"
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(RestartEtcdStepBuilder).Init(s)
	return b
}

func (s *RestartEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if etcd is running")
	return false, nil
}

func (s *RestartEtcdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get host connector")
		return result, err
	}

	logger.Info("Restarting etcd...")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts for precheck")
		return result, err
	}

	if err := runner.RestartService(ctx.GoContext(), conn, facts, "etcd.service"); err != nil {
		result.MarkFailed(err, "failed to restart etcd service")
		return result, err
	}

	logger.Info("Etcd service restart command issued successfully.")
	result.MarkCompleted("etcd restarted successfully")
	return result, nil
}

func (s *RestartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not required for this step.")
	return nil
}

var _ step.Step = (*RestartEtcdStep)(nil)
