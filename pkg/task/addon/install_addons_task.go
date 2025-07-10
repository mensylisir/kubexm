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
	AddonName string
	Config    map[string]interface{} // Generic config, could be typed if addon configs are structured in API
}

// NewInstallAddonsTask creates a new InstallAddonsTask for a specific addon.
func NewInstallAddonsTask(addonName string, config map[string]interface{}) task.Task {
	// Ensure BaseTask.TaskName is unique for each addon instance for clarity in logging/fragments.
	// The AddonsModule will create one such task per addon string from the config.
	return &InstallAddonsTask{
		BaseTask: task.NewBaseTask(
			fmt.Sprintf("InstallAddon-%s", addonName), // Unique task name
			fmt.Sprintf("Deploys addon: %s", addonName),
			nil,   // RunOnRoles - typically runs on control-node or a master
			nil,   // HostFilter
			false, // IgnoreError
		),
		AddonName: addonName,
		Config:    config,
	}
}

func (t *InstallAddonsTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// Required if ClusterConfig.Spec.Addons is not empty and contains enabled addons.
	// clusterCfg := ctx.GetClusterConfig()
	// return len(clusterCfg.Spec.Addons) > 0, nil // Assuming Addons is []string of names
	return true, nil // Placeholder
}

func (t *InstallAddonsTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "addon", t.AddonName)
	addonFragment := task.NewExecutionFragment(t.Name())

	// Determine how to install this addon (e.g., from t.Config or a predefined map)
	// This is highly dependent on how addon sources/types are defined.
	// For this example, let's assume a simple map or if-else based on t.AddonName.

	// Example:
	// addonInstallType := determineAddonInstallType(t.AddonName, t.Config)
	// controlNode, _ := ctx.GetControlNode() // Steps like Helm might run on control node
	// masterNode, _ := ctx.GetHostsByRole(common.RoleMaster) // Assuming apply on first master
	// if len(masterNode) == 0 { return nil, fmt.Errorf("no master node found to apply addon %s", t.AddonName)}
	// execNode := masterNode[0]

	logger.Info("Planning installation for addon.")

	// switch addonInstallType {
	// case "helm":
	//    chartName := t.Config["chartName"].(string)
	//    repoURL := t.Config["repoURL"].(string) (optional)
	//    repoName := t.Config["repoName"].(string) (optional)
	//    version := t.Config["version"].(string) (optional)
	//    namespace := t.Config["namespace"].(string) (optional, defaults to "default")
	//    valuesFilePath := t.Config["valuesFile"].(string) (optional, path on control node)
	//
	//    var lastStepID plan.NodeID = ""
	//    if repoURL != "" && repoName != "" {
	//        addRepoStep := helmsteps.NewHelmRepoAddStep(repoName, repoURL, "", "", true)
	//        repoNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{... Step: addRepoStep, Hosts: []connector.Host{controlNode} ...})
	//        lastStepID = repoNodeID
	//    }
	//    installStep := helmsteps.NewHelmInstallStep(t.AddonName, chartName, namespace, version, valuesFilePath, true, "") // kubeconfig path
	//    installNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{... Step: installStep, Hosts: []connector.Host{controlNode}, Dependencies: []plan.NodeID{lastStepID} ...})
	//    addonFragment.EntryNodes = ...
	//    addonFragment.ExitNodes = []plan.NodeID{installNodeID}
	//
	// case "yaml":
	//    manifestURL := t.Config["manifestURL"].(string) // Or local path
	//    // 1. Resource handle for manifestURL (download to control node)
	//    // 2. Render template if it's a template
	//    // 3. Upload to execNode
	//    // 4. KubectlApplyStep on execNode
	//    // addonFragment.EntryNodes = ...
	//    // addonFragment.ExitNodes = ...
	// default:
	//    return nil, fmt.Errorf("unknown install type for addon %s", t.AddonName)
	// }

	// Placeholder implementation - actual logic depends on addon management strategy
	if t.AddonName == "metallb" {
		// Simulate planning steps for metallb (e.g., applying a YAML)
		logger.Info("Planning metallb YAML apply (placeholder).")
		// manifestPathOnNode := "/tmp/metallb.yaml" // Assume uploaded by a previous step/resource handle
		// applyStep := kubernetessteps.NewKubectlApplyStep("ApplyMetallb", manifestPathOnNode, "", false, 0,0)
		// applyNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
		// 	Name: applyStep.Meta().Name,
		// 	Step: applyStep,
		// 	Hosts: []connector.Host{execNode},
		//  // Dependencies: Would depend on manifest upload step, which depends on download/render.
		// })
		// addonFragment.EntryNodes = []plan.NodeID{applyNodeID} // Simplified
		// addonFragment.ExitNodes = []plan.NodeID{applyNodeID}
	} else {
		logger.Warn("No specific plan defined for addon in this placeholder.", "addon_name", t.AddonName)
		return task.NewEmptyFragment(), nil // Return empty if no plan for this addon
	}

	// addonFragment.CalculateEntryAndExitNodes() // Call if nodes were added
	if addonFragment.IsEmpty() {
		return task.NewEmptyFragment(), nil
	}
	logger.Info("Addon task planning complete.")
	return addonFragment, nil
}

var _ task.Task = (*InstallAddonsTask)(nil)
