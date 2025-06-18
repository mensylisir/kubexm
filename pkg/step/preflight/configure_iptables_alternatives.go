package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// IPTablesAlternativeConfig defines a tool and its desired legacy path for update-alternatives.
type IPTablesAlternativeConfig struct {
	ToolName   string `json:"toolName"`
	LegacyPath string `json:"legacyPath"`
}

// SetIPTablesAlternativesStepSpec defines the specification for setting iptables alternatives.
type SetIPTablesAlternativesStepSpec struct {
	Alternatives []IPTablesAlternativeConfig `json:"alternatives,omitempty"`
}

// GetName returns the name of the step.
func (s *SetIPTablesAlternativesStepSpec) GetName() string {
	return "Set IPTables Alternatives to Legacy"
}

// PopulateDefaults fills the spec with default values.
func (s *SetIPTablesAlternativesStepSpec) PopulateDefaults() {
	if len(s.Alternatives) == 0 {
		s.Alternatives = []IPTablesAlternativeConfig{
			{ToolName: "iptables", LegacyPath: "/usr/sbin/iptables-legacy"},
			{ToolName: "ip6tables", LegacyPath: "/usr/sbin/ip6tables-legacy"},
			{ToolName: "arptables", LegacyPath: "/usr/sbin/arptables-legacy"},
			{ToolName: "ebtables", LegacyPath: "/usr/sbin/ebtables-legacy"},
		}
	}
}

// SetIPTablesAlternativesStepExecutor implements the logic for setting iptables alternatives.
type SetIPTablesAlternativesStepExecutor struct{}

// Check determines if the iptables alternatives are already set to their legacy paths.
func (e *SetIPTablesAlternativesStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	spec, ok := s.(*SetIPTablesAlternativesStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T", s)
	}
	spec.PopulateDefaults()
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	for _, alt := range spec.Alternatives {
		// update-alternatives --display <tool>
		// Output format:
		// <tool> - auto mode
		//   link currently points to /usr/sbin/<tool>-nft (or -legacy)
		//   link best version is /usr/sbin/<tool>-nft
		//   link /usr/bin/<tool> -> /etc/alternatives/<tool>
		//   slave ...
		// /usr/sbin/<tool>-legacy - priority 10
		// /usr/sbin/<tool>-nft - priority 20
		cmd := fmt.Sprintf("update-alternatives --display %s", alt.ToolName)
		// Sudo not typically needed for --display. AllowFailure as tool might not be managed.
		stdout, _, errDisplay := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmd, &connector.ExecOptions{Sudo: false, AllowFailure: true, TrimOutput: true})

		if errDisplay != nil {
			// If `update-alternatives --display` fails, it might mean the tool is not managed by alternatives
			// or `update-alternatives` is not installed. Script uses `|| true`, so we treat as "not applicable" or "done for this tool".
			hostCtxLogger.Warnf("`update-alternatives --display %s` failed or tool not managed: %v. Assuming no action needed or possible for this tool.", alt.ToolName, errDisplay)
			continue // Move to the next alternative
		}

		currentLink := ""
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			// A common pattern for current link: "link currently points to <path>" or "current link is <path>"
			// Or simply the first line after "auto mode" or "manual mode" might show current best.
			// Let's look for "link currently points to"
			if strings.Contains(trimmedLine, "link currently points to") {
				parts := strings.Split(trimmedLine, "link currently points to")
				if len(parts) > 1 {
					currentLink = strings.TrimSpace(parts[1])
					break
				}
			}
			// Another pattern for some versions: "<tool_path> - family <tool_name> priority <num> status <current_selection_state>"
			// This is harder to parse reliably. The "link currently points to" is more standard.
			// If not found, try to infer from other lines, e.g. if only one alternative is listed and it matches.
		}

		// If we couldn't parse the current link, log it and assume not done to be safe.
		if currentLink == "" {
			hostCtxLogger.Warnf("Could not determine current alternative for %s from display output. Output:\n%s\nAssuming not configured correctly.", alt.ToolName, stdout)
			return false, nil
		}


		if currentLink != alt.LegacyPath {
			hostCtxLogger.Infof("IPTables alternative for %s is '%s', expected '%s'.", alt.ToolName, currentLink, alt.LegacyPath)
			return false, nil
		}
		hostCtxLogger.Debugf("IPTables alternative for %s is correctly set to '%s'.", alt.ToolName, alt.LegacyPath)
	}

	hostCtxLogger.Infof("All configured iptables alternatives are correctly set to their legacy paths or are not managed by update-alternatives.")
	return true, nil
}

// Execute sets the iptables alternatives to their legacy paths.
func (e *SetIPTablesAlternativesStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	spec, ok := s.(*SetIPTablesAlternativesStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T", s)
		stepName := "SetIPTablesAlternatives (type error)"; if s != nil { stepName = s.GetName() }
		return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	}
	spec.PopulateDefaults()

	stepName := spec.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	var warnings []string

	for _, alt := range spec.Alternatives {
		// Check if the legacy path actually exists before trying to set it.
		// `update-alternatives --set` might fail if the path is invalid.
		// Sudo not needed for stat/ls. AllowFailure true.
		_, _, errStat := ctx.Host.Runner.RunWithOptions(ctx.GoContext, fmt.Sprintf("stat %s", alt.LegacyPath), &connector.ExecOptions{Sudo: false, AllowFailure: true})
		if errStat != nil {
			msg := fmt.Sprintf("Legacy path %s for tool %s does not exist or is not accessible. Skipping --set for this alternative.", alt.LegacyPath, alt.ToolName)
			hostCtxLogger.Warnf(msg)
			warnings = append(warnings, msg)
			continue
		}

		cmd := fmt.Sprintf("update-alternatives --set %s %s", alt.ToolName, alt.LegacyPath)
		hostCtxLogger.Infof("Attempting to set iptables alternative: %s", cmd)
		// Sudo is required for update-alternatives --set
		// AllowFailure=true mimics `|| true` from script.
		_, stderr, errSet := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmd, &connector.ExecOptions{Sudo: true, AllowFailure: true})
		if errSet != nil {
			// This is a warning, not a fatal error for the step.
			msg := fmt.Sprintf("Command '%s' failed or had non-zero exit (as script uses '|| true', this is a warning): %v. Stderr: %s", cmd, errSet, stderr)
			hostCtxLogger.Warnf(msg)
			warnings = append(warnings, msg)
		} else {
			hostCtxLogger.Infof("Command '%s' executed successfully or handled by AllowFailure.", cmd)
		}
	}

	// Perform a post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		// This check error is more serious as it's about verification.
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed verification: %v", res.Error)
		return res
	}
	if !done {
		errMsg := "post-execution check indicates iptables alternatives are not correctly set."
		// Add collected warnings to the message if any.
		if len(warnings) > 0 {
			errMsg += " Previous warnings: " + strings.Join(warnings, "; ")
		}
		res.Error = fmt.Errorf(errMsg)
		res.SetFailed(errMsg) // Mark as failed if check doesn't pass.
		hostCtxLogger.Errorf("Step failed verification: %s", errMsg)
		return res
	}

	if len(warnings) > 0 {
		res.SetSucceededWithWarnings("IPTables alternatives set, but with warnings: " + strings.Join(warnings, "; "))
		hostCtxLogger.Warnf("Step finished with warnings: %s", res.Message)
	} else {
		res.SetSucceeded("IPTables alternatives set successfully to legacy versions where applicable.")
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	}
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&SetIPTablesAlternativesStepSpec{}), &SetIPTablesAlternativesStepExecutor{})
}
