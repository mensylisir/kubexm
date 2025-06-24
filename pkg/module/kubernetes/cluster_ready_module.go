package module

import (
	// "fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// networktask "github.com/mensylisir/kubexm/pkg/task/network"
	// kubernetestask "github.com/mensylisir/kubexm/pkg/task/kubernetes"
	// addontask "github.com/mensylisir/kubexm/pkg/task/addon"
)

// ClusterReadyModule groups tasks for final cluster configurations like CNI,
// post-install scripts, and addons.
type ClusterReadyModule struct {
	BaseModule
}

// NewClusterReadyModule creates a new ClusterReadyModule.
func NewClusterReadyModule() Module {
	return &ClusterReadyModule{
		BaseModule: NewBaseModule(
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
func (m *ClusterReadyModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	// 1. Instantiate tasks: InstallNetworkPlugin, PostScriptTask, InstallAddons.
	// 2. Plan each task.
	// 3. Link task fragments:
	//    - PostScriptTask might depend on InstallNetworkPlugin.
	//    - InstallAddons might depend on PostScriptTask or InstallNetworkPlugin.
	//    - Some addons might be installable in parallel.
	// 4. This module depends on ClusterBootstrapModule (cluster is joined).
	return task.NewEmptyFragment(), nil // Placeholder
}

var _ Module = (*ClusterReadyModule)(nil)
