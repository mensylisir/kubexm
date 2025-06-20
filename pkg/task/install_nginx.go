package task

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host in Plan
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step" // For step.NewInstallPackagesStep
	// Assuming runner is not directly needed here, but TaskContext provides access.
)

// InstallNginxTask defines the task for installing Nginx.
type InstallNginxTask struct {
	*BaseTask // Embed BaseTask
}

// NewInstallNginxTask creates a new task for installing Nginx.
func NewInstallNginxTask() Interface { // Return task.Interface
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

// Description returns a brief summary of the task.
// Overriding to ensure it uses the embedded BaseTask's description.
func (t *InstallNginxTask) Description() string {
	return t.BaseTask.TaskDesc
}

// IsRequired can use BaseTask.RunOnRoles to check if any hosts match.
func (t *InstallNginxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "IsRequired")
	if len(t.BaseTask.RunOnRoles) == 0 {
		logger.Info("No specific roles defined in RunOnRoles, task considered required by default.")
		return true, nil
	}
	for _, role := range t.BaseTask.RunOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			logger.Errorf("Error checking hosts for role %s: %v", role, err)
			// Depending on policy, might return true to attempt plan and let Plan handle errors,
			// or false to skip if any role check fails. For now, let's be conservative.
			return false, fmt.Errorf("error checking hosts for role %s: %w", role, err)
		}
		if len(hosts) > 0 {
			logger.Debugf("Hosts found for role '%s', task is required.", role)
			return true, nil
		}
	}
	logger.Infof("No hosts found for roles %v, task not required.", t.BaseTask.RunOnRoles)
	return false, nil
}

// Plan generates the execution plan to install Nginx.
func (t *InstallNginxTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")

	var allTargetHosts []connector.Host

	if len(t.BaseTask.RunOnRoles) == 0 {
		logger.Info("No specific roles defined in RunOnRoles for InstallNginxTask. Planning for no hosts by role.")
		// Depending on desired behavior, this could mean all hosts, or truly no hosts.
		// If it means all hosts, GetHostsByRole("") or a new method ctx.GetAllHosts() would be needed.
		// For now, sticking to the explicit roles: if none, then no hosts selected by this logic.
	} else {
	    for _, role := range t.BaseTask.RunOnRoles {
	        hosts, err := ctx.GetHostsByRole(role)
	        if err != nil {
			logger.Warnf("Error getting hosts for role '%s': %v. Skipping this role for planning.", role, err)
			continue
	        }
	        if len(hosts) > 0 {
	            allTargetHosts = append(allTargetHosts, hosts...)
	        }
	    }
	}

	// Deduplicate hosts if a host has multiple matching roles
	uniqueHostsMap := make(map[string]connector.Host)
	for _, h := range allTargetHosts {
		uniqueHostsMap[h.GetName()] = h
	}
	allTargetHosts = []connector.Host{} // Clear and repopulate
	for _, h := range uniqueHostsMap {
		allTargetHosts = append(allTargetHosts, h)
	}


	if len(allTargetHosts) == 0 {
		logger.Info("No target hosts found for Nginx installation after checking roles. Returning empty plan.")
		return &plan.ExecutionPlan{Phases: []plan.Phase{}}, nil
	}

	hostNames := []string{}
	for _, h := range allTargetHosts { hostNames = append(hostNames, h.GetName()) }
	logger.Infof("Planning to install Nginx on %d hosts: %v", len(allTargetHosts), hostNames)


	// Create the installation step using the refactored NewInstallPackagesStep
	// This assumes NewInstallPackagesStep takes the list of packages and returns a step.Step
	installStep := step.NewInstallPackagesStep([]string{"nginx"}, "") // Second arg is StepName, can be empty for default

	installAction := plan.Action{
		Name:  "Install nginx package", // Name of the action
		Step:  installStep,             // The step to execute
		Hosts: allTargetHosts,          // The hosts to run this action on
	}

	installPhase := plan.Phase{
		Name:    "Install Nginx Server", // Name of the phase
		Actions: []plan.Action{installAction},
	}

	executionPlan := &plan.ExecutionPlan{
		Phases: []plan.Phase{installPhase},
	}

	return executionPlan, nil
}

// Ensure InstallNginxTask implements the task.Interface.
var _ Interface = (*InstallNginxTask)(nil)
