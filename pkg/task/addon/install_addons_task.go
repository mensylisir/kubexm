package addon

import (
	"github.com/mensylisir/kubexm/pkg/task"
	// commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	// kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes"
	// helmsteps "github.com/mensylisir/kubexm/pkg/step/helm"
)

// InstallAddonsTask deploys configured addons to the Kubernetes cluster.
type InstallAddonsTask struct {
	task.BaseTask
	// AddonConfigs []v1alpha1.AddonConfig // This would be read from ClusterConfig.Spec.Addons
	// If ClusterConfig.Spec.Addons is just []string, this task would need to map names to actual configs.
}

// NewInstallAddonsTask creates a new InstallAddonsTask.
func NewInstallAddonsTask( /*addonCfgs []v1alpha1.AddonConfig*/ ) task.Task {
	return &InstallAddonsTask{
		BaseTask: task.BaseTask{
			TaskName: "InstallClusterAddons",
			TaskDesc: "Deploys optional cluster addons (e.g., metrics-server, dashboard).",
			// Steps typically run on a master node (kubectl apply/helm install) or control node.
		},
		// AddonConfigs: addonCfgs,
	}
}

func (t *InstallAddonsTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// Required if ClusterConfig.Spec.Addons is not empty and contains enabled addons.
	// clusterCfg := ctx.GetClusterConfig()
	// return len(clusterCfg.Spec.Addons) > 0, nil // Assuming Addons is []string of names
	return true, nil // Placeholder
}

func (t *InstallAddonsTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	// Plan would involve:
	// For each addon specified in ClusterConfig.Spec.Addons:
	//  1. Determine addon type (Helm chart, YAML manifests).
	//  2. If Helm:
	//     a. (Optional) Step to add Helm repo if not already added.
	//     b. Step: HelmInstallStep.
	//  3. If YAML:
	//     a. (Optional) Step(s) to download/render YAML manifests to control node.
	//     b. Step: UploadFileStep to upload manifests to a master node.
	//     c. Step: KubectlApplyStep on the master node.
	//
	// Different addons can be installed in parallel if they don't have inter-dependencies.
	// All addon installations depend on PostScriptTask (or a similar "cluster ready" state).
	return task.NewEmptyFragment(), nil // Placeholder
}

var _ task.Task = (*InstallAddonsTask)(nil)
