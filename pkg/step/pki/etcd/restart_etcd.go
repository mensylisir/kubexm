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
}

func NewRestartEtcdStep(ctx runtime.Context, instanceName string) *RestartEtcdStep {
	s := &RestartEtcdStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart the etcd service on the current node"
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *RestartEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if etcd is running")
	return false, nil
}

func (s *RestartEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Restarting etcd...")

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts for precheck: %w", err)
	}

	if err := runner.RestartService(ctx.GoContext(), conn, facts, "etcd.service"); err != nil {
		return fmt.Errorf("failed to restart etcd service: %w", err)
	}

	logger.Info("Etcd service restart command issued successfully.")
	return nil
}

func (s *RestartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not required for this step.")
	return nil
}

var _ step.Step = (*RestartEtcdStep)(nil)
