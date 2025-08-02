package keepalived

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallKeepalivedStep struct {
	step.Base
}

func NewInstallKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *InstallKeepalivedStep] {
	s := &InstallKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Install keepalived package"
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *InstallKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get update && apt-get install -y keepalived"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum install -y keepalived"
	} else {
		return fmt.Errorf("unsupported operating system for keepalived installation: %s", os)
	}

	logger.Info("Installing keepalived package...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		return fmt.Errorf("failed to install keepalived: %w", err)
	}

	logger.Info("keepalived package installed successfully.")
	return nil
}

func (s *InstallKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil // Avoid failing rollback on connection error
	}

	os := ctx.GetHost().GetOS()
	var cmd string
	if strings.Contains(os, "Ubuntu") || strings.Contains(os, "Debian") {
		cmd = "apt-get remove -y keepalived"
	} else if strings.Contains(os, "CentOS") || strings.Contains(os, "Red Hat") {
		cmd = "yum remove -y keepalived"
	} else {
		logger.Warnf("Unsupported OS for rollback, skipping keepalived removal: %s", os)
		return nil
	}

	logger.Warn("Rolling back keepalived installation...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		logger.Errorf("Failed to rollback keepalived installation: %v", err)
	}

	return nil
}

func (s *InstallKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
