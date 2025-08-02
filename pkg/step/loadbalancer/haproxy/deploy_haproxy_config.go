package haproxy

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DeployHAProxyConfigStep struct {
	step.Base
}

func NewDeployHAProxyConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DeployHAProxyConfigStep] {
	s := &DeployHAProxyConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Deploy haproxy config file"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DeployHAProxyConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, ok := ctx.Get("haproxy.cfg")
	if !ok {
		return fmt.Errorf("haproxy.cfg not found in context")
	}

	configBytes, ok := configContent.([]byte)
	if !ok {
		return fmt.Errorf("haproxy.cfg in context is not of type []byte")
	}

	remotePath := "/etc/haproxy/haproxy.cfg"
	logger.Infof("Deploying haproxy config to %s", remotePath)

	if err := runner.WriteFile(ctx.GoContext(), conn, configBytes, remotePath, "0644", true); err != nil {
		return fmt.Errorf("failed to deploy haproxy config file: %w", err)
	}

	return nil
}

func (s *DeployHAProxyConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remotePath := "/etc/haproxy/haproxy.cfg"
	logger.Warnf("Rolling back by removing %s", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, true, false); err != nil {
		logger.Errorf("Failed to remove haproxy config during rollback: %v", err)
	}

	return nil
}

func (s *DeployHAProxyConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
