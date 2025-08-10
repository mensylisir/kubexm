package os

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common" // 确保已导入 common 包
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

var knownFirewallServices = []string{"firewalld", "ufw", "iptables"}

type DisableFirewallStep struct {
	step.Base
	activeServicesBeforeRun []string
}

type DisableFirewallStepBuilder struct {
	step.Builder[DisableFirewallStepBuilder, *DisableFirewallStep]
}

func NewDisableFirewallStepBuilder(ctx runtime.Context, instanceName string) *DisableFirewallStepBuilder {
	s := &DisableFirewallStep{
		activeServicesBeforeRun: make([]string, 0),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Disable firewall", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DisableFirewallStepBuilder).Init(s)
	return b
}

func (s *DisableFirewallStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableFirewallStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	if !*ctx.GetClusterConfig().Spec.Preflight.DisableFirewalld {
		return true, nil
	}
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, service := range knownFirewallServices {
		status, _ := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-active %s", service), s.Sudo)
		if strings.TrimSpace(status) == "active" {
			logger.Infof("Firewall service '%s' is active and needs to be disabled.", service)
			return false, nil
		}
	}

	logger.Info("No active firewall services found. Firewall is considered disabled.")
	return true, nil
}

func IsIn(target string, slice []string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

func (s *DisableFirewallStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Gathering host facts to determine firewall service...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather host facts")
	}
	osID := facts.OS.ID

	var servicesToDisable []string
	switch {
	case IsIn(osID, common.RedHatFamilyDistributions):
		servicesToDisable = append(servicesToDisable, "firewalld")
	case IsIn(osID, common.DebianFamilyDistributions):
		servicesToDisable = append(servicesToDisable, "ufw")
	default:
		logger.Warnf("Unknown OS family for '%s'. Attempting to disable all known firewall services as a fallback.", osID)
		servicesToDisable = knownFirewallServices
	}

	logger.Infof("Identified services to disable for OS '%s': %v", osID, servicesToDisable)

	for _, service := range servicesToDisable {
		isActive, _ := runner.IsServiceActive(ctx.GoContext(), conn, facts, service)
		if isActive {
			s.activeServicesBeforeRun = append(s.activeServicesBeforeRun, service)

			logger.Infof("Stopping service '%s'...", service)
			if err := runner.StopService(ctx.GoContext(), conn, facts, service); err != nil {
				logger.Warnf("Failed to stop service '%s', it may not exist or was already stopped. Error: %v", service, err)
			}
		}

		logger.Infof("Disabling service '%s' from startup...", service)
		if err := runner.DisableService(ctx.GoContext(), conn, facts, service); err != nil {
			logger.Warnf("Failed to disable service '%s', it may not exist. Error: %v", service, err)
		}
	}

	logger.Info("Firewall disable step completed successfully.")
	return nil
}

func (s *DisableFirewallStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if len(s.activeServicesBeforeRun) == 0 {
		logger.Info("Nothing to roll back as no active firewall services were disabled.")
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather host facts for rollback")
	}

	logger.Info("Attempting to roll back firewall changes...")
	for _, service := range s.activeServicesBeforeRun {
		logger.Infof("Re-enabling service '%s'...", service)
		if err := runner.EnableService(ctx.GoContext(), conn, facts, service); err != nil {
			logger.Warnf("Failed to enable service '%s' during rollback. Error: %v", service, err)
		}

		logger.Infof("Restarting service '%s'...", service)
		if err := runner.RestartService(ctx.GoContext(), conn, facts, service); err != nil {
			logger.Warnf("Failed to restart service '%s' during rollback. Error: %v", service, err)
		}
	}

	logger.Infof("Firewall state has been rolled back.")
	return nil
}
