package kubernetes

import (
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	// networktask "github.com/mensylisir/kubexm/internal/task/network"
	// kubernetestask "github.com/mensylisir/kubexm/internal/task/kubernetes"
	// addontask "github.com/mensylisir/kubexm/internal/task/addon"
)

// ClusterReadyModule groups tasks for final cluster configurations like CNI,
// post-install scripts, and addons.
type ClusterReadyModule struct {
	module.BaseModule
}

// NewClusterReadyModule creates a new ClusterReadyModule.
func NewClusterReadyModule() module.Module {
	return &ClusterReadyModule{
		BaseModule: module.NewBaseModule(
			"ClusterFinalConfiguration",
			[]task.Task{
				// networktask.NewInstallNetworkPluginTask(),
				// kubernetestask.NewPostScriptTask(),
				// addontask.NewInstallAddonsTask(),
			},
		),
	}
}

// Plan orchestrates the planning of tasks within this module.
func (m *ClusterReadyModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	// 1. Instantiate tasks: InstallNetworkPlugin, PostScriptTask, InstallAddons.
	// 2. Plan each task.
	// 3. Link task fragments:
	//    - PostScriptTask might depend on InstallNetworkPlugin.
	//    - InstallAddons might depend on PostScriptTask or InstallNetworkPlugin.
	//    - Some addons might be installable in parallel.
	// 4. This module depends on ClusterBootstrapModule (cluster is joined).
	return plan.NewEmptyFragment(m.Name()), nil // Placeholder
}

var _ module.Module = (*ClusterReadyModule)(nil)
