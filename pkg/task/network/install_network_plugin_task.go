package network

import (
	// "fmt"
	// "github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	// kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes"
)

// InstallNetworkPluginTask deploys the chosen CNI network plugin.
type InstallNetworkPluginTask struct {
	task.BaseTask
	// CNIPluginName string // e.g., "calico", "flannel" - from ClusterConfig
	// CNIManifestURLOrPath string // Path/URL to the CNI manifest template or static file
	// CNIConfigParams map[string]interface{} // Parameters for CNI manifest template
}

// NewInstallNetworkPluginTask creates a new InstallNetworkPluginTask.
func NewInstallNetworkPluginTask( /*pluginName, manifestPath string, params map[string]interface{}*/ ) task.Task {
	return &InstallNetworkPluginTask{
		BaseTask: task.BaseTask{
			TaskName: "InstallNetworkPlugin",
			TaskDesc: "Deploys the CNI network plugin to the cluster.",
			// This task usually runs steps on a master node (kubectl apply) or control node (template rendering).
		},
		// CNIPluginName: pluginName,
		// CNIManifestURLOrPath: manifestPath,
		// CNIConfigParams: params,
	}
}

func (t *InstallNetworkPluginTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Required if a CNI plugin is specified in ClusterConfig.
	// return ctx.GetClusterConfig().Spec.Network.Plugin != "", nil
	return true, nil // Placeholder
}

func (t *InstallNetworkPluginTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	// Plan would involve:
	// 1. Determine CNI plugin type and version from ClusterConfig.
	// 2. Step: (Optional) Download CNI manifest/template to control node if it's remote.
	//    (Could use a resource.RemoteFileHandle or similar).
	// 3. Step: RenderTemplateStep to render the CNI manifest using CNIConfigParams (on control node).
	// 4. Step: UploadFileStep to upload the rendered manifest to a master node.
	// 5. Step: KubectlApplyStep on the master node to apply the manifest.
	//
	// Dependencies:
	//  - KubectlApplyStep depends on UploadFileStep.
	//  - UploadFileStep depends on RenderTemplateStep (if applicable).
	//  - RenderTemplateStep depends on Download (if applicable).
	//  - This whole task depends on the Kubernetes control plane being ready (InitMasterTask, JoinMastersTask).
	return task.NewEmptyFragment(), nil // Placeholder
}

var _ task.Task = (*InstallNetworkPluginTask)(nil)
