package haproxy

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallHAProxyStep struct {
	step.Base
}

func NewInstallHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *InstallHAProxyStep] {
	s := &InstallHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Install haproxy package"
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *InstallHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get update && apt-get install -y haproxy"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum install -y haproxy"
	} else {
		return fmt.Errorf("unsupported operating system for haproxy installation: %s", os)
	}

	logger.Info("Installing haproxy package...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		return fmt.Errorf("failed to install haproxy: %w", err)
	}

	logger.Info("haproxy package installed successfully.")
	return nil
}

func (s *InstallHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get remove -y haproxy"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum remove -y haproxy"
	} else {
		logger.Warnf("Unsupported OS for rollback, skipping haproxy removal: %s", os)
		return nil
	}

	logger.Warn("Rolling back haproxy installation...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		logger.Errorf("Failed to rollback haproxy installation: %v", err)
	}

	return nil
}

func (s *InstallHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
