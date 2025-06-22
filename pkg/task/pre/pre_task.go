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

	// Get all unique remote hosts from relevant roles to run checks on them.
	allRemoteHostsMap := make(map[string]connector.Host)

	// Define roles that should typically undergo pre-flight checks.
	// These should align with roles defined in your common constants or config.
	rolesToCheck := []string{
		common.RoleEtcd,
		common.RoleMaster,
		common.RoleWorker,
		// Add other roles if they exist and need checks, e.g. common.RoleRegistry, common.RoleLoadBalancer
	}
	// Ensure common.RoleRegistry and common.RoleLoadBalancer exist or adjust list.
	// For now, assuming the primary three.
	// If RoleRegistry and RoleLoadBalancer are defined in common, you can add them:
	// rolesToCheck = append(rolesToCheck, common.RoleRegistry, common.RoleLoadBalancer)


	for _, role := range rolesToCheck {
		hostsInRole, err := ctx.GetHostsByRole(role)
		if err != nil {
			// Log or handle error, e.g., if a role is defined but no hosts assigned.
			logger.Warn("Could not get hosts for role, skipping for pre-flight checks.", "role", role, "error", err)
			continue
		}
		for _, h := range hostsInRole {
			// Pre-flight checks are typically for all nodes part of the cluster,
			// excluding the control node itself if it's not part of the cluster roles.
			// The GetHostsByRole should ideally return only actual cluster members.
			allRemoteHostsMap[h.GetName()] = h
		}
	}

	var allRemoteHosts []connector.Host
	for _, h := range allRemoteHostsMap {
		allRemoteHosts = append(allRemoteHosts, h)
	}

	if len(allRemoteHosts) == 0 {
		logger.Info("No remote target hosts found to run pre-flight checks on.")
		// The task might still create local checks for the controlNode or an empty report.
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
		Dependencies: plan.UniqueNodeIDs(checkNodeIDs), // Depends on all checks
	}

	if len(checkNodeIDs) > 0 {
		fragment.EntryNodes = plan.UniqueNodeIDs(checkNodeIDs) // Use plan.UniqueNodeIDs
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
