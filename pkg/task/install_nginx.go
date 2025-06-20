package task

import (
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
	"github.com/mensylisir/kubexm/pkg/step"    // Updated import path
)

type InstallNginxTask struct{}

func NewInstallNginxTask() Task {
	return &InstallNginxTask{}
}

func (t *InstallNginxTask) Name() string {
	return "InstallNginx"
}

func (t *InstallNginxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Example: Check if any host has the "web-server" role.
	// This is a simplified check; a real task might have more complex conditions.
	// webHosts, err := ctx.GetHostsByRole("web-server")
	// if err != nil {
	//	 return false, err
	// }
	// return len(webHosts) > 0, nil
	return true, nil // For this example, always assume it's required.
}

func (t *InstallNginxTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	// Example: Get hosts with the "web-server" role.
	// The issue description assumes GetHostsByRole is implemented on TaskContext.
	webHosts, err := ctx.GetHostsByRole("web-server")
	if err != nil {
		// If no hosts found for the role, or an error occurs,
		// it might mean an empty plan or an error, depending on desired behavior.
		// Returning an empty plan if no hosts match the role.
		// ctx.GetLogger().Warnf("InstallNginxTask: Error getting web-server hosts or no hosts found: %v. Returning empty plan.", err)
		return &plan.ExecutionPlan{Phases: []plan.Phase{}}, nil // Return empty plan, not an error
	}
    if len(webHosts) == 0 {
        // ctx.GetLogger().Infof("InstallNginxTask: No hosts found with role 'web-server'. Skipping Nginx installation task.")
        return &plan.ExecutionPlan{Phases: []plan.Phase{}}, nil // Return empty plan
    }


	// Create the installation step
	installStep := step.NewInstallPackagesStep("nginx") // Using the step from pkg/step

	// Define the action for installing Nginx on these hosts
	installAction := plan.Action{
		Name:  "Install nginx package",
		Step:  installStep,
		Hosts: webHosts,
	}

	// Define the phase for this action
	installPhase := plan.Phase{
		Name:    "Install Nginx",
		Actions: []plan.Action{installAction},
	}

	// Construct the execution plan
	executionPlan := &plan.ExecutionPlan{
		Phases: []plan.Phase{installPhase},
	}

	return executionPlan, nil
}
