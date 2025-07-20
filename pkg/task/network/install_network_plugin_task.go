package network

import (
	"fmt"
	"path/filepath"
	"strings"

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
		BaseTask: task.NewBaseTask(
			"InstallNetworkPlugin",
			"Deploys the CNI network plugin to the cluster.",
			nil,   // RunOnRoles - typically control-node or a master for applying manifests
			nil,   // HostFilter
			false, // IgnoreError
		),
	}
}

func (t *InstallNetworkPluginTask) IsRequired(ctx task.TaskContext) (bool, error) {
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network == nil || clusterCfg.Spec.Network.Plugin == "" {
		ctx.GetLogger().Info("No CNI plugin specified, InstallNetworkPluginTask is not required.")
		return false, nil
	}
	return true, nil
}

func (t *InstallNetworkPluginTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	taskFragment := task.NewExecutionFragment(t.Name())

	clusterCfg := ctx.GetClusterConfig()
	// Ensure NetworkConfig is available (IsRequired should have caught this if plugin is empty)
	if clusterCfg.Spec.Network == nil || clusterCfg.Spec.Network.Plugin == "" {
		logger.Info("No CNI plugin specified in Plan, returning empty fragment.") // Should be caught by IsRequired
		return task.NewEmptyFragment(), nil
	}
	t.NetworkConfig = clusterCfg.Spec.Network // Store for easy access
	pluginName := t.NetworkConfig.Plugin
	logger.Info("Planning CNI plugin installation.", "plugin", pluginName)

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for CNI task: %w", err)
	}

	execNodes, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil || len(execNodes) == 0 {
		logger.Warn("No master nodes found to apply CNI manifest. This task may fail or act unexpectedly if control node lacks kubectl setup.", "plugin", pluginName)
		// Attempting to use control node as a fallback if it's the only option.
		// This assumes kubectl is configured on the control node.
		// A robust implementation might add a step to configure kubectl on control node if needed.
		// For now, if no masters, error out, as applying CNI typically requires cluster access.
		return nil, fmt.Errorf("no master nodes found to apply CNI manifest for plugin %s. This task usually runs on a master.", pluginName)
	}
	kubectlApplyNode := execNodes[0] // Apply CNI from the first master node

	var manifestTemplateContent string
	var templateData interface{}
	localRenderedManifestPath := filepath.Join(ctx.GetGlobalWorkDir(), pluginName+"-rendered-cni.yaml")
	remoteManifestPathOnExecNode := filepath.Join("/tmp", pluginName+"-cni-manifest.yaml")
	var lastLocalStepID plan.NodeID = ""

	// 1. Get/Prepare CNI Manifest (on Control Node)
	switch pluginName {
	case common.CNICalico:
		logger.Info("Preparing Calico CNI manifest.")
		manifestTemplateContent = GetCalicoManifestTemplate()
		calicoAPICfg := t.NetworkConfig.Calico
		if calicoAPICfg == nil { // Ensure CalicoConfig exists if plugin is Calico
			calicoAPICfg = &v1alpha1.CalicoConfig{} // Use defaults if not specified
			// Apply defaults to this local copy for template data
			v1alpha1.SetDefaults_CalicoConfig(calicoAPICfg, t.NetworkConfig.KubePodsCIDR, t.NetworkConfig.IPPool.BlockSize)
		}
		templateData = struct {
			PodCIDR      string
			IPIPMode     string
			VXLANMode    string
			VethMTU      int
			NatOutgoing  bool
			TyphaEnabled bool
			TyphaReplicas int
		}{
			PodCIDR:      t.NetworkConfig.KubePodsCIDR,
			IPIPMode:     calicoAPICfg.IPIPMode,
			VXLANMode:    calicoAPICfg.VXLANMode,
			VethMTU:      0, // Default, will be overridden if VethMTU is not nil
			NatOutgoing:  true, // Default
			TyphaEnabled: false, // Default
			TyphaReplicas: 2,   // Default
		}
		// Access pointer fields safely
		if calicoAPICfg.VethMTU != nil { templateData.(struct{/*...fields...*/ VethMTU int; /*...*/}).VethMTU = *calicoAPICfg.VethMTU }
		if calicoAPICfg.IPv4NatOutgoing != nil { templateData.(struct{/*...fields...*/ NatOutgoing bool; /*...*/}).NatOutgoing = *calicoAPICfg.IPv4NatOutgoing }
		if calicoAPICfg.EnableTypha != nil { templateData.(struct{/*...fields...*/ TyphaEnabled bool; /*...*/}).TyphaEnabled = *calicoAPICfg.EnableTypha }
		if calicoAPICfg.TyphaReplicas != nil { templateData.(struct{/*...fields...*/ TyphaReplicas int}).TyphaReplicas = *calicoAPICfg.TyphaReplicas }


	case common.CNIFlannel:
		logger.Info("Preparing Flannel CNI manifest.")
		manifestTemplateContent = GetFlannelManifestTemplate()
		flannelAPICfg := t.NetworkConfig.Flannel
		if flannelAPICfg == nil {
			flannelAPICfg = &v1alpha1.FlannelConfig{}
			v1alpha1.SetDefaults_FlannelConfig(flannelAPICfg)
		}
		templateData = struct {
			PodCIDR     string
			BackendMode string
		}{
			PodCIDR:     t.NetworkConfig.KubePodsCIDR,
			BackendMode: flannelAPICfg.BackendMode,
		}
	// TODO: Add cases for Cilium, Kube-OVN based on their installation methods (YAML, Helm, Operator)
	default:
		return nil, fmt.Errorf("unsupported CNI plugin '%s' for task %s", pluginName, t.Name())
	}

	// Step 1.1: Render CNI Manifest on Control Node (if templated)
	renderStepName := fmt.Sprintf("Render-%s-CNIManifest", pluginName)
	renderStep := commonstep.NewRenderTemplateStep(
		renderStepName,
		manifestTemplateContent,
		templateData,
		localRenderedManifestPath, // Render to local path on control node
		"0644", false, // Sudo false for local render
	)
	renderNodeID, err := taskFragment.AddNode(&plan.ExecutionNode{
		Name: renderStep.Meta().Name, Step: renderStep, Hosts: []connector.Host{controlNode},
	})
	if err != nil { return nil, fmt.Errorf("failed to add render CNI manifest node: %w", err) }
	lastLocalStepID = renderNodeID

	// Step 2: Upload Manifest to Execution Node (e.g., first master)
	uploadStepName := fmt.Sprintf("Upload-%s-CNIManifest-To-%s", pluginName, kubectlApplyNode.GetName())
	uploadManifestStep := commonstep.NewUploadFileStep(
		uploadStepName,
		localRenderedManifestPath,      // Source from control node
		remoteManifestPathOnExecNode, // Destination on master
		"0644", false, false,         // Sudo false for /tmp on remote
	)
	uploadNodeID, err := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  uploadManifestStep.Meta().Name,
		Step:  uploadManifestStep,
		Hosts: []connector.Host{kubectlApplyNode},
		Dependencies: common.NonEmptyNodeIDs(lastLocalStepID),
	})
	if err != nil { return nil, fmt.Errorf("failed to add upload CNI manifest node: %w", err) }

	// Step 3: Apply Manifest using Kubectl on Execution Node
	kubeconfigPathOnNode := "/etc/kubernetes/admin.conf"
	applyStepName := fmt.Sprintf("Apply-%s-CNIManifest-On-%s", pluginName, kubectlApplyNode.GetName())
	applyManifestStep := kubernetessteps.NewKubectlApplyStep(
		applyStepName,
		remoteManifestPathOnExecNode,
		kubeconfigPathOnNode,
		true, // Sudo for kubectl if kubeconfig is root-only or kubectl needs root
		3, 10, // Retries and delay
	)
	_, err = taskFragment.AddNode(&plan.ExecutionNode{
		Name:  applyManifestStep.Meta().Name,
		Step:  applyManifestStep,
		Hosts: []connector.Host{kubectlApplyNode},
		Dependencies: []plan.NodeID{uploadNodeID},
	})
	if err != nil { return nil, fmt.Errorf("failed to add apply CNI manifest node: %w", err) }

	taskFragment.CalculateEntryAndExitNodes()
	logger.Info("CNI plugin installation task planning complete.", "plugin", pluginName, "entryNodes", taskFragment.EntryNodes, "exitNodes", taskFragment.ExitNodes)
	return taskFragment, nil
}

