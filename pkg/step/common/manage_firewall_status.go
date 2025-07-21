package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type FirewallServiceState string

const (
	FirewallServiceRunning  FirewallServiceState = "running"
	FirewallServiceStopped  FirewallServiceState = "stopped"
	FirewallServiceEnabled  FirewallServiceState = "enabled"
	FirewallServiceDisabled FirewallServiceState = "disabled"
)

type ManageFirewallStateStep struct {
	step.Base
	State FirewallServiceState
}

type ManageFirewallStateStepBuilder struct {
	step.Builder[ManageFirewallStateStepBuilder, *ManageFirewallStateStep]
}

func NewManageFirewallStateStepBuilder(ctx runtime.ExecutionContext, instanceName string, state FirewallServiceState) *ManageFirewallStateStepBuilder {
	cs := &ManageFirewallStateStep{
		State: state,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure firewall service state is [%s]", instanceName, state)
	cs.Base.Sudo = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageFirewallStateStepBuilder).Init(cs)
}

func (s *ManageFirewallStateStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageFirewallStateStep) getFirewallServiceNameAndTool(ctx runtime.ExecutionContext) (serviceName string, tool FirewallToolType, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", ToolUnknown, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "firewalld"); err == nil {
		return "firewalld", ToolFirewalld, nil
	}
	if _, err := runner.LookPath(ctx.GoContext(), conn, "ufw"); err == nil {
		return "ufw", ToolUFW, nil
	}
	return "", ToolUnknown, fmt.Errorf("no supported firewall service (firewalld, ufw) found for state management")
}

func (s *ManageFirewallStateStep) getSystemdCmds(serviceName string) (checkCmd, runCmd, rollbackCmd string) {
	switch s.State {
	case FirewallServiceRunning:
		checkCmd = fmt.Sprintf("systemctl is-active --quiet %s", serviceName)
		runCmd = fmt.Sprintf("systemctl start %s", serviceName)
		rollbackCmd = fmt.Sprintf("systemctl stop %s", serviceName)
	case FirewallServiceStopped:
		checkCmd = fmt.Sprintf("! systemctl is-active --quiet %s", serviceName)
		runCmd = fmt.Sprintf("systemctl stop %s", serviceName)
		rollbackCmd = fmt.Sprintf("systemctl start %s", serviceName)
	case FirewallServiceEnabled:
		checkCmd = fmt.Sprintf("systemctl is-enabled --quiet %s", serviceName)
		runCmd = fmt.Sprintf("systemctl enable %s", serviceName)
		rollbackCmd = fmt.Sprintf("systemctl disable %s", serviceName)
	case FirewallServiceDisabled:
		checkCmd = fmt.Sprintf("! systemctl is-enabled --quiet %s", serviceName)
		runCmd = fmt.Sprintf("systemctl disable %s", serviceName)
		rollbackCmd = fmt.Sprintf("systemctl enable %s", serviceName)
	}
	return
}

func (s *ManageFirewallStateStep) getUfwCmds() (checkCmd, runCmd, rollbackCmd string) {
	switch s.State {
	case FirewallServiceRunning, FirewallServiceEnabled:
		checkCmd = "ufw status | grep -q 'Status: active'"
		runCmd = "ufw --force enable"
		rollbackCmd = "ufw disable"
	case FirewallServiceStopped, FirewallServiceDisabled:
		checkCmd = "ufw status | grep -q 'Status: inactive'"
		runCmd = "ufw disable"
		rollbackCmd = "ufw --force enable"
	}
	return
}

func (s *ManageFirewallStateStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	serviceName, tool, err := s.getFirewallServiceNameAndTool(ctx)
	if err != nil {
		logger.Warnf("Could not determine firewall service, assuming step needs to run. %v", err)
		return false, nil
	}
	logger.Infof("Detected firewall service: %s (%s)", serviceName, tool)

	var checkCmd string
	if tool == ToolUFW && (s.State == FirewallServiceRunning || s.State == FirewallServiceStopped) {
		checkCmd, _, _ = s.getUfwCmds()
	} else {
		checkCmd, _, _ = s.getSystemdCmds(serviceName)
	}

	_, checkErr := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if checkErr == nil {
		logger.Infof("Firewall service state is already '%s'. Step considered done.", s.State)
		return true, nil
	}

	return false, nil
}

func (s *ManageFirewallStateStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	serviceName, tool, err := s.getFirewallServiceNameAndTool(ctx)
	if err != nil {
		return err
	}

	var runCmd string
	if tool == ToolUFW && (s.State == FirewallServiceRunning || s.State == FirewallServiceStopped) {
		_, runCmd, _ = s.getUfwCmds()
	} else {
		_, runCmd, _ = s.getSystemdCmds(serviceName)
	}

	logger.Infof("Executing command to set firewall state to '%s': %s", s.State, runCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, runCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to set firewall service state to '%s': %w", s.State, err)
	}

	logger.Infof("Firewall service state successfully set to '%s'.", s.State)
	return nil
}

func (s *ManageFirewallStateStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	serviceName, tool, err := s.getFirewallServiceNameAndTool(ctx)
	if err != nil {
		logger.Warnf("No firewall service found, cannot perform rollback. %v", err)
		return nil
	}

	var rollbackCmd string
	if tool == ToolUFW && (s.State == FirewallServiceRunning || s.State == FirewallServiceStopped) {
		_, _, rollbackCmd = s.getUfwCmds()
	} else {
		_, _, rollbackCmd = s.getSystemdCmds(serviceName)
	}

	if rollbackCmd == "" {
		logger.Warn("No rollback action defined for this state.")
		return nil
	}

	logger.Warnf("Attempting to rollback firewall service state by executing: %s", rollbackCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, rollbackCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to rollback firewall service state (best effort): %v", err)
	}
	return nil
}

var _ step.Step = (*ManageFirewallStateStep)(nil)
