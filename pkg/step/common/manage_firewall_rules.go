package common

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type FirewallRuleState string

const (
	FirewallRulePresent FirewallRuleState = "present"
	FirewallRuleAbsent  FirewallRuleState = "absent"
)

type FirewallToolType string

const (
	ToolFirewalld FirewallToolType = "firewalld"
	ToolUFW       FirewallToolType = "ufw"
	ToolIptables  FirewallToolType = "iptables"
	ToolUnknown   FirewallToolType = "unknown"
)

type FirewallRule struct {
	Port    string
	Service string
	Source  string
	Action  string
}

type ManageFirewallRuleStep struct {
	step.Base
	Rule  FirewallRule
	State FirewallRuleState
}

type ManageFirewallRuleStepBuilder struct {
	step.Builder[ManageFirewallRuleStepBuilder, *ManageFirewallRuleStep]
}

func NewManageFirewallRuleStepBuilder(ctx runtime.ExecutionContext, instanceName string, rule FirewallRule, state FirewallRuleState) *ManageFirewallRuleStepBuilder {
	cs := &ManageFirewallRuleStep{Rule: rule, State: state}
	cs.Base.Meta.Name = instanceName
	desc := ""
	if rule.Port != "" {
		desc = fmt.Sprintf("port %s", rule.Port)
	} else if rule.Service != "" {
		desc = fmt.Sprintf("service %s", rule.Service)
	}
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure firewall rule for %s is %s", instanceName, desc, state)
	cs.Base.Sudo = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageFirewallRuleStepBuilder).Init(cs)
}

func (b *ManageFirewallRuleStepBuilder) WithRule(rule FirewallRule) *ManageFirewallRuleStepBuilder {
	b.Step.Rule = rule
	return b
}

func (b *ManageFirewallRuleStepBuilder) WithState(state FirewallRuleState) *ManageFirewallRuleStepBuilder {
	b.Step.State = state
	return b
}

func (s *ManageFirewallRuleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageFirewallRuleStep) detectFirewallTool(ctx runtime.ExecutionContext) (FirewallToolType, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active --quiet firewalld", true); err == nil {
		return ToolFirewalld, nil
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active --quiet ufw", true); err == nil {
		return ToolUFW, nil
	}
	if _, err := runner.LookPath(ctx.GoContext(), conn, "iptables"); err == nil {
		return ToolIptables, nil
	}
	return ToolUnknown, fmt.Errorf("no supported firewall management tool (firewalld, ufw, iptables) found")
}

func (s *ManageFirewallRuleStep) getFirewallCmds(toolType FirewallToolType) (checkCmd, runCmd, rollbackCmd string, err error) {
	isAdding := s.State == FirewallRulePresent

	switch toolType {
	case ToolFirewalld:
		return s.getFirewalldCmds(isAdding)
	case ToolUFW:
		return s.getUfwCmds(isAdding)
	case ToolIptables:
		return s.getIptablesCmds(isAdding)
	default:
		return "", "", "", fmt.Errorf("unsupported firewall tool type: %s", toolType)
	}
}

func (s *ManageFirewallRuleStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector: %w", err)
	}

	tool, err := s.detectFirewallTool(ctx)
	if err != nil {
		if s.State == FirewallRuleAbsent {
			logger.Warnf("No firewall tool found, considering rule as absent. %v", err)
			return true, nil
		}
		return false, err
	}
	logger.Infof("Detected active firewall tool: %s", tool)

	checkCmd, _, _, err := s.getFirewallCmds(tool)
	if err != nil {
		return false, err
	}

	_, checkErr := runnerSvc.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	ruleExists := checkErr == nil

	if s.State == FirewallRulePresent && ruleExists {
		logger.Info("Firewall rule already exists. Step considered done.")
		return true, nil
	}
	if s.State == FirewallRuleAbsent && !ruleExists {
		logger.Info("Firewall rule is already absent. Step considered done.")
		return true, nil
	}

	return false, nil
}

func (s *ManageFirewallRuleStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector: %w", err)
	}

	tool, err := s.detectFirewallTool(ctx)
	if err != nil {
		if s.State == FirewallRuleAbsent {
			logger.Warnf("No firewall tool found, rule is considered absent by default. %v", err)
			return nil
		}
		return err
	}

	_, runCmd, _, err := s.getFirewallCmds(tool)
	if err != nil {
		return err
	}

	logger.Infof("Executing firewall command with %s: %s", tool, runCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, runCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to manage firewall rule with %s: %w", tool, err)
	}
	logger.Infof("Firewall rule successfully set to state '%s'.", s.State)
	return nil
}

func (s *ManageFirewallRuleStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector: %w", err)
	}

	tool, err := s.detectFirewallTool(ctx)
	if err != nil {
		logger.Warnf("No firewall tool found, cannot perform rollback. %v", err)
		return nil
	}

	_, _, rollbackCmd, err := s.getFirewallCmds(tool)
	if err != nil {
		return err
	}

	logger.Warnf("Attempting to rollback firewall rule by executing: %s", rollbackCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, rollbackCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to rollback firewall rule (best effort): %v", err)
	}
	return nil
}

