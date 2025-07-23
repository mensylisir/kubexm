package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopKubeApiServerStep struct {
	step.Base
}

type StopKubeApiServerStepBuilder struct {
	step.Builder[StopKubeApiServerStepBuilder, *StopKubeApiServerStep]
}

func NewStopKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *StopKubeApiServerStepBuilder {
	s := &StopKubeApiServerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *StopKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		logger.Warnf("Failed to check service status, assuming it's not active. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("KubeApiServer service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is currently active. Step needs to run.")
	return false, nil
}

func (s *StopKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return err
	}

	logger.Infof("Stopping kube-apiserver service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		return fmt.Errorf("failed to stop kube-apiserver service: %w", err)
	}

	logger.Info("KubeApiServer service stopped successfully.")
	return nil
}

func (s *StopKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot start service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by starting kube-apiserver service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logger.Errorf("Failed to start kube-apiserver service during rollback: %v", err)
	}

	return nil
}
