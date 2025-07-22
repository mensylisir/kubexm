package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableKubeApiServerStep struct {
	step.Base
}

type DisableKubeApiServerStepBuilder struct {
	step.Builder[DisableKubeApiServerStepBuilder, *DisableKubeApiServerStep]
}

func NewDisableKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *DisableKubeApiServerStepBuilder {
	s := &DisableKubeApiServerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *DisableKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service is enabled, assuming it's disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("KubeApiServer service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is currently enabled. Step needs to run.")
	return false, nil
}

func (s *DisableKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Disabling kube-apiserver service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		return fmt.Errorf("failed to disable kube-apiserver service: %w", err)
	}

	logger.Info("KubeApiServer service disabled successfully.")
	return nil
}

func (s *DisableKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot enable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by enabling kube-apiserver service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logger.Errorf("Failed to enable kube-apiserver service during rollback: %v", err)
	}

	return nil
}