func (s *ManageFirewallRuleStep) getFirewalldCmds(isAdding bool) (string, string, string, error) {
	var args []string
	if s.Rule.Port != "" {
		args = append(args, fmt.Sprintf("--port=%s", s.Rule.Port))
	} else if s.Rule.Service != "" {
		args = append(args, fmt.Sprintf("--service=%s", s.Rule.Service))
	} else {
		return "", "", "", fmt.Errorf("firewalld rule requires Port or Service")
	}

	var addAction, removeAction string
	if s.Rule.Source != "" {
		addAction = fmt.Sprintf("--add-rich-rule='rule family=ipv4 source address=%s %s accept'", s.Rule.Source, args[0])
		removeAction = fmt.Sprintf("--remove-rich-rule='rule family=ipv4 source address=%s %s accept'", s.Rule.Source, args[0])
	} else {
		addAction = fmt.Sprintf("--add-%s", strings.TrimPrefix(args[0], "--"))
		removeAction = fmt.Sprintf("--remove-%s", strings.TrimPrefix(args[0], "--"))
	}

	var runAction, rollbackAction string
	if isAdding {
		runAction, rollbackAction = addAction, removeAction
	} else {
		runAction, rollbackAction = removeAction, addAction
	}

	checkCmd := fmt.Sprintf("firewall-cmd --query-%s", strings.TrimPrefix(args[0], "--"))
	runCmd := fmt.Sprintf("firewall-cmd --permanent %s && firewall-cmd --reload", runAction)
	rollbackCmd := fmt.Sprintf("firewall-cmd --permanent %s && firewall-cmd --reload", rollbackAction)
	return checkCmd, runCmd, rollbackCmd, nil
}

func (s *ManageFirewallRuleStep) getUfwCmds(isAdding bool) (string, string, string, error) {
	var ruleParts []string
	action := "allow"
	if s.Rule.Action == "deny" {
		action = "deny"
	}
	ruleParts = append(ruleParts, action)

	if s.Rule.Source != "" {
		ruleParts = append(ruleParts, "from", s.Rule.Source)
	}
	if s.Rule.Port != "" {
		proto := "any"
		port := s.Rule.Port
		if strings.Contains(s.Rule.Port, "/") {
			parts := strings.Split(s.Rule.Port, "/")
			port = parts[0]
			proto = parts[1]
		}
		ruleParts = append(ruleParts, "to", "any", "port", port, "proto", proto)
	} else if s.Rule.Service != "" {
		ruleParts = append(ruleParts, s.Rule.Service)
	} else {
		return "", "", "", fmt.Errorf("ufw rule requires Port or Service")
	}

	ruleStr := strings.Join(ruleParts, " ")

	checkCmd := fmt.Sprintf("ufw status | grep -qE '%s'", ufwRuleToRegex(ruleParts))

	var runCmd, rollbackCmd string
	if isAdding {
		runCmd = fmt.Sprintf("ufw %s", ruleStr)
		rollbackCmd = fmt.Sprintf("ufw delete %s", ruleStr)
	} else {
		runCmd = fmt.Sprintf("ufw delete %s", ruleStr)
		rollbackCmd = fmt.Sprintf("ufw %s", ruleStr)
	}

	return checkCmd, runCmd, rollbackCmd, nil
}

func ufwRuleToRegex(parts []string) string {
	portRegex := ""
	actionRegex := "ALLOW"
	sourceRegex := "Anywhere"

	for i, p := range parts {
		switch p {
		case "allow":
			actionRegex = "ALLOW"
		case "deny":
			actionRegex = "DENY"
		case "port":
			if i+1 < len(parts) {
				portRegex = parts[i+1]
				if i+3 < len(parts) && parts[i+2] == "proto" {
					portRegex += "/" + strings.ToUpper(parts[i+3])
				}
			}
		case "from":
			if i+1 < len(parts) {
				sourceRegex = parts[i+1]
			}
		}
	}
	return fmt.Sprintf("%s\\s+%s\\s+%s", regexp.QuoteMeta(portRegex), actionRegex, regexp.QuoteMeta(sourceRegex))
}

func (s *ManageFirewallRuleStep) getIptablesCmds(isAdding bool) (string, string, string, error) {
	var args []string
	if s.Rule.Port != "" {
		parts := strings.Split(s.Rule.Port, "/")
		proto := "tcp"
		if len(parts) > 1 {
			proto = parts[1]
		}
		args = append(args, "-p", proto, "--dport", parts[0])
	} else {
		return "", "", "", fmt.Errorf("iptables rule requires Port")
	}
	if s.Rule.Source != "" {
		args = append(args, "-s", s.Rule.Source)
	}
	action := "ACCEPT"
	if s.Rule.Action == "deny" {
		action = "DROP"
	}
	args = append(args, "-j", action)
	argStr := strings.Join(args, " ")

	checkCmd := fmt.Sprintf("iptables -C INPUT %s", argStr)
	var runCmd, rollbackCmd string
	if isAdding {
		runCmd = fmt.Sprintf("iptables -I INPUT 1 %s", argStr)
		rollbackCmd = fmt.Sprintf("iptables -D INPUT %s", argStr)
	} else {
		runCmd = fmt.Sprintf("iptables -D INPUT %s", argStr)
		rollbackCmd = fmt.Sprintf("iptables -I INPUT 1 %s", argStr)
	}

	warning := " && echo 'Warning: iptables rule is not persistent across reboots by default.'"
	runCmd += warning
	rollbackCmd += warning

	return checkCmd, runCmd, rollbackCmd, nil
}

var _ step.Step = (*ManageFirewallRuleStep)(nil)