// GetCalicoManifestTemplate returns the Calico manifest template string.
func GetCalicoManifestTemplate() string {
	return `apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    ipPools:
    - blockSize: 26
      cidr: "{{ .PodCIDR }}"
      encapsulation: "{{ .IPIPMode }}"
      natOutgoing: "{{ .NatOutgoing }}"
      nodeSelector: all()
{{ if .TyphaEnabled }}
  typhaDeployment:
    spec:
      template:
        spec:
          nodeSelector:
            kubernetes.io/os: linux
      replicas: {{ .TyphaReplicas }}
{{ end }}`
}

// GetFlannelManifestTemplate returns the Flannel CNI manifest template string.
func GetFlannelManifestTemplate() string {
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
- apiGroups: ['']
  resources: ['pods']
  verbs: ['get']
- apiGroups: ['']
  resources: ['nodes']
  verbs: ['list', 'watch']
- apiGroups: ['']
  resources: ['nodes/status']
  verbs: ['patch']
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: flannel
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: flannel
subjects:
- kind: ServiceAccount
  name: flannel
  namespace: kube-flannel
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: flannel
  namespace: kube-flannel
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: kube-flannel-cfg
  namespace: kube-flannel
  labels:
    tier: node
    app: flannel
data:
  cni-conf.json: |
    {
      "name": "cbr0",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "flannel",
          "delegate": {
            "hairpinMode": true,
            "isDefaultGateway": true
          }
        },
        {
          "type": "portmap",
          "capabilities": {
            "portMappings": true
          }
        }
      ]
    }
  net-conf.json: |
    {
      "Network": "{{ .PodCIDR }}",
      "Backend": {
        "Type": "{{ .BackendMode | default "vxlan" }}"
      }
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-flannel-ds
  namespace: kube-flannel
  labels:
    tier: node
    app: flannel
spec:
  selector:
    matchLabels:
      app: flannel
  template:
    metadata:
      labels:
        tier: node
        app: flannel
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/os
                operator: In
                values:
                - linux
      hostNetwork: true
      priorityClassName: system-node-critical
      tolerations:
      - operator: Exists
        effect: NoSchedule
      serviceAccountName: flannel
      initContainers:
      - name: install-cni-plugins
        # Using a common CNI plugin image, ensure it contains flannel CNI plugin
        image: docker.io/flannel/flannel-cni-plugin:v1.2.0 # Official image
        command:
        - cp
        args:
        - -f
        - /opt/cni/bin/flannel
        - /opt/cni/bin/flannel
        volumeMounts:
        - name: cni-plugin
          mountPath: /opt/cni/bin
      containers:
      - name: kube-flannel
        image: docker.io/flannel/flannel:v0.24.2 # Official Flannel image
        command:
        - /opt/bin/flanneld
        args:
        - --ip-masq
        - --kube-subnet-mgr
        - --kubeconfig-file=/etc/kube-flannel/kubeconfig # Added kubeconfig file
        - --iface-regex=^e.* # Example: match common ethernet interfaces like eth0, enpXsY
        resources:
          requests:
            cpu: "100m"
            memory: "50Mi"
        securityContext:
          privileged: false
          capabilities:
            add: ["NET_ADMIN", "NET_RAW"]
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        volumeMounts:
        - name: run
          mountPath: /run/flannel
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      volumes:
      - name: run
        hostPath:
          path: /run/flannel
      - name: cni-plugin # For CNI plugins binary
        hostPath:
          path: /opt/cni/bin
      - name: flannel-cfg # For net-conf.json (via ConfigMap) and kubeconfig
        configMap:
          name: kube-flannel-cfg
`
}

var _ task.Task = (*InstallNetworkPluginTask)(nil)
```
