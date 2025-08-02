package kubernetes

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InitMasterTask initializes the first Kubernetes master node using kubeadm.
type InitMasterTask struct {
	task.BaseTask
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

func (t *InitMasterTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	masters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return nil, fmt.Errorf("failed to get master nodes for task %s: %w", t.Name(), err)
	}
	if len(masters) == 0 {
		return task.NewEmptyFragment(), nil
	}
	firstMaster := masters[0]

	// 1. Render kubeadm-config.yaml on the first master node
	// The template for this is defined in this file, but should be moved to pkg/templates
	kubeadmConfigTemplate := GetDefaultKubeadmConfigV1Beta3()
	kubeadmConfigData := getKubeadmConfigData(ctx, firstMaster)
	remoteKubeadmConfigPath := "/tmp/kubeadm-config.yaml"

	renderStep := commonstep.NewRenderTemplateStep(
		"RenderKubeadmConfig-"+firstMaster.GetName(),
		kubeadmConfigTemplate,
		kubeadmConfigData,
		remoteKubeadmConfigPath,
		"0640",
		true, // sudo
	)
	renderNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  renderStep.Meta().Name,
		Step:  renderStep,
		Hosts: []connector.Host{firstMaster},
	})

	// 2. Run kubeadm init
	kubeadmInitCmd := fmt.Sprintf("sudo kubeadm init --config %s --upload-certs", remoteKubeadmConfigPath)
	initStep := commonstep.NewCommandStep(
		"KubeadmInit-"+firstMaster.GetName(),
		kubeadmInitCmd,
		true,  // sudo
		false, // ignore error
		0, nil, 0, "", true, 0, "", false, // other params with defaults
	)
	initNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         initStep.Meta().Name,
		Step:         initStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{renderNodeID},
	})

	// 3. Capture join command information from the output of the init command.
	// This is a simplification. A real implementation would need a step that can parse
	// the output of a previous command. For now, we assume a subsequent command can get it.
	// We will create a step to get the token and hash and put them in the cache.
	getTokenCmd := "kubeadm token list | grep 'authentication' | awk '{print $1}'"
	captureTokenStep := commonstep.NewCommandStep(
		"CaptureJoinToken",
		getTokenCmd,
		true, true, 0, nil, 0, common.KubeadmTokenCacheKey, false, 0, "", false,
	)
	captureTokenNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         captureTokenStep.Meta().Name,
		Step:         captureTokenStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{initNodeID},
	})

	getHashCmd := "openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //'"
	captureHashStep := commonstep.NewCommandStep(
		"CaptureDiscoveryHash",
		getHashCmd,
		true, true, 0, nil, 0, common.KubeadmDiscoveryHashCacheKey, false, 0, "", false,
	)
	captureHashNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         captureHashStep.Meta().Name,
		Step:         captureHashStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{initNodeID},
	})

	getCertKeyCmd := "sudo kubeadm init phase upload-certs --upload-certs | tail -1"
	captureCertKeyStep := commonstep.NewCommandStep(
		"CaptureCertKey",
		getCertKeyCmd,
		true, true, 0, nil, 0, common.KubeadmCertificateKeyCacheKey, false, 0, "", false,
	)
	captureCertKeyNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         captureCertKeyStep.Meta().Name,
		Step:         captureCertKeyStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{initNodeID},
	})

	// 4. Setup kubeconfig for the user on the master node.
	userHomeSetupCmd := `mkdir -p $HOME/.kube && sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config`
	userKubeconfigStep := commonstep.NewCommandStep(
		"SetupUserKubeconfig-"+firstMaster.GetName(),
		userHomeSetupCmd, true, false, 0, nil, 0, "", false, 0, "", false)

	userKubeconfigNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         userKubeconfigStep.Meta().Name,
		Step:         userKubeconfigStep,
		Hosts:        []connector.Host{firstMaster},
		Dependencies: []plan.NodeID{initNodeID},
	})

	fragment.EntryNodes = []plan.NodeID{renderNodeID}
	fragment.ExitNodes = []plan.NodeID{captureTokenNodeID, captureHashNodeID, captureCertKeyNodeID, userKubeconfigNodeID}

	logger.Info("InitMasterTask planning complete.")
	return fragment, nil
}

type KubeadmConfigData struct {
	ClusterName          string
	KubernetesVersion    string
	ControlPlaneEndpoint string
	PodSubnet            string
	ServiceSubnet        string
	ImageRepository      string
	NodeName             string
	CriSocket            string
}

func getKubeadmConfigData(ctx task.TaskContext, masterHost connector.Host) KubeadmConfigData {
	clusterCfg := ctx.GetClusterConfig()
	data := KubeadmConfigData{
		ClusterName:          clusterCfg.Name,
		KubernetesVersion:    clusterCfg.Spec.Kubernetes.Version,
		ControlPlaneEndpoint: fmt.Sprintf("%s:%d", clusterCfg.Spec.ControlPlaneEndpoint.Domain, clusterCfg.Spec.ControlPlaneEndpoint.Port),
		PodSubnet:            clusterCfg.Spec.Network.KubePodsCIDR,
		ServiceSubnet:        clusterCfg.Spec.Network.KubeServiceCIDR,
		ImageRepository:      common.DefaultImageRegistry,
		NodeName:             masterHost.GetName(),
		CriSocket:            common.ContainerdSocketPath, // Assuming containerd
	}
	if clusterCfg.Spec.Registry.Mirrors != nil && len(clusterCfg.Spec.Registry.Mirrors) > 0 {
		data.ImageRepository = clusterCfg.Spec.Registry.Mirrors[0]
	}
	return data
}

func GetDefaultKubeadmConfigV1Beta3() string {
	return `apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: {{ .NodeName }}
  bindPort: 6443
nodeRegistration:
  name: {{ .NodeName }}
  criSocket: {{ .CriSocket }}
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
clusterName: {{ .ClusterName }}
kubernetesVersion: {{ .KubernetesVersion }}
controlPlaneEndpoint: "{{ .ControlPlaneEndpoint }}"
imageRepository: {{ .ImageRepository }}
networking:
  podSubnet: {{ .PodSubnet }}
  serviceSubnet: {{ .ServiceSubnet }}
`
}

var _ task.Task = (*InitMasterTask)(nil)
