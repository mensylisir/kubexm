package keepalived

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DeployKeepalivedConfigStep struct {
	step.Base
}

func NewDeployKeepalivedConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DeployKeepalivedConfigStep] {
	s := &DeployKeepalivedConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Deploy keepalived config file"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DeployKeepalivedConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, ok := ctx.Get("keepalived.conf")
	if !ok {
		return fmt.Errorf("keepalived.conf not found in context")
	}

	configBytes, ok := configContent.([]byte)
	if !ok {
		return fmt.Errorf("keepalived.conf in context is not of type []byte")
	}

	remotePath := "/etc/keepalived/keepalived.conf"
	logger.Infof("Deploying keepalived config to %s", remotePath)

	if err := runner.WriteFile(ctx.GoContext(), conn, configBytes, remotePath, "0644", true); err != nil {
		return fmt.Errorf("failed to deploy keepalived config file: %w", err)
	}

	return nil
}

func (s *DeployKeepalivedConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remotePath := "/etc/keepalived/keepalived.conf"
	logger.Warnf("Rolling back by removing %s", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, true, false); err != nil {
		logger.Errorf("Failed to remove keepalived config during rollback: %v", err)
	}

	return nil
}

func (s *DeployKeepalivedConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
