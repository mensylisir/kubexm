package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartKubeApiServerStep struct {
	step.Base
}

type StartKubeApiServerStepBuilder struct {
	step.Builder[StartKubeApiServerStepBuilder, *StartKubeApiServerStep]
}

func NewStartKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *StartKubeApiServerStepBuilder {
	s := &StartKubeApiServerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StartKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *StartKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		logger.Warnf("Failed to check if kube-apiserver service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("KubeApiServer service is already active. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is not active. Step needs to run.")
	return false, nil
}

func (s *StartKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to start service: %w", err)
	}

	logger.Infof("Starting kube-apiserver service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", KubeApiServerServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start kube-apiserver service. Recent logs:\n%s", out)

		return fmt.Errorf("failed to start kube-apiserver service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify kube-apiserver service status after starting: %w", err)
	}
	if !active {
		return fmt.Errorf("kube-apiserver service did not become active after start command")
	}

	logger.Info("KubeApiServer service started successfully.")
	return nil
}

func (s *StartKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot stop service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping kube-apiserver service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logger.Errorf("Failed to stop kube-apiserver service during rollback: %v", err)
	}

	return nil
}
