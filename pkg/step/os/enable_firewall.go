package os

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

var knownFirewallServices = []string{"firewalld", "ufw"}

func IsIn(target string, list []string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

type EnableFirewallStep struct {
	step.Base
	serviceEnabledInRun string
}

type EnableFirewallStepBuilder struct {
	step.Builder[EnableFirewallStepBuilder, *EnableFirewallStep]
}

func NewEnableFirewallStepBuilder(ctx runtime.ExecutionContext, instanceName string) *EnableFirewallStepBuilder {
	s := &EnableFirewallStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Enable firewall", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(EnableFirewallStepBuilder).Init(s)
	return b
}

func (s *EnableFirewallStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableFirewallStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, service := range knownFirewallServices {
		status, _ := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-active %s", service), s.Sudo)
		if strings.TrimSpace(status) == "active" {
			logger.Infof("Firewall service '%s' is already active. Nothing to do.", service)
			return true, nil
		}
	}

	logger.Info("No active firewall services found. Firewall needs to be enabled.")
	return false, nil
}

func (s *EnableFirewallStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Gathering host facts to determine firewall service to enable...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather host facts")
	}
	osID := facts.OS.ID

	var serviceToEnable string
	switch {
	case IsIn(osID, common.RedHatFamilyDistributions):
		serviceToEnable = "firewalld"
	case IsIn(osID, common.DebianFamilyDistributions):
		serviceToEnable = "ufw"
	default:
		logger.Warnf("Unknown OS family for '%s'. Cannot determine a default firewall to enable. Skipping.", osID)
		return nil
	}

	logger.Infof("Identified firewall service to enable for OS '%s': %s", osID, serviceToEnable)

	logger.Infof("Enabling service '%s' to start on boot...", serviceToEnable)
	if err := runner.EnableService(ctx.GoContext(), conn, facts, serviceToEnable); err != nil {
		return errors.Wrapf(err, "failed to enable service '%s'", serviceToEnable)
	}

	logger.Infof("Starting service '%s'...", serviceToEnable)
	if err := runner.StartService(ctx.GoContext(), conn, facts, serviceToEnable); err != nil {
		return errors.Wrapf(err, "failed to start service '%s'", serviceToEnable)
	}

	s.serviceEnabledInRun = serviceToEnable

	logger.Infof("Firewall service '%s' enabled and started successfully.", serviceToEnable)
	return nil
}

func (s *EnableFirewallStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.serviceEnabledInRun == "" {
		logger.Info("Nothing to roll back as no firewall service was enabled in the run step.")
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return errors.Wrap(err, "failed to gather host facts for rollback")
	}

	logger.Infof("Attempting to roll back firewall enablement by disabling '%s'...", s.serviceEnabledInRun)

	logger.Infof("Stopping service '%s'...", s.serviceEnabledInRun)
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.serviceEnabledInRun); err != nil {
		logger.Warnf("Failed to stop service '%s' during rollback. Error: %v", s.serviceEnabledInRun, err)
	}

	logger.Infof("Disabling service '%s'...", s.serviceEnabledInRun)
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.serviceEnabledInRun); err != nil {
		logger.Warnf("Failed to disable service '%s' during rollback. Error: %v", s.serviceEnabledInRun, err)
	}

	logger.Infof("Firewall state has been rolled back.")
	return nil
}
