package network

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes"
	"github.com/mensylisir/kubexm/pkg/task"
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
	taskFragment := task.NewExecutionFragment(t.Name())

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network == nil || clusterCfg.Spec.Network.Plugin == "" {
		logger.Info("No CNI plugin specified, skipping network plugin installation.")
		return task.NewEmptyFragment(), nil
	}
	// Store NetworkConfig on the task instance for easier access within Plan
	// This should be done in the constructor or via a setter if design prefers.
	// For this implementation, we'll assume it's okay to set it here if nil.
	if t.NetworkConfig == nil {
		t.NetworkConfig = clusterCfg.Spec.Network
	}
	pluginName := t.NetworkConfig.Plugin
	logger.Info("Planning CNI plugin installation.", "plugin", pluginName)

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for CNI task: %w", err)
	}

	// Determine the node to run kubectl apply (typically a master node)
	// In a single control-plane node setup, this could also be the control node if it's also a master.
	// For multi-master, any master can do it. For simplicity, pick the first one.
	// If no masters, this task might not be runnable or should target control-node if it has kubectl.
	execNodes, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil || len(execNodes) == 0 {
		logger.Info("No master nodes found to apply CNI manifest, will attempt on control node if it has kubectl access (not implemented yet).", "plugin", pluginName)
		// Fallback or error based on design. For now, let's assume CNI apply needs a k8s node.
		// If control node is also a master (common in single-node dev setups), this is fine.
		// If strictly separate, this task might need to ensure kubectl is on control node and configured.
		// For a typical multi-node cluster, applying from a master is standard.
		// If this task is part of a pipeline after master init, a master should be available.
		return nil, fmt.Errorf("no master nodes found to apply CNI manifest for plugin %s. Ensure master nodes are available or control node is configured for kubectl.", pluginName)
	}
	kubectlApplyNode := execNodes[0] // Apply CNI from the first master node

	var manifestContent string
	var templateData interface{}
	var localManifestTempPath string // Path on control node for downloaded/rendered manifest
	var remoteManifestPathOnExecNode string = filepath.Join("/tmp", pluginName+"-cni-manifest.yaml")

	var lastLocalStepID plan.NodeID = "" // Tracks last step on control node

	// 1. Get/Prepare CNI Manifest (on Control Node)
	switch pluginName {
	case common.CNICalico:
		// TODO: Calico manifests are versioned. Need to get the correct URL or embedded template.
		// Example: "https://raw.githubusercontent.com/projectcalico/calico/vX.Y.Z/manifests/calico.yaml"
		// Or "https://raw.githubusercontent.com/projectcalico/calico/master/manifests/tigera-operator.yaml" and "custom-resources.yaml"
		// For simplicity, let's assume a single templated manifest.
		// manifestContent = calicoTemplate // Assume calicoTemplate is an embedded Go template string
		// templateData = t.NetworkConfig.Calico // This would be v1alpha1.CalicoConfig
		// A more complete Calico setup might involve specific data extraction for the template:
		templateData = struct {
			PodCIDR      string
			CalicoConfig *v1alpha1.CalicoConfig
		}{
			PodCIDR:      t.NetworkConfig.KubePodsCIDR,
			CalicoConfig: t.NetworkConfig.Calico,
		}
		// This template would need to be defined, similar to kubeadm config template.
		// For now, let's assume a placeholder step that "prepares" it.
		// In reality, this would be a resource.RemoteFileHandle or resource.LocalTemplateHandle
		// For now, let's assume we have a function that returns the content for calico
		manifestContent, err = GetCalicoManifestContent(ctx, t.NetworkConfig) // Placeholder func
		if err != nil { return nil, err }
		localManifestTempPath = filepath.Join(ctx.GetGlobalWorkDir(), "calico-rendered.yaml")

		renderStep := commonstep.NewRenderTemplateStep(
			"RenderCalicoManifest",
			manifestContent, // This is the template string itself
			templateData,
			localManifestTempPath, // Render to local path on control node
			"0644", false, // Sudo false for local render
		)
		renderNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
			Name: renderStep.Meta().Name, Step: renderStep, Hosts: []connector.Host{controlNode},
		})
		lastLocalStepID = renderNodeID

	case common.CNIFlannel:
		// Example: "https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml"
		// This might also need templating for PodCIDR.
		// manifestContent = flannelTemplate
		// templateData = struct{ PodCIDR string }{ PodCIDR: t.NetworkConfig.KubePodsCIDR }
		manifestContent, err = GetFlannelManifestContent(ctx, t.NetworkConfig) // Placeholder func
		if err != nil { return nil, err }
		localManifestTempPath = filepath.Join(ctx.GetGlobalWorkDir(), "flannel-rendered.yaml")

		renderStep := commonstep.NewRenderTemplateStep(
			"RenderFlannelManifest", manifestContent,
			struct{ PodCIDR string }{ PodCIDR: t.NetworkConfig.KubePodsCIDR }, // Example data
			localManifestTempPath, "0644", false)
		renderNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
			Name: renderStep.Meta().Name, Step: renderStep, Hosts: []connector.Host{controlNode},
		})
		lastLocalStepID = renderNodeID

	// TODO: Add cases for Cilium (likely Helm or Operator), Kube-OVN, etc.
	default:
		return nil, fmt.Errorf("unsupported CNI plugin '%s' for task %s", pluginName, t.Name())
	}

	// 2. Upload Manifest to Execution Node (e.g., first master)
	uploadManifestStep := commonstep.NewUploadFileStep(
		"Upload-"+pluginName+"-Manifest",
		localManifestTempPath, // Source from control node
		remoteManifestPathOnExecNode,
		"0644", false, false, // Sudo false for /tmp on remote
	)
	uploadNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  uploadManifestStep.Meta().Name,
		Step:  uploadManifestStep,
		Hosts: []connector.Host{kubectlApplyNode},
		Dependencies: []plan.NodeID{lastLocalStepID}, // Depends on local render/prep
	})

	// 3. Apply Manifest using Kubectl on Execution Node
	// Kubeconfig path for kubectl apply. If empty, assumes in-cluster config or default path on node.
	// For applying CNI right after init, /etc/kubernetes/admin.conf is typically used.
	kubeconfigPathOnNode := "/etc/kubernetes/admin.conf"
	applyManifestStep := kubernetessteps.NewKubectlApplyStep(
		"Apply-"+pluginName+"-Manifest",
		remoteManifestPathOnExecNode,
		kubeconfigPathOnNode,
		true, // Sudo for kubectl if needed, or if kubeconfig is root-only
		3, 10, // Retries and delay
	)
	applyNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  applyManifestStep.Meta().Name,
		Step:  applyManifestStep,
		Hosts: []connector.Host{kubectlApplyNode},
		Dependencies: []plan.NodeID{uploadNodeID},
	})

	taskFragment.CalculateEntryAndExitNodes()
	logger.Info("CNI plugin installation task planning complete.", "plugin", pluginName, "entryNodes", taskFragment.EntryNodes, "exitNodes", taskFragment.ExitNodes)
	return taskFragment, nil
}

