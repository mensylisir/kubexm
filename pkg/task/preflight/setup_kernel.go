package preflight

import (
	"fmt" // For errors or unique node IDs

	"github.com/mensylisir/kubexm/pkg/plan"    // For plan.NodeID, plan.ExecutionNode
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.TaskContext
	"github.com/mensylisir/kubexm/pkg/task"    // For task.BaseTask, task.ExecutionFragment, task.Task interface

	// Assuming these New... functions return actual step.Step instances
	steppreflight "github.com/mensylisir/kubexm/pkg/step/preflight" // For preflight.NewLoadKernelModulesStep, preflight.NewSetSystemConfigStep
	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For accessing config types
)

// SetupKernelTask is responsible for configuring kernel parameters and loading modules.
type SetupKernelTask struct {
	task.BaseTask
	// Add any specific fields this task might need, if not covered by BaseTask
}

// NewSetupKernelTask creates a new SetupKernelTask.
func NewSetupKernelTask() task.Task { // Returns task.Task interface
	return &SetupKernelTask{
		BaseTask: task.NewBaseTask(
			"SetupKernel", // TaskName
			"Configures kernel parameters and loads necessary modules.", // TaskDesc
			[]string{}, // RunOnRoles - empty means all hosts, or roles determined in Plan
			nil,        // HostFilter - can be nil or a specific filter
			false,      // IgnoreError - kernel setup is usually critical
		),
	}
}

// IsRequired can be overridden if this task is conditional.
// func (t *SetupKernelTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
//    clusterConfig := ctx.GetClusterConfig()
//    // Example: return clusterConfig.Spec.Kernel.ManageKernel, nil
//	  return true, nil // Default
// }

// Plan generates the execution fragment for setting up the kernel.
func (t *SetupKernelTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	clusterConfig := ctx.GetClusterConfig() // Access config via context

	// Default values (similar to the original file)
	kernelModules := []string{"br_netfilter", "overlay", "ip_vs"}
	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
	}
	sysctlConfigPath := "/etc/sysctl.d/90-kubexms-kernel.conf" // Default path
	reloadSysctl := true

	// Override with values from config if provided
	// Assuming ClusterConfig.Spec.Kernel exists and has Modules and SysctlParams fields
	if clusterConfig.Spec.Kernel != nil {
		if len(clusterConfig.Spec.Kernel.Modules) > 0 {
			kernelModules = clusterConfig.Spec.Kernel.Modules
			logger.Debug("Overriding kernel modules from config", "modules", kernelModules)
		}
		if len(clusterConfig.Spec.Kernel.SysctlParams) > 0 {
			sysctlParams = clusterConfig.Spec.Kernel.SysctlParams
			logger.Debug("Overriding sysctl params from config", "params", sysctlParams)
		}
		// Example for SysctlConfigFilePath if it were part of v1alpha1.KernelConfig:
		// if clusterConfig.Spec.Kernel.SysctlConfigFilePath != "" {
		//    sysctlConfigPath = clusterConfig.Spec.Kernel.SysctlConfigFilePath
		//    logger.Debug("Overriding sysctl config file path from config", "path", sysctlConfigPath)
		// }
	}

	// Create Step instances
	// IMPORTANT: These New...Step functions must return objects implementing step.Step
	// And they should take necessary parameters directly, not "StepSpec" structs.
	loadModulesStepName := fmt.Sprintf("%s-LoadModules", t.Name())
	// Using os.NewLoadKernelModulesStep(instanceName string, modules []string, sudo bool, confFile string)
	// The confFile for modules is often /etc/modules-load.d/<name>.conf
	// Let LoadKernelModulesStep use its default config file path.
	loadModulesStep := os.NewLoadKernelModulesStep(loadModulesStepName, kernelModules, true, "") // sudo true, default conf file

	setSysctlStepName := fmt.Sprintf("%s-SetSysctl", t.Name())
	// Using os.NewSetSystemConfigStep(instanceName string, params map[string]string, configFilePath string, reload bool, sudo bool)
	setSysctlStep := os.NewSetSystemConfigStep(setSysctlStepName, sysctlParams, sysctlConfigPath, reloadSysctl, true) // sudo true

	// Define ExecutionNodes
	fragment := task.NewExecutionFragment(t.Name() + "-Fragment")

	allHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...) // Use roles from BaseTask
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}

	if len(allHosts) == 0 {
		logger.Info("No hosts targeted for this task based on roles, returning empty fragment.")
		return task.NewEmptyFragment(), nil
	}
	logger.Debug("Targeting hosts for kernel setup", "count", len(allHosts))

	// Node for loading kernel modules (runs on all target hosts)
	nodeIDLoadModules, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:     loadModulesStepName, // Use step's instance name
		Step:     loadModulesStep,
		Hosts:    allHosts,
		StepName: loadModulesStep.Meta().Name,
	})

	// Node for setting sysctl parameters (runs on all target hosts)
	// This node depends on the kernel modules being loaded first on each respective host.
	// This implies a per-host dependency rather than a global one, if AddNode created per-host nodes.
	// However, if AddNode creates one node for allHosts, then the dependency is global.
	// Current AddNodePerHost (if used by module) or AddNode (if used directly) strategy matters.
	// For this task, let's assume these steps apply to all hosts in one go.
	nodeIDSetSysctl, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:     setSysctlStepName, // Use step's instance name
		Step:     setSysctlStep,
		Hosts:    allHosts,
		StepName: setSysctlStep.Meta().Name,
		Dependencies: []plan.NodeID{nodeIDLoadModules},
	})

	fragment.EntryNodes = []plan.NodeID{nodeIDLoadModules}
	fragment.ExitNodes = []plan.NodeID{nodeIDSetSysctl}

	logger.Info("Kernel setup task planned.")
	return fragment, nil
}

// Ensure SetupKernelTask implements the task.Task interface.
var _ task.Task = (*SetupKernelTask)(nil)
