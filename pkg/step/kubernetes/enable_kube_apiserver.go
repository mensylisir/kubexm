package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	KubeApiServerServiceName = "kube-apiserver"
)

type EnableKubeApiServerStep struct {
	step.Base
}

type EnableKubeApiServerStepBuilder struct {
	step.Builder[EnableKubeApiServerStepBuilder, *EnableKubeApiServerStep]
}

func NewEnableKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *EnableKubeApiServerStepBuilder {
	s := &EnableKubeApiServerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *EnableKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if kube-apiserver service is enabled, assuming it needs to be. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("KubeApiServer service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to enable service: %w", err)
	}

	logger.Infof("Enabling kube-apiserver service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		return fmt.Errorf("failed to enable kube-apiserver service: %w", err)
	}

	logger.Info("KubeApiServer service enabled successfully.")
	return nil
}

func (s *EnableKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot disable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling kube-apiserver service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logger.Errorf("Failed to disable kube-apiserver service during rollback: %v", err)
	}

	return nil
}
