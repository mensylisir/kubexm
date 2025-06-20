package module

import (
	// "fmt" // No longer needed for this basic spec construction
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	taskFactory "github.com/mensylisir/kubexm/pkg/task" // Assuming NewInstallNginxTaskSpec is in the root task package
)

// NewWebServerModuleSpec creates a new ModuleSpec for managing a web server (e.g., Nginx).
// It takes a config.Cluster for consistency, though it might not use it directly
// if the web server setup is static.
func NewWebServerModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	// Default roles for web server tasks. Could be made configurable.
	webServerRoles := []string{"web-server", "loadbalancer"}

	// Create TaskSpecs for this module.
	// Assumes NewInstallNginxTaskSpec is the refactored factory in the 'task' package (or a sub-package).
	// The previous refactoring of install_nginx.go placed NewInstallNginxTaskSpec in 'package task'.
	installNginxTaskSpec := taskFactory.NewInstallNginxTaskSpec(webServerRoles)

	tasks := []*spec.TaskSpec{
		installNginxTaskSpec,
		// Add other webserver related tasks here, e.g.,
		// taskFactory.NewConfigureVirtualHostTaskSpec(...),
		// taskFactory.NewDeployWebAppTaskSpec(...),
	}

	// IsEnabled condition: For this example, assume it's always enabled if created.
	// Could be tied to a config field like "cfg.Spec.Addons.WebServer.Enabled == true".
	isEnabledCondition := "true"

	return &spec.ModuleSpec{
		Name:        "WebServer",
		Description: "Manages web server installation and configuration (e.g., Nginx).",
		IsEnabled:   isEnabledCondition,
		Tasks:       tasks,
		PreRunHook:  "", // No PreRunHook for this example
		PostRunHook: "", // No PostRunHook for this example
	}
}
