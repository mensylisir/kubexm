package task // Or consider moving to pkg/task/webservers or pkg/task/nginx

import (
	// "fmt" // No longer needed for this basic spec

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/common" // Assuming NewInstallPackagesStep is here or in a "packages" sub-package
	// "github.com/mensylisir/kubexm/pkg/config" // Example if config needed for params
)

// NewInstallNginxTaskSpec creates a new TaskSpec for installing the Nginx web server.
// Parameters:
//   runOnRoles: A slice of strings specifying which host roles this task should run on.
//               If nil or empty, and if the executor doesn't have a default behavior
//               (like running on all nodes), it might not run anywhere.
//               Defaults to ["web-server"] if nil is passed.
func NewInstallNginxTaskSpec(runOnRoles []string) *spec.TaskSpec {
	if runOnRoles == nil {
		runOnRoles = []string{"web-server"}
	}

	// Create the installation step.
	// Assuming NewInstallPackagesStep returns a spec.StepSpec compatible type.
	// The second argument to NewInstallPackagesStep was StepName, can be empty for default.
	installStep := common.NewInstallPackagesStep([]string{"nginx"}, "")

	taskSteps := []spec.StepSpec{
		installStep,
	}

	return &spec.TaskSpec{
		Name:        "InstallNginx",
		Description: "Installs the Nginx web server on target hosts.",
		RunOnRoles:  runOnRoles,
		Steps:       taskSteps,
		IgnoreError: false,
		// Filter: "", // No specific filter
		// Concurrency: 0, // Use global default
	}
}
