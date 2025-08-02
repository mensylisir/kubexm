package nginx

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallNginxStep struct {
	step.Base
}

func NewInstallNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *InstallNginxStep] {
	s := &InstallNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Install nginx package"
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *InstallNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get update && apt-get install -y nginx"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum install -y nginx"
	} else {
		return fmt.Errorf("unsupported operating system for nginx installation: %s", os)
	}

	logger.Info("Installing nginx package...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		return fmt.Errorf("failed to install nginx: %w", err)
	}

	logger.Info("nginx package installed successfully.")
	return nil
}

func (s *InstallNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get remove -y nginx"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum remove -y nginx"
	} else {
		logger.Warnf("Unsupported OS for rollback, skipping nginx removal: %s", os)
		return nil
	}

	logger.Warn("Rolling back nginx installation...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		logger.Errorf("Failed to rollback nginx installation: %v", err)
	}

	return nil
}

func (s *InstallNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
