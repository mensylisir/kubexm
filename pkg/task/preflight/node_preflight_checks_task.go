package preflight

import (
	"fmt"

	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // No longer needed directly
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/os" // Using steps from pkg/step/os
	"github.com/mensylisir/kubexm/pkg/task"
)

// InitialNodeSetupTask performs essential OS-level configurations on target nodes.
type InitialNodeSetupTask struct {
	task.BaseTask
}

// NewInitialNodeSetupTask creates a new InitialNodeSetupTask.
func NewInitialNodeSetupTask() task.Task {
	return &InitialNodeSetupTask{
		BaseTask: task.BaseTask{
			TaskName: "InitialNodeOSSetup",
			TaskDesc: "Performs essential OS configurations like disabling firewall, swap, and SELinux.",
			// RunOnRoles can be set by the module if specific roles need this, or empty for all.
		},
	}
}

// Name, Description, IsRequired can use BaseTask's defaults or be overridden if needed.

// Plan generates the execution fragment for initial node OS setup.
func (t *InitialNodeSetupTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name() + "-Fragment")

	allHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...) // Use roles from BaseTask or "" for all
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}

	if len(allHosts) == 0 {
		logger.Info("No target hosts found for initial node setup.")
		return task.NewEmptyFragment(), nil
	}

	clusterCfg := ctx.GetClusterConfig()
	var currentEntryNodes []plan.NodeID // All steps in this task can run in parallel

	// 1. Disable Firewall (configurable via clusterCfg.Spec.Preflight or a new SystemSetup section)
	// For now, assume it's generally desired. Sudo typically true.
	// TODO: Make 'disableFirewall' configurable from clusterCfg
	disableFirewallEnabled := true // Default or from config
	if disableFirewallEnabled {
		firewallStepName := fmt.Sprintf("%s-DisableFirewall", t.Name())
		// Using os.NewDisableFirewallStep(instanceName string, sudo bool, targetFirewalls []string)
		firewallStep := os.NewDisableFirewallStep(firewallStepName, true, nil) // Targets default firewalls
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  firewallStepName, Step: firewallStep, Hosts: allHosts, StepName: firewallStep.Meta().Name,
		})
		currentEntryNodes = append(currentEntryNodes, nodeID)
	}

	// 2. Disable SELinux (configurable)
	// TODO: Make 'disableSelinux' configurable from clusterCfg
	disableSelinuxEnabled := true // Default or from config
	if disableSelinuxEnabled {
		selinuxStepName := fmt.Sprintf("%s-DisableSELinux", t.Name())
		// Using os.NewDisableSelinuxStep(instanceName string, sudo bool)
		selinuxStep := os.NewDisableSelinuxStep(selinuxStepName, true)
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name: selinuxStepName, Step: selinuxStep, Hosts: allHosts, StepName: selinuxStep.Meta().Name,
		})
		currentEntryNodes = append(currentEntryNodes, nodeID)
	}

	// 3. Disable Swap (configurable via clusterCfg.Spec.Preflight.DisableSwap)
	disableSwapEnabled := true // Default
	if clusterCfg != nil && clusterCfg.Spec.Preflight != nil {
		disableSwapEnabled = clusterCfg.Spec.Preflight.DisableSwap
	}
	if disableSwapEnabled {
		swapStepName := fmt.Sprintf("%s-DisableSwap", t.Name())
		// Using os.NewDisableSwapStep(instanceName string, sudo bool)
		swapStep := os.NewDisableSwapStep(swapStepName, true)
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name: swapStepName, Step: swapStep, Hosts: allHosts, StepName: swapStep.Meta().Name,
		})
		currentEntryNodes = append(currentEntryNodes, nodeID)
	}

	if len(fragment.Nodes) == 0 {
		logger.Info("No initial node setup actions were planned.")
		return task.NewEmptyFragment(), nil
	}

	fragment.EntryNodes = task.UniqueNodeIDs(currentEntryNodes)
	fragment.ExitNodes = task.UniqueNodeIDs(currentEntryNodes) // All are parallel and can be exits

	logger.Info("Initial node OS setup task planned.")
	return fragment, nil
}

var _ task.Task = (*InitialNodeSetupTask)(nil)
