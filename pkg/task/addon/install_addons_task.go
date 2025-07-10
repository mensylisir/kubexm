package addon

import (
	"fmt"
	"path/filepath"
	"strings"

	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // If reading structured addon config
	"github.com/mensylisir/kubexm/pkg/common" // For common.RoleMaster, common.ControlNodeRole potentially
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	helmsteps "github.com/mensylisir/kubexm/pkg/step/helm"
	kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes"
	"github.com/mensylisir/kubexm/pkg/task"
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

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for addon task %s: %w", t.AddonName, err)
	}
	// Kubectl/Helm commands are typically run from a node that can access the cluster.
	// This could be the control node if it's configured with kubectl, or a master node.
	// For simplicity, let's assume control node for now if it's set up for it,
	// or default to first master node.
	execHost := controlNode // Default execution host to control node

	// Determine install type (helm or yaml) and parameters from t.AddonName and t.Config
	// This logic needs to be more robust, possibly looking up addon details from a catalog.
	// Example:
	addonType := "yaml" // Default to YAML, or determine from config
	manifestURLOrPath := ""
	chartName := ""
	helmRepoURL := ""
	helmRepoName := ""
	helmChartVersion := ""
	helmNamespace := "default"
	helmValuesFiles := []string{} // Paths on the execHost
	helmSetValues := []string{}

	if configMap, ok := t.Config["type"].(string); ok {
		addonType = strings.ToLower(configMap)
	}
	// Populate other helm/yaml params from t.Config...
	// E.g., chartName = t.Config["chartName"].(string), etc.

	// Simplified example for "metallb" (YAML) and a hypothetical "prometheus-op" (Helm)
	if t.AddonName == "metallb" {
		addonType = "yaml"
		// manifestURLOrPath = "https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml" // Example
		manifestURLOrPath = "https://raw.githubusercontent.com/metallb/metallb/main/config/manifests/metallb-native.yaml" // Use main for now
		logger.Info("Configured to install metallb using YAML.", "url", manifestURLOrPath)
	} else if t.AddonName == "prometheus-operator" { // Hypothetical helm addon
		addonType = "helm"
		helmRepoName = "prometheus-community"
		helmRepoURL = "https.prometheus-community.github.io/helm-charts"
		chartName = "prometheus-community/kube-prometheus-stack"
		helmNamespace = "monitoring"
		helmChartVersion = "55.6.0" // Example version
		logger.Info("Configured to install prometheus-operator using Helm.", "chart", chartName)
	} else {
		logger.Warn("No specific plan defined for addon in this placeholder.", "addon_name", t.AddonName)
		return task.NewEmptyFragment(), nil
	}


	var lastStepID plan.NodeID = ""

	if addonType == "yaml" {
		if manifestURLOrPath == "" {
			return nil, fmt.Errorf("manifestURLOrPath is required for YAML addon %s", t.AddonName)
		}

		// 1. Resource Handle for manifest (download to control node if URL)
		localManifestPath := manifestURLOrPath
		if strings.HasPrefix(manifestURLOrPath, "http://") || strings.HasPrefix(manifestURLOrPath, "https://") {
			localManifestPath = filepath.Join(ctx.GetGlobalWorkDir(), t.AddonName+"-manifest.yaml")
			downloadStep := commonstep.NewDownloadFileStep("Download-"+t.AddonName, manifestURLOrPath, localManifestPath, "", "", false)
			downloadNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
				Name: downloadStep.Meta().Name, Step: downloadStep, Hosts: []connector.Host{controlNode},
			})
			lastStepID = downloadNodeID
		}
		// TODO: Add RenderTemplateStep if manifest is a template

		// 2. Upload manifest to execNode (if execNode is not controlNode)
		remoteManifestPath := filepath.Join("/tmp", t.AddonName+"-manifest.yaml")
		if execHost.GetName() != controlNode.GetName() {
			uploadStep := commonstep.NewUploadFileStep("UploadManifest-"+t.AddonName, localManifestPath, remoteManifestPath, "0644", false, false)
			uploadNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
				Name: uploadStep.Meta().Name, Step: uploadStep, Hosts: []connector.Host{execHost}, Dependencies: []plan.NodeID{lastStepID},
			})
			lastStepID = uploadNodeID
		} else {
			remoteManifestPath = localManifestPath // Applying from control node directly
		}

		// 3. KubectlApplyStep on execNode
		kubeconfigPath := "/etc/kubernetes/admin.conf" // Default path on a master/configured node
		// If execHost is controlNode, KubeconfigPath might be $HOME/.kube/config or from context
		if execHost.GetName() == controlNode.GetName() {
			// TODO: Determine appropriate kubeconfig path for control node execution
			// For now, assume default or an empty string to use kubectl's default discovery.
			kubeconfigPath = ""
		}
		applyStep := kubernetessteps.NewKubectlApplyStep("ApplyManifest-"+t.AddonName, remoteManifestPath, kubeconfigPath, true, 2, 5)
		applyNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
			Name: applyStep.Meta().Name, Step: applyStep, Hosts: []connector.Host{execHost}, Dependencies: []plan.NodeID{lastStepID},
		})
		lastStepID = applyNodeID

	} else if addonType == "helm" {
		// TODO: Implement HelmRepoAddStep if it doesn't exist.
		// For now, assume repo is added manually or by a prior global step.
		// if helmRepoURL != "" && helmRepoName != "" {
		//     addRepoStep := helmsteps.NewHelmRepoAddStep(helmRepoName, helmRepoURL, "", "", true) // username, password, sudo
		//     repoNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
		// 		Name: addRepoStep.Meta().Name, Step: addRepoStep, Hosts: []connector.Host{execHost}, Dependencies: []plan.NodeID{lastStepID},
		// 	})
		//     lastStepID = repoNodeID
		// }

		// Values files would need to be prepared on control node and uploaded to execHost if not already there.
		// For simplicity, assume ValuesFiles contains paths already accessible on execHost or are not used.

		kubeconfigPath := "/etc/kubernetes/admin.conf"
		if execHost.GetName() == controlNode.GetName() { kubeconfigPath = "" }

		installStep := helmsteps.NewHelmInstallStep(
			t.AddonName, // Instance name for step, use addon name as release name
			t.AddonName, // Release name
			chartName,   // Chart path (<repo>/<chart> or URL or local path)
			helmNamespace,
			helmChartVersion,
			helmValuesFiles,
			helmSetValues,
			true, // Create namespace
			true, // Sudo for helm command (usually not needed if kubeconfig is set right)
			2, 5, // Retries, RetryDelay
		)
		// The HelmInstallStep needs its KubeconfigPath field set.
		// For now, assume it's passed in constructor or a field.
		// The current NewHelmInstallStep doesn't take kubeconfig as a direct param.
		// Let's assume it's configured globally or via env for helm, or HelmInstallStep needs update.
		// The HelmInstallStep I reviewed has KubeconfigPath field.
		// helmInstallStepInstance := installStep.(*helmsteps.HelmInstallStep)
		// helmInstallStepInstance.KubeconfigPath = kubeconfigPath

		// Re-creating with all params for clarity based on NewHelmInstallStep signature:
		// NewHelmInstallStep(instanceName, releaseName, chartPath, namespace, version string, valuesFiles, setValues []string, createNamespace, sudo bool, retries, retryDelay int)
		 finalHelmInstallStep := helmsteps.NewHelmInstallStep(
			fmt.Sprintf("HelmInstall-%s", t.AddonName),
			t.AddonName, // Release Name
			chartName,   // Chart Path
			helmNamespace,
			helmChartVersion,
			helmValuesFiles, // Paths on execHost
			helmSetValues,
			true, // createNamespace
			true, // Sudo (for helm binary itself, if needed)
			2, 5,  // retries, retryDelay
		)
		// Manually set KubeconfigPath on the struct if New constructor doesn't take it
		 if typedStep, ok := finalHelmInstallStep.(*helmsteps.HelmInstallStep); ok {
			typedStep.KubeconfigPath = kubeconfigPath
		 }


		installNodeID, _ := addonFragment.AddNode(&plan.ExecutionNode{
			Name: finalHelmInstallStep.Meta().Name, Step: finalHelmInstallStep, Hosts: []connector.Host{execHost}, Dependencies: []plan.NodeID{lastStepID},
		})
		lastStepID = installNodeID
	} else {
		return nil, fmt.Errorf("unknown addon type '%s' for addon %s", addonType, t.AddonName)
	}

	addonFragment.CalculateEntryAndExitNodes()
	if addonFragment.IsEmpty() {
		logger.Info("Addon task planned no executable nodes.", "addon_name", t.AddonName)
		return task.NewEmptyFragment(), nil
	}
	logger.Info("Addon task planning complete.", "addon_name", t.AddonName)
	return addonFragment, nil
}

var _ task.Task = (*InstallAddonsTask)(nil)