// Placeholder functions for getting manifest content - these would fetch or load templates
func GetCalicoManifestContent(ctx task.TaskContext, netCfg *v1alpha1.NetworkConfig) (string, error) {
	// In a real scenario, this might fetch from a URL or read an embedded file.
	// It would then replace placeholders like {{ .PodCIDR }} etc.
	// For now, return a very basic Calico template structure.
	// This needs to be the *template string itself*, not the rendered content yet.
	// The actual calico.yaml is complex. This is a simplified representation of a template.
	// Using a well-known Calico version manifest URL might be better.
	// Example: return fetchContent("https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml")
	// And then the RenderTemplateStep would replace vars if needed.
	// If it's a static file, RenderTemplateStep is not needed, just Upload + Apply.
	// For this example, assume it's a template that needs PodCIDR.
	return `apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    ipPools:
    - blockSize: 26
      cidr: {{ .PodCIDR | default "192.168.0.0/16" }}
      encapsulation: {{ .CalicoConfig.IPIPMode | default "IPIP" | replace "Always" "IPIP" | replace "CrossSubnet" "IPIP" | replace "Never" "None" }} # Simplified mapping
      natOutgoing: {{ .CalicoConfig.IPv4NatOutgoing | default true }}
      nodeSelector: all()
---
# Other Calico resources if needed...
`, nil
}

func GetFlannelManifestContent(ctx task.TaskContext, netCfg *v1alpha1.NetworkConfig) (string, error) {
	// Example: fetch "https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml"
	// This manifest often requires the podCIDR to be patched in or uses a ConfigMap.
	// A common approach is to download it, modify the net-conf.json part, then apply.
	// For simplicity, assume a template where PodCIDR can be injected.
	return `
apiVersion: v1
kind: Namespace
metadata:
  name: kube-flannel
  labels:
    pod-security.kubernetes.io/enforce: privileged
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: flannel
rules:
  # ... (flannel RBAC rules) ...
---
# ... (rest of flannel manifest) ...
# The important part for PodCIDR is usually in a ConfigMap for net-conf.json
# Example snippet that would be part of the larger YAML:
# ---
# kind: ConfigMap
# apiVersion: v1
# metadata:
#   name: kube-flannel-cfg
#   namespace: kube-flannel
#   labels:
#     tier: node
#     app: flannel
# data:
#   net-conf.json: |
#     {
#       "Network": "{{ .PodCIDR | default "10.244.0.0/16" }}",
#       "Backend": {
#         "Type": "{{ .FlannelConfig.BackendMode | default "vxlan" }}"
#       }
#     }
# ...
`, nil
}


var _ task.Task = (*InstallNetworkPluginTask)(nil)
