package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	KubeControllerManagerServiceName = "kube-controller-manager"
)

type EnableKubeControllerManagerStep struct {
	step.Base
}

type EnableKubeControllerManagerStepBuilder struct {
	step.Builder[EnableKubeControllerManagerStepBuilder, *EnableKubeControllerManagerStep]
}

func NewEnableKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *EnableKubeControllerManagerStepBuilder {
	s := &EnableKubeControllerManagerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable kube-controller-manager service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *EnableKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, KubeControllerManagerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if kube-controller-manager service is enabled, assuming it needs to be. Error: %v", err)
		return false, nil
	}

	if enabled {
		logger.Infof("KubeControllerManager service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("KubeControllerManager service is not enabled. Step needs to run.")
	return false, nil
}

func (s *EnableKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Enabling kube-controller-manager service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, KubeControllerManagerServiceName); err != nil {
		return fmt.Errorf("failed to enable kube-controller-manager service: %w", err)
	}

	logger.Info("KubeControllerManager service enabled successfully.")
	return nil
}

func (s *EnableKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by disabling kube-controller-manager service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, KubeControllerManagerServiceName); err != nil {
		logger.Errorf("Failed to disable kube-controller-manager service during rollback: %v", err)
	}

	return nil
}
