package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// SetIPTablesAlternativesStep sets the iptables alternatives mode (e.g., "legacy", "nft").
type SetIPTablesAlternativesStep struct {
	meta spec.StepMeta
	Mode string // "legacy" or "nft"
	Sudo bool
}

// NewSetIPTablesAlternativesStep creates a new SetIPTablesAlternativesStep.
func NewSetIPTablesAlternativesStep(instanceName, mode string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("SetIPTablesAlternatives-%s", mode)
	}
	return &SetIPTablesAlternativesStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Sets iptables alternatives mode to %s.", mode),
		},
		Mode: strings.ToLower(mode),
		Sudo: sudo,
	}
}

func (s *SetIPTablesAlternativesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *SetIPTablesAlternativesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, err
	}

	// Check current alternatives for iptables
	// update-alternatives --display iptables | grep 'link currently points to' | awk '{print $NF}'
	// This command is specific to update-alternatives systems (Debian/Ubuntu).
	// For RHEL/CentOS, a different mechanism or check might be needed if alternatives are managed differently.
	// Assuming update-alternatives is the target mechanism for now.

	checkCmd := "update-alternatives --display iptables | grep 'link currently points to' | awk '{print $NF}'"
	stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd, false) // Sudo usually not needed for display
	if err != nil {
		// If update-alternatives is not found, or iptables is not managed by it,
		// this check might fail. Assume step needs to run to try and set it.
		logger.Warn("Could not determine current iptables alternative setting. Step will run.", "error", err)
		return false, nil
	}

	currentPath := strings.TrimSpace(string(stdout))
	// Expected paths: /usr/sbin/iptables-legacy or /usr/sbin/iptables-nft
	expectedPathSuffix := fmt.Sprintf("iptables-%s", s.Mode)

	if strings.HasSuffix(currentPath, expectedPathSuffix) {
		logger.Info("IPTables alternatives already set to desired mode.", "mode", s.Mode, "current_path", currentPath)
		return true, nil
	}

	logger.Info("IPTables alternatives not in desired mode.", "current_path", currentPath, "desired_suffix", expectedPathSuffix)
	return false, nil
}

func (s *SetIPTablesAlternativesStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return err
	}

	var targetPath string
	if s.Mode == "legacy" {
		targetPath = "/usr/sbin/iptables-legacy"
	} else if s.Mode == "nft" {
		targetPath = "/usr/sbin/iptables-nft"
	} else {
		return fmt.Errorf("unsupported iptables mode: %s. Must be 'legacy' or 'nft'", s.Mode)
	}

	// Check if the target path exists before trying to set it
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return fmt.Errorf("failed to check for existence of %s: %w", targetPath, err)
	}
	if !exists {
		return fmt.Errorf("target iptables binary %s does not exist on host %s. Cannot set alternative.", targetPath, host.GetName())
	}

	// Command: update-alternatives --set iptables /usr/sbin/iptables-legacy (or -nft)
	cmd := fmt.Sprintf("update-alternatives --set iptables %s", targetPath)
	logger.Info("Setting iptables alternatives.", "command", cmd)
	_, stderr, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute '%s': %w. Stderr: %s", cmd, err, string(stderr))
	}

	// Also set for ip6tables, arptables, ebtables if they exist and follow same pattern
	for _, tbl := range []string{"ip6tables", "arptables", "ebtables"} {
		targetTblPath := strings.Replace(targetPath, "iptables", tbl, 1)
		// Check if this alternative exists (e.g. /usr/sbin/ip6tables-legacy)
		if altExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, targetTblPath); altExists {
			tblCmd := fmt.Sprintf("update-alternatives --set %s %s", tbl, targetTblPath)
			logger.Info("Setting alternatives for related table.", "command", tblCmd)
			if _, stderrTbl, errTbl := runnerSvc.Run(ctx.GoContext(), conn, tblCmd, s.Sudo); errTbl != nil {
				logger.Warn("Failed to set alternatives for table. This might be okay if the table/alternative doesn't exist.", "table", tbl, "error", errTbl, "stderr", string(stderrTbl))
			}
		} else {
			logger.Debug("Skipping alternatives for table as target path does not exist.", "table", tbl, "path_checked", targetTblPath)
		}
	}


	logger.Info("IPTables alternatives set successfully.", "mode", s.Mode)
	return nil
}

func (s *SetIPTablesAlternativesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// Rollback would require knowing the previous alternatives setting.
	// This is complex and often not done for this type of system-wide change.
	ctx.GetLogger().Warn("Rollback for SetIPTablesAlternativesStep is not implemented.", "step", s.meta.Name)
	return nil
}

var _ step.Step = (*SetIPTablesAlternativesStep)(nil)
```
