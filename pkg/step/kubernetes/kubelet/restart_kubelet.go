package kubelet

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartKubeletStep struct {
	step.Base
	ServiceName    string
	postWaitSettle time.Duration
}

type RestartKubeletStepBuilder struct {
	step.Builder[RestartKubeletStepBuilder, *RestartKubeletStep]
}

func NewRestartKubeletStepBuilder(ctx runtime.Context, instanceName string) *RestartKubeletStepBuilder {
	s := &RestartKubeletStep{
		ServiceName:    "kubelet.service",
		postWaitSettle: 15 * time.Second,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart the kubelet service to apply new configuration"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartKubeletStepBuilder).Init(s)
	return b
}

func (s *RestartKubeletStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKubeletStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	exists, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' exists, assuming it does. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if !exists {
		logger.Errorf("Service '%s' does not exist on the host.", s.ServiceName)
		return false, fmt.Errorf("precheck failed: service '%s' not found", s.ServiceName)
	}

	logger.Infof("Precheck passed: Service '%s' exists.", s.ServiceName)
	return false, nil
}

func (s *RestartKubeletStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Info("Reloading systemd daemon before restarting service...")
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		logger.Warnf("Failed to run daemon-reload, continuing with restart anyway. Error: %v", err)
	}

	logger.Infof("Restarting service: %s", s.ServiceName)
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to restart service '%s' on host %s: %w", s.ServiceName, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Service '%s' restart signal sent. Waiting for %v to settle...", s.ServiceName, s.postWaitSettle)
	time.Sleep(s.postWaitSettle)

	logger.Infof("Service '%s' has been restarted.", s.ServiceName)
	return nil
}

func (s *RestartKubeletStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Attempting to restart kubelet again as part of rollback...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot restart service. Error: %v", err)
		return err
	}

	logger.Warnf("Rolling back by restarting service again: %s", s.ServiceName)
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("CRITICAL: Failed to restart service '%s' during rollback. The node might be in a 'NotReady' state. MANUAL INTERVENTION REQUIRED. Error: %v", s.ServiceName, err)
		return err
	}

	logger.Info("Rollback restart of service completed.")
	return nil
}

var _ step.Step = (*RestartKubeletStep)(nil)
