package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartKubeApiServerStep struct {
	step.Base
}

type RestartKubeApiServerStepBuilder struct {
	step.Builder[RestartKubeApiServerStepBuilder, *RestartKubeApiServerStep]
}

func NewRestartKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *RestartKubeApiServerStepBuilder {
	s := &RestartKubeApiServerStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *RestartKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to restart service: %w", err)
	}

	logger.Infof("Restarting kube-apiserver service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, KubeApiServerServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", KubeApiServerServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart kube-apiserver service. Recent logs:\n%s", out)
		return fmt.Errorf("failed to restart kube-apiserver service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, KubeApiServerServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify kube-apiserver service status after restarting: %w", err)
	}
	if !active {
		return fmt.Errorf("kube-apiserver service did not become active after restart command")
	}

	logger.Info("KubeApiServer service restarted successfully.")
	return nil
}

func (s *RestartKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}
