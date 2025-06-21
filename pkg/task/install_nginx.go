package task

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/plan"      // For plan.NodeID, plan.ExecutionNode
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step" // For step types, e.g., step.NewInstallPackagesStep
	// Assuming spec.StepMeta is available via step.Step's Meta() method.
)

// InstallNginxTask defines the task for installing Nginx.
type InstallNginxTask struct {
	*BaseTask // Embed BaseTask
}

// NewInstallNginxTask creates a new task for installing Nginx.
func NewInstallNginxTask() Task { // Return task.Task (updated interface name)
	base := NewBaseTask(
		"InstallNginx",                                  // Name
		"Installs the Nginx web server on target hosts.", // Description
		[]string{"web-server"},                          // Default RunOnRoles
		nil,                                             // No custom host filter for this example
		false,                                           // Not ignoring errors by default
	)
	return &InstallNginxTask{
		BaseTask: &base,
	}
}

// Name() is inherited from BaseTask.

// Description is inherited from BaseTask.
// func (t *InstallNginxTask) Description() string {
// 	return t.BaseTask.TaskDesc
// }

// IsRequired can use BaseTask.RunOnRoles to check if any hosts match.
// (Implementation remains the same as before, just logging phase might change if desired)
func (t *InstallNginxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "IsRequired") // "phase" label might be less relevant now
	if len(t.BaseTask.RunOnRoles) == 0 {
		logger.Info("No specific roles defined in RunOnRoles, task considered required by default if not filtered out otherwise.")
		// If BaseTask.HostFilter is nil, this task will apply to all hosts if no roles specified.
		// This behavior might need adjustment based on overall design for role-less tasks.
		// For now, assume if roles are empty, it's required (and might apply to all hosts or control-node).
		return true, nil
	}
	for _, role := range t.BaseTask.RunOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			logger.Error(err, "Error checking hosts for role", "role", role)
			return false, fmt.Errorf("error checking hosts for role %s: %w", role, err)
		}
		if len(hosts) > 0 {
			logger.V(1).Info("Hosts found for role, task is required.", "role", role)
			return true, nil
		}
	}
	logger.Info("No hosts found for specified roles, task not required.", "roles", t.BaseTask.RunOnRoles)
	return false, nil
}

// Plan generates the execution fragment to install Nginx.
func (t *InstallNginxTask) Plan(ctx runtime.TaskContext) (*ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan") // "phase" label might be less relevant

	var allTargetHosts []connector.Host

	if len(t.BaseTask.RunOnRoles) == 0 {
		logger.Info("No specific roles defined for Nginx installation. This task will target no hosts by role.")
		// Potentially, if no roles, it might target control-node or all nodes depending on task type.
		// For InstallNginx, it likely needs specific hosts.
	} else {
		for _, role := range t.BaseTask.RunOnRoles {
			hosts, err := ctx.GetHostsByRole(role)
			if err != nil {
				logger.Error(err, "Error getting hosts for role", "role", role)
				// Decide if one role error fails the whole task plan or just skips that role.
				// For now, let's continue and collect hosts from other successful roles.
				continue
			}
			if len(hosts) > 0 {
				allTargetHosts = append(allTargetHosts, hosts...)
			}
		}
	}

	// Deduplicate hosts if a host has multiple matching roles
	if len(allTargetHosts) > 0 {
		uniqueHostsMap := make(map[string]connector.Host)
		for _, h := range allTargetHosts {
			uniqueHostsMap[h.GetName()] = h
		}
		allTargetHosts = make([]connector.Host, 0, len(uniqueHostsMap)) // Clear and repopulate
		for _, h := range uniqueHostsMap {
			allTargetHosts = append(allTargetHosts, h)
		}
	}

	if len(allTargetHosts) == 0 {
		logger.Info("No target hosts found for Nginx installation after checking roles. Returning empty fragment.")
		return &ExecutionFragment{
			Nodes:      make(map[plan.NodeID]*plan.ExecutionNode),
			EntryNodes: []plan.NodeID{},
			ExitNodes:  []plan.NodeID{},
		}, nil
	}

	hostNames := make([]string, len(allTargetHosts))
	for i, h := range allTargetHosts {
		hostNames[i] = h.GetName()
	}
	logger.Info("Planning to install Nginx", "hostsCount", len(allTargetHosts), "hostNames", hostNames)

	// Create the installation step
	// Assuming NewInstallPackagesStep's second arg was for a custom Step name/ID.
	// The step instance itself will have Meta().Name.
	installPkgsStep := step.NewInstallPackagesStep([]string{"nginx"}, "Install Nginx Package") // Pass a descriptive name for the step instance

	nodeName := "InstallNginxPackageNode" // A descriptive name for this specific node in the graph
	nodeID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), nodeName))

	execNode := &plan.ExecutionNode{
		Name:         nodeName,
		Step:         installPkgsStep,
		Hosts:        allTargetHosts,
		StepName:     installPkgsStep.Meta().Name, // Get name from step's metadata
		Dependencies: []plan.NodeID{},          // No dependencies for this single-step task
		HostNames:    hostNames,                 // Store hostnames for marshalling/logging
	}

	fragment := &ExecutionFragment{
		Nodes: map[plan.NodeID]*plan.ExecutionNode{
			nodeID: execNode,
		},
		EntryNodes: []plan.NodeID{nodeID}, // This node is the entry point
		ExitNodes:  []plan.NodeID{nodeID}, // This node is also the exit point
	}

	return fragment, nil
}

// Ensure InstallNginxTask implements the new task.Task interface.
var _ Task = (*InstallNginxTask)(nil)
