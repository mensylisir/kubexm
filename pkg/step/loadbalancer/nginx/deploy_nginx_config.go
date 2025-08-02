package nginx

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DeployNginxConfigStep struct {
	step.Base
}

func NewDeployNginxConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DeployNginxConfigStep] {
	s := &DeployNginxConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Deploy nginx config file"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DeployNginxConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, ok := ctx.Get("nginx.conf")
	if !ok {
		return fmt.Errorf("nginx.conf not found in context")
	}

	configBytes, ok := configContent.([]byte)
	if !ok {
		return fmt.Errorf("nginx.conf in context is not of type []byte")
	}

	// The location for stream configs can vary. /etc/nginx/conf.d/ is common.
	remotePath := "/etc/nginx/conf.d/kube_apiserver.conf"
	logger.Infof("Deploying nginx stream config to %s", remotePath)

	if err := runner.WriteFile(ctx.GoContext(), conn, configBytes, remotePath, "0644", true); err != nil {
		return fmt.Errorf("failed to deploy nginx config file: %w", err)
	}

	return nil
}

func (s *DeployNginxConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remotePath := "/etc/nginx/conf.d/kube_apiserver.conf"
	logger.Warnf("Rolling back by removing %s", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, true, false); err != nil {
		logger.Errorf("Failed to remove nginx config during rollback: %v", err)
	}

	return nil
}

func (s *DeployNginxConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
