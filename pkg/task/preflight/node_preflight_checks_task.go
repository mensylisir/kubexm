package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For role constants
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/os" // For DisableFirewallStep
	"github.com/mensylisir/kubexm/pkg/task"
	// Assuming steppreflight for other preflight steps like DisableSwapStep
	steppreflight "github.com/mensylisir/kubexm/pkg/step/preflight"
)

// NodePreflightChecksTask performs various preflight checks and configurations on target nodes.
type NodePreflightChecksTask struct {
	task.BaseTask
}

// NewNodePreflightChecksTask creates a new NodePreflightChecksTask.
func NewNodePreflightChecksTask() task.Task {
	return &NodePreflightChecksTask{
		BaseTask: task.BaseTask{
			TaskName: "NodePreflightChecksAndConfig",
			TaskDesc: "Performs OS-level preflight checks and configurations like disabling firewall, swap, SELinux, etc.",
		},
	}
}

func (t *NodePreflightChecksTask) Name() string {
	return t.BaseTask.TaskName
}

func (t *NodePreflightChecksTask) Description() string {
	return t.BaseTask.TaskDesc
}

// IsRequired is true by default for preflight checks, could be made configurable.
func (t *NodePreflightChecksTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Could check a global Preflight.Enabled flag in ClusterConfig if available.
	return true, nil
}

// Plan generates the execution fragment for node preflight checks.
func (t *NodePreflightChecksTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment()

	// These checks typically run on all nodes (masters and workers).
	// Tasks can define target roles, or modules can pass specific host lists.
	// For now, assume this task gets all hosts from the context unless roles are specified in BaseTask.
	// If BaseTask.RunOnRoles is empty, GetHostsByRole should handle it (e.g. return all or error).
	// Let's target all roles for these universal preflights.
	// A more sophisticated approach might get roles from config or have separate tasks per role group if checks differ.
	allHosts, err := ctx.GetHostsByRole("") // Get all configured hosts
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts for NodePreflightChecksTask: %w", err)
	}

	if len(allHosts) == 0 {
		logger.Info("No target hosts found for node preflight checks.")
		return task.NewEmptyFragment(), nil
	}

	clusterCfg := ctx.GetClusterConfig()
	// Default settings for preflight actions, can be overridden by clusterCfg.Spec.Preflight
	disableFirewall := true
	disableSwap := true
	// selinuxPermissive := true
	// loadKernelModules := []string{"overlay", "br_netfilter"}
	// sysctlParams := map[string]string{"net.bridge.bridge-nf-call-iptables": "1", "net.ipv4.ip_forward": "1"}

	if clusterCfg != nil && clusterCfg.Spec.Preflight != nil {
		// Example: allow overriding these actions from config
		// if clusterCfg.Spec.Preflight.DisableFirewall != nil {
		// 	disableFirewall = *clusterCfg.Spec.Preflight.DisableFirewall
		// }
		disableSwap = clusterCfg.Spec.Preflight.DisableSwap // Already uses this
	}


	var currentEntryNodes []plan.NodeID
	var currentExitNodes []plan.NodeID


	// 1. Disable Firewall
	if disableFirewall {
		firewallStepName := "DisableFirewallOnNodes"
		// Assuming DisableFirewallStep defaults to common firewalls like firewalld, ufw
		// Sudo is typically required for firewall operations.
		firewallStep := os.NewDisableFirewallStep(firewallStepName, true, nil)
		firewallNodeID := plan.NodeID(firewallStepName + "-Global") // Make ID unique

		fragment.Nodes[firewallNodeID] = &plan.ExecutionNode{
			Name:         firewallStepName,
			Step:         firewallStep,
			Hosts:        allHosts, // Run on all target hosts
			StepName:     firewallStep.Meta().Name,
			Dependencies: []plan.NodeID{}, // No dependencies for this first group of checks
		}
		currentEntryNodes = append(currentEntryNodes, firewallNodeID)
		currentExitNodes = append(currentExitNodes, firewallNodeID) // This node is also an exit until more are added
	}

	// 2. Disable Swap (using existing preflight step)
	if disableSwap {
		swapStepName := "DisableSwapOnNodes"
		// Sudo is true for DisableSwapStep by default in its constructor if not specified,
		// or can be passed: steppreflight.NewDisableSwapStep(swapStepName, true)
		swapStep := steppreflight.NewDisableSwapStep(swapStepName, true)
		swapNodeID := plan.NodeID(swapStepName + "-Global")

		fragment.Nodes[swapNodeID] = &plan.ExecutionNode{
			Name:         swapStepName,
			Step:         swapStep,
			Hosts:        allHosts,
			StepName:     swapStep.Meta().Name,
			Dependencies: []plan.NodeID{}, // Runs in parallel with firewall check
		}
		currentEntryNodes = append(currentEntryNodes, swapNodeID)
		currentExitNodes = append(currentExitNodes, swapNodeID)
	}

	// TODO: Add other preflight checks as nodes:
	// - Set SELinux to permissive
	// - Load Kernel Modules (overlay, br_netfilter)
	// - Configure Sysctl parameters (bridge-nf-call-iptables, ip_forward)
	// Each of these would be a new step type and a new node in the fragment.
	// They can mostly run in parallel, so they would all be entry nodes initially.

	fragment.EntryNodes = task.UniqueNodeIDs(currentEntryNodes)
	fragment.ExitNodes = task.UniqueNodeIDs(currentExitNodes) // All these parallel checks are also exits

	logger.Info("Node preflight checks and configurations task planned.")
	return fragment, nil
}

var _ task.Task = (*NodePreflightChecksTask)(nil)
