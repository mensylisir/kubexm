package network

import (
	"github.com/mensylisir/kubexm/pkg/task"
	// commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	// kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes"
)

// InstallNetworkPluginTask deploys the chosen CNI network plugin.
type InstallNetworkPluginTask struct {
	task.BaseTask
	NetworkConfig *v1alpha1.NetworkConfig // To access plugin type, CNI specific configs, CIDRs
}

// NewInstallNetworkPluginTask creates a new InstallNetworkPluginTask.
func NewInstallNetworkPluginTask() task.Task { // Config will be fetched from context
	return &InstallNetworkPluginTask{
		BaseTask: task.NewBaseTask( // Use NewBaseTask
			"InstallNetworkPlugin",
			"Deploys the CNI network plugin to the cluster.",
			nil,   // RunOnRoles - typically control-node or a master
			nil,   // HostFilter
			false, // IgnoreError
		),
	}
}

func (t *InstallNetworkPluginTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// Required if a CNI plugin is specified in ClusterConfig.
	// return ctx.GetClusterConfig().Spec.Network.Plugin != "", nil
	return true, nil // Placeholder
}

func (t *InstallNetworkPluginTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	networkFragment := task.NewExecutionFragment(t.Name())

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network == nil || clusterCfg.Spec.Network.Plugin == "" {
		logger.Info("No CNI plugin specified, skipping network plugin installation.")
		return task.NewEmptyFragment(), nil
	}
	t.NetworkConfig = clusterCfg.Spec.Network // Store for easy access

	logger.Info("Planning CNI plugin installation.", "plugin", t.NetworkConfig.Plugin)

	// controlNode, _ := ctx.GetControlNode()
	// masterNodes, _ := ctx.GetHostsByRole(common.RoleMaster)
	// if len(masterNodes) == 0 { return nil, fmt.Errorf("no master node found to apply CNI manifest for plugin %s", t.NetworkConfig.Plugin) }
	// execNode := masterNodes[0] // Apply CNI from one master

	// --- Conceptual Plan ---
	// manifestContent := ""
	// manifestIsTemplate := false
	// templateData := make(map[string]interface{})

	// switch t.NetworkConfig.Plugin {
	// case common.CNICalico:
	//    // manifestContent = calicoManifestTemplateContent (could be embedded or from resource.LocalFileHandle)
	//    // manifestIsTemplate = true
	//    // templateData["PodCIDR"] = t.NetworkConfig.KubePodsCIDR
	//    // templateData["CalicoConfig"] = t.NetworkConfig.Calico
	//    // ... other calico specific params
	// case common.CNIFlannel:
	//    // manifestContent = flannelManifestContent
	//    // templateData["PodCIDR"] = t.NetworkConfig.KubePodsCIDR
	//    // ...
	// case common.CNICilium:
	//    // Cilium is often installed via Helm or a more complex operator.
	//    // This task might create HelmInstallStep or KubectlApplyStep for an operator.
	//    // For simplicity, assume YAML manifest for now.
	//    // manifestContent = ciliumManifestContent
	//    // templateData["CiliumConfig"] = t.NetworkConfig.Cilium
	// default:
	//    return nil, fmt.Errorf("unsupported CNI plugin: %s", t.NetworkConfig.Plugin)
	// }

	// var lastStepID plan.NodeID = ""
	// localRenderedManifestPath := filepath.Join(ctx.GetGlobalWorkDir(), "rendered-"+t.NetworkConfig.Plugin+"-cni.yaml")

	// if manifestIsTemplate {
	//    renderStep := commonsteps.NewRenderTemplateStep("Render-"+t.NetworkConfig.Plugin+"-CNI", manifestContent, templateData, localRenderedManifestPath, "0644", false)
	//    renderNodeID, _ := networkFragment.AddNode(&plan.ExecutionNode{ Step: renderStep, Hosts: []connector.Host{controlNode}})
	//    lastStepID = renderNodeID
	// } else {
	//    // If static manifest, resource handle would place it at localRenderedManifestPath (or similar)
	//    // For now, assume content is ready or use a resource handle for it.
	// }

	// remoteManifestPath := "/tmp/" + t.NetworkConfig.Plugin + "-cni.yaml"
	// uploadStep := commonsteps.NewUploadFileStep("Upload-"+t.NetworkConfig.Plugin+"-CNI", localRenderedManifestPath, remoteManifestPath, "0644", false, false)
	// uploadNodeID, _ := networkFragment.AddNode(&plan.ExecutionNode{ Step: uploadStep, Hosts: []connector.Host{execNode}, Dependencies: []plan.NodeID{lastStepID} })
	// lastStepID = uploadNodeID

	// applyStep := kubernetessteps.NewKubectlApplyStep("Apply-"+t.NetworkConfig.Plugin+"-CNI", remoteManifestPath, "", false, 3, 10) // Retries
	// applyNodeID, _ := networkFragment.AddNode(&plan.ExecutionNode{ Step: applyStep, Hosts: []connector.Host{execNode}, Dependencies: []plan.NodeID{lastStepID} })

	// networkFragment.EntryNodes = ...
	// networkFragment.ExitNodes = []plan.NodeID{applyNodeID}
	// networkFragment.CalculateEntryAndExitNodes()

	logger.Warn("InstallNetworkPluginTask.Plan is a placeholder and needs full implementation.", "plugin", t.NetworkConfig.Plugin)
	return task.NewEmptyFragment(), nil // Placeholder until fully implemented
}

var _ task.Task = (*InstallNetworkPluginTask)(nil)
