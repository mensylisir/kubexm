package pre

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// PreTask performs various pre-flight checks on target nodes.
type PreTask struct {
	task.BaseTask
	// Configuration for checks can be added here if needed,
	// e.g., expected OS versions, minimum resource requirements.
}

// NewPreTask creates a new PreTask.
func NewPreTask() task.Task {
	return &PreTask{
		BaseTask: task.BaseTask{
			TaskName: "PreFlightChecks",
			TaskDesc: "Performs pre-flight checks on all relevant cluster nodes.",
		},
	}
}

// IsRequired for PreTask is typically always true.
func (t *PreTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

// Plan generates the execution fragment for pre-flight checks.
func (t *PreTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment()

	controlHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for %s: %w", t.Name(), err)
	}

	// Get all unique hosts from all roles to run checks on them.
	// This avoids running the same check multiple times on a host if it has multiple roles.
	allTargetHostsMap := make(map[string]connector.Host)
	rolesToFetch := []string{"etcd", "master", "worker", "registry", "loadbalancer"} // Add all relevant roles

	if ctx.GetClusterConfig() != nil && ctx.GetClusterConfig().Spec.RoleGroups != nil {
		for roleName := range ctx.GetClusterConfig().Spec.RoleGroups {
			// This is a simplification. RoleGroups is map[string][]string.
			// We need to iterate through the actual node names listed in those roles.
			// For now, using a placeholder for fetching all hosts.
			// A more robust way is to iterate Spec.Hosts and check their roles.
			// Or iterate Spec.RoleGroups and then Spec.Hosts to match names.
		}
	}

	// Fallback: Get all hosts if role groups are complex to parse here directly
	// A better way is to have a TaskContext method like ctx.GetAllTargetHosts()
	if len(allTargetHostsMap) == 0 && ctx.GetClusterConfig() != nil {
		for _, hostSpec := range ctx.GetClusterConfig().Spec.Hosts {
			// This is creating new host objects. We should use what's in HostRuntimes.
			// host := connector.NewHostFromSpec(*hostSpec) // This creates a new instance.
			// We need to fetch the existing connector.Host from runtime.
			// For now, let's assume TaskContext can give us all unique target hosts.
			// This part needs a robust way to get all *connector.Host* objects
			// that are part of the cluster definition, excluding the control node if checks are remote.
		}
	}
	// Placeholder: for now, we'll assume a method on context or manually get them.
	// This part is crucial and needs to correctly identify all remote hosts.
	// For simplicity in this step, let's assume we get a list of all hosts from HostRuntimes
	// excluding the control node.

	var allRemoteHosts []connector.Host
	for _, hr := range ctx.GetClusterConfig().Spec.Hosts { // Iterating specs, then need to find matching connector.Host
		// This is still not ideal. The TaskContext should provide a way to get all configured *connector.Host* instances.
		// Let's assume such a method exists or is added to TaskContext later.
		// For now, we'll try to get them by iterating HostRuntimes.
		// This will include the control node if it was added to HostRuntimes, so we might need to filter it out
		// if checks are meant to be only on "remote" cluster nodes.
		// However, some preflight checks (like local binary availability) might run on control node.
	}

	for hostName, hostRuntime := range ctx.HostRuntimes() { // Assuming HostRuntimes() gives map[string]*HostRuntime
		if hostRuntime.Host.GetName() != common.ControlNodeHostName {
			allRemoteHosts = append(allRemoteHosts, hostRuntime.Host)
		}
	}
	if len(allRemoteHosts) == 0 {
		logger.Info("No remote hosts found to run pre-flight checks on (other than control node).")
		// Still might proceed with local checks or a report.
	}

	// --- Define Check Commands ---
	// Example checks (more can be added from 20-kubernetes流程设计.md)
	// These commands are illustrative. Real checks might be more complex.
	checks := []struct {
		Name       string
		Command    string
		Sudo       bool
		RunOnAll   bool // True if runs on allRemoteHosts, false if specific (e.g. only masters)
		TargetHosts []connector.Host // If not RunOnAll, specify target hosts. If nil with RunOnAll=false, runs on control node.
	}{
		// System Info Checks (run on all remote nodes)
		{"CheckHostname", "hostname", false, true, nil},
		{"CheckOSRelease", "cat /etc/os-release", false, true, nil},
		{"CheckKernelVersion", "uname -r", false, true, nil},
		{"CheckTotalMemory", "free -m | awk '/^Mem:/{print $2}'", false, true, nil},
		{"CheckCPUInfo", "lscpu | grep '^CPU(s):'", false, true, nil},
		// Configuration Checks (run on all remote nodes)
		{"CheckFirewallDisabled", "systemctl is-active firewalld || echo 'inactive or not found (OK)'", false, true, nil}, // Command needs to succeed if inactive
		{"CheckSELinuxDisabled", "sestatus | grep -i 'SELinux status:' | grep -i 'disabled'", false, true, nil},           // Will fail if not disabled
		{"CheckSwapDisabled", "swapon --summary | grep -v '^FILENAME' || echo 'swap not configured (OK)'", false, true, nil}, // Succeeds if no output or only header
		// Kernel Modules (run on all remote nodes)
		{"CheckOverlayModule", "lsmod | grep overlay", false, true, nil},
		{"CheckBrNetfilterModule", "lsmod | grep br_netfilter", false, true, nil},
		// Sysctl Params (run on all remote nodes)
		{"CheckIPv4Forward", "sysctl net.ipv4.ip_forward | grep 'net.ipv4.ip_forward = 1'", false, true, nil},
		{"CheckBridgeNFCallIPTables", "sysctl net.bridge.bridge-nf-call-iptables | grep 'net.bridge.bridge-nf-call-iptables = 1'", false, true, nil},
	}

	var checkNodeIDs []plan.NodeID

	for _, check := range checks {
		nodeID := plan.NodeID(fmt.Sprintf("preflight-%s-check", strings.ToLower(strings.ReplaceAll(check.Name, " ", "-"))))

		hostsForThisCheck := controlHost // Default to controlHost if TargetHosts is nil and not RunOnAll
		if check.RunOnAll {
			if len(allRemoteHosts) > 0 {
				hostsForThisCheck = allRemoteHosts
			} else {
				// If RunOnAll is true but no remote hosts, maybe this check is only for control node or skipped.
				// For now, let's assume if RunOnAll and no remote hosts, it runs on control node.
				// Or, simply don't create the node if allRemoteHosts is empty for a RunOnAll=true check.
				logger.Debug("Skipping remote check as no remote hosts found.", "check", check.Name)
				continue
			}
		} else if len(check.TargetHosts) > 0 {
			hostsForThisCheck = check.TargetHosts
		}


		// For PreTask, we mostly care if the command runs. Success/failure is logged.
		// The ReportTableStep will just list that checks were performed.
		// Setting ExpectedExitCode to 0 for simplicity, meaning check command itself should succeed.
		// More advanced: some checks might expect non-zero if a condition is NOT met.
		cmdStep := commonsteps.NewCommandStep(
			check.Name, // Instance name for the step
			check.Command,
			check.Sudo,
			true, // IgnoreError = true for pre-flight checks (we want all to run and report)
			0,    // Timeout (default)
			nil,  // Env
			0,    // ExpectedExitCode (0 for simple success)
			"", false, 0, // No CheckCmd for these checks
			"", false,    // No RollbackCmd
		)

		fragment.Nodes[nodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Preflight Check: %s", check.Name),
			Step:         cmdStep,
			Hosts:        hostsForThisCheck, // Ensure this is a slice
			StepName:     cmdStep.Meta().Name,
			Dependencies: []plan.NodeID{}, // Initially no dependencies among checks
		}
		checkNodeIDs = append(checkNodeIDs, nodeID)
	}

	// Report Step (runs on control node, depends on all check nodes)
	reportNodeID := plan.NodeID("preflight-summary-report")
	reportHeaders := []string{"Check Name", "Status"}
	reportRows := [][]string{}
	for _, check := range checks {
		// As per Option 1, status is just "Executed". Details in logs.
		reportRows = append(reportRows, []string{check.Name, "Executed (Details in Logs)"})
	}

	reportStep := commonsteps.NewReportTableStep(
		"PreFlightCheckSummary",
		reportHeaders,
		reportRows,
	)
	fragment.Nodes[reportNodeID] = &plan.ExecutionNode{
		Name:         "Pre-flight Check Summary Report",
		Step:         reportStep,
		Hosts:        []connector.Host{controlHost},
		StepName:     reportStep.Meta().Name,
		Dependencies: task.UniqueNodeIDs(checkNodeIDs), // Depends on all checks
	}

	if len(checkNodeIDs) > 0 {
		fragment.EntryNodes = task.UniqueNodeIDs(checkNodeIDs)
		fragment.ExitNodes = []plan.NodeID{reportNodeID}
	} else {
		// If no checks were planned (e.g., no remote hosts and no local checks defined)
		// The report step might be the only node.
		fragment.EntryNodes = []plan.NodeID{reportNodeID}
		fragment.ExitNodes = []plan.NodeID{reportNodeID}
	}

	// If there were absolutely no checks AND the report is empty, fragment could be empty.
    if len(fragment.Nodes) == 0 {
        logger.Info("No pre-flight checks or report nodes were planned.")
        return task.NewEmptyFragment(), nil
    }


	logger.Info("Pre-flight check task planned.", "num_check_nodes", len(checkNodeIDs), "num_total_nodes", len(fragment.Nodes))
	return fragment, nil
}

var _ task.Task = (*PreTask)(nil)
