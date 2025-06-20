package pipeline

import (
	// "fmt" // No longer needed for this basic spec construction
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	moduleFactory "github.com/mensylisir/kubexm/pkg/module" // Assuming NewWebServerModuleSpec is in the root module package
)

// NewDeployAppPipelineSpec creates a new PipelineSpec for deploying an application,
// which might involve setting up a web server.
// It takes a config.Cluster for consistency, though it might not use it directly
// if the application deployment is static or configured elsewhere.
func NewDeployAppPipelineSpec(cfg *config.Cluster) *spec.PipelineSpec {
	// Create ModuleSpecs for this pipeline.
	// Assumes NewWebServerModuleSpec is the refactored factory in the 'module' package.
	webServerModuleSpec := moduleFactory.NewWebServerModuleSpec(cfg)

	modules := []*spec.ModuleSpec{
		webServerModuleSpec,
		// Add other modules relevant to app deployment, e.g.,
		// moduleFactory.NewDatabaseSetupModuleSpec(cfg),
		// moduleFactory.NewAppDeployerModuleSpec(cfg),
	}

	// IsEnabled condition for the pipeline itself is not part of PipelineSpec.
	// Pipelines are typically selected and run explicitly by the user or higher-level orchestrator.

	return &spec.PipelineSpec{
		Name:        "DeployApplication",
		Description: "Pipeline to deploy a standard web application, including web server setup.",
		Modules:     modules,
		PreRunHook:  "", // Example: "deploy_app_pre_check_hook"
		PostRunHook: "", // Example: "deploy_app_post_cleanup_hook"
	}
}
