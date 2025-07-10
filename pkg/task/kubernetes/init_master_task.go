package kubernetes

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes" // For NewKubeadmInitStep
	"github.com/mensylisir/kubexm/pkg/task"
)

// InitMasterTask initializes the first Kubernetes master node using kubeadm.
type InitMasterTask struct {
	task.BaseTask
	// No specific fields needed here if all config comes from ClusterConfig via context
}

// NewInitMasterTask creates a new InitMasterTask.
func NewInitMasterTask() task.Task {
	return &InitMasterTask{
		BaseTask: task.NewBaseTask(
			"InitializeFirstMasterNode",
			"Initializes the first Kubernetes master node using kubeadm.",
			[]string{common.RoleMaster},
			nil,
			false,
		),
	}
}

func (t *InitMasterTask) IsRequired(ctx task.TaskContext) (bool, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	// Required if setting up a new cluster.
	// This task should only target ONE master. The module/pipeline should ensure it's
	// only planned if a first master needs initialization.
	// A more robust check could involve seeing if the cluster already has an API server.
	// For now, if master nodes exist, assume init is needed by one of them if cluster isn't up.
	masters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return false, fmt.Errorf("failed to get master nodes for task %s: %w", t.Name(), err)
	}
	if len(masters) == 0 {
		logger.Info("No master nodes found, skipping first master initialization.")
		return false, nil
	}
	// TODO: Add a check if cluster is already initialized (e.g. by checking a well-known key in PipelineCache)
	logger.Info("First master initialization is required.")
	return true, nil
}

// KubeadmConfigData holds data for the kubeadm-config.yaml template.
type KubeadmConfigData struct {
	ClusterName               string
	KubernetesVersion         string
	ControlPlaneEndpoint      string // "hostname:port"
	PodSubnet                 string
	ServiceSubnet             string
	ImageRepository           string // e.g., registry.k8s.io
	UseSystemdCgroup          bool
	EtcdEndpoints             []string // For external etcd
	EtcdCAFile                string
	EtcdCertFile              string
	EtcdKeyFile               string
	IgnorePreflightErrors     []string
	APIServerCertExtraSANs    []string
	NodeName                  string // Name of the current node
	CriSocket                 string // e.g., /var/run/containerd/containerd.sock or /var/run/cri-dockerd.sock
}

func (t *InitMasterTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	taskFragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	masters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return nil, fmt.Errorf("failed to get master nodes for task %s: %w", t.Name(), err)
	}
	if len(masters) == 0 {
		logger.Info("No master nodes found. Cannot plan InitMasterTask.")
		return task.NewEmptyFragment(), nil
	}
	firstMaster := masters[0] // Target only the first master for kubeadm init

	logger.Info("Planning kubeadm init on first master.", "host", firstMaster.GetName())

	// 1. Prepare Kubeadm Config Data
	kubeadmConfigData := KubeadmConfigData{
		ClusterName:          clusterCfg.Spec.Kubernetes.ClusterName,
		KubernetesVersion:    strings.TrimPrefix(clusterCfg.Spec.Kubernetes.Version, "v"),
		ControlPlaneEndpoint: fmt.Sprintf("%s:%d", clusterCfg.Spec.ControlPlaneEndpoint.Domain, clusterCfg.Spec.ControlPlaneEndpoint.Port),
		PodSubnet:            clusterCfg.Spec.Network.KubePodsCIDR,
		ServiceSubnet:        clusterCfg.Spec.Network.KubeServiceCIDR,
		ImageRepository:      common.DefaultImageRegistry, // TODO: Make this configurable from clusterCfg.Spec.Registry
		UseSystemdCgroup:     true, // TODO: Make this configurable or derive from runtime facts/config
		NodeName:             firstMaster.GetName(),
		APIServerCertExtraSANs: clusterCfg.Spec.Kubernetes.ApiserverCertExtraSans,
	}
	// Handle IgnorePreflightErrors (string to []string)
	if clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.SkipPreflight {
		kubeadmConfigData.IgnorePreflightErrors = []string{"all"}
	} else if clusterCfg.Spec.Kubernetes.IgnorePreflightErrors != "" { // Assuming this field exists on KubernetesConfig
		kubeadmConfigData.IgnorePreflightErrors = strings.Split(clusterCfg.Spec.Kubernetes.IgnorePreflightErrors, ",")
	}


	// Determine CRI socket based on configured container runtime
	// This requires access to host facts for the firstMaster or the global runtime config.
	// For simplicity, let's assume it's passed or defaulted.
	// This logic should ideally be more robust, checking facts for the specific node.
	if clusterCfg.Spec.Kubernetes.ContainerRuntime != nil {
		switch clusterCfg.Spec.Kubernetes.ContainerRuntime.Type {
		case v1alpha1.ContainerRuntimeContainerd:
			kubeadmConfigData.CriSocket = common.ContainerdSocketPath
		case v1alpha1.ContainerRuntimeDocker:
			kubeadmConfigData.CriSocket = common.CriDockerdSocketPath // Docker uses cri-dockerd
		default:
			// Default or error if unknown. For now, leave empty and let kubeadm detect or use its default.
		}
	}


	// Handle Etcd configuration for kubeadm
	if clusterCfg.Spec.Etcd.Type == v1alpha1.EtcdTypeExternal && clusterCfg.Spec.Etcd.External != nil {
		kubeadmConfigData.EtcdEndpoints = clusterCfg.Spec.Etcd.External.Endpoints
		kubeadmConfigData.EtcdCAFile = clusterCfg.Spec.Etcd.External.CAFile   // Path on the master node
		kubeadmConfigData.EtcdCertFile = clusterCfg.Spec.Etcd.External.CertFile // Path on the master node
		kubeadmConfigData.EtcdKeyFile = clusterCfg.Spec.Etcd.External.KeyFile   // Path on the master node
		// These paths need to be the paths *on the master node* where certs have been uploaded.
		// This implies a prior step uploaded these if they are external and needed by kubeadm.
		// If etcd is stacked (managed by kubeadm), these are not set.
	}


	// 2. Render kubeadm-config.yaml on the first master node
	// TODO: kubeadmConfigTemplate needs to be defined.
	kubeadmConfigTemplate := GetDefaultKubeadmConfigV1Beta3() // Placeholder for actual template

	remoteKubeadmConfigPath := "/tmp/kubeadm-config.yaml" // Path on the master node
	renderKubeadmCfgStep := commonstep.NewRenderTemplateStep(
		"RenderKubeadmConfig-"+firstMaster.GetName(),
		kubeadmConfigTemplate,
		kubeadmConfigData,
		remoteKubeadmConfigPath,
		"0640", // Permissions
		true,   // Sudo for writing to /tmp might not be needed, but for /etc/kubernetes yes.
	)
	renderNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:  renderKubeadmCfgStep.Meta().Name,
		Step:  renderKubeadmCfgStep,
		Hosts: []connector.Host{firstMaster},
	})

	// 3. Run KubeadmInitStep
	// The IgnorePreflightErrors for the step itself is different from the ones in the config file.
	// The ones in the config file are for kubeadm internal checks. The step's one is for the command.
	// For now, let's use the one from kubeadmConfigData.
	var ignoreErrorsForCmd string
	if len(kubeadmConfigData.IgnorePreflightErrors) > 0 {
		ignoreErrorsForCmd = strings.Join(kubeadmConfigData.IgnorePreflightErrors, ",")
	}

	kubeadmInitStep := kubernetessteps.NewKubeadmInitStep(
		"KubeadmInit-"+firstMaster.GetName(),
		remoteKubeadmConfigPath,
		ignoreErrorsForCmd,
		true, // Sudo
	)
	initNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:         kubeadmInitStep.Meta().Name,
		Step:         kubeadmInitStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{renderNodeID},
	})

	// 4. Post-init: Copy kubeconfig for user, and to control node for remote kubectl
	userHomeSetupCmd := `mkdir -p $HOME/.kube && sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config`
	userKubeconfigStep := commonstep.NewCommandStep(
		"SetupUserKubeconfig-"+firstMaster.GetName(),
		userHomeSetupCmd, true, false, 0, nil, 0, "", false, 0, "", false)

	userKubeconfigNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
		Name:         userKubeconfigStep.Meta().Name,
		Step:         userKubeconfigStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{initNodeID},
	})

	// TODO: Add step to download /etc/kubernetes/admin.conf from firstMaster to control node's GlobalWorkDir.
	// This would require a DownloadFromRemoteStep or similar.
	// For now, this step ensures it's available for the user on the master node.

	taskFragment.CalculateEntryAndExitNodes()
	logger.Info("InitMasterTask planning complete.", "entryNodes", taskFragment.EntryNodes, "exitNodes", taskFragment.ExitNodes)
	return taskFragment, nil
}

// GetDefaultKubeadmConfigV1Beta3 returns a basic kubeadm config template string.
// TODO: This should be externalized or made more robust.
func GetDefaultKubeadmConfigV1Beta3() string {
	return `apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: {{ .NodeName }} # This should be the node's primary IP, kubeadm might pick it. Or pass explicitly.
  bindPort: 6443
nodeRegistration:
  name: {{ .NodeName }}
  criSocket: {{ .CriSocket | default "/var/run/containerd/containerd.sock" }}
  kubeletExtraArgs:
    cgroup-driver: "{{ if .UseSystemdCgroup }}systemd{{ else }}cgroupfs{{ end }}"
    housekeeping-interval: "10s"
{{- if .IgnorePreflightErrors }}
  ignorePreflightErrors:
  {{- range .IgnorePreflightErrors }}
  - {{ . }}
  {{- end }}
{{- end }}
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
clusterName: {{ .ClusterName | default "kubernetes" }}
kubernetesVersion: {{ .KubernetesVersion }}
controlPlaneEndpoint: "{{ .ControlPlaneEndpoint }}" # e.g. "lb.example.com:6443"
{{- if .ImageRepository }}
imageRepository: {{ .ImageRepository }}
{{- end }}
networking:
  podSubnet: {{ .PodSubnet }}
  serviceSubnet: {{ .ServiceSubnet }}
  dnsDomain: cluster.local # Default, can be configured
{{- if .EtcdEndpoints }}
etcd:
  external:
    endpoints:
    {{- range .EtcdEndpoints }}
    - {{ . }}
    {{- end }}
    caFile: {{ .EtcdCAFile }}
    certFile: {{ .EtcdCertFile }}
    keyFile: {{ .EtcdKeyFile }}
{{- end }}
{{- if .APIServerCertExtraSANs }}
apiServer:
  certSANs:
  {{- range .APIServerCertExtraSANs }}
  - {{ . }}
  {{- end }}
{{- end }}
# featureGates:
#   SomeFeature: true
`
}


var _ task.Task = (*InitMasterTask)(nil)
