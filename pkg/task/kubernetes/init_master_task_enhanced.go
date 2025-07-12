package kubernetes

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/command"
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InitMasterTask initializes the first Kubernetes master node.
type InitMasterTask struct {
	name        string
	description string
}

// NewInitMasterTask creates a new InitMasterTask.
func NewInitMasterTask() task.Task {
	return &InitMasterTask{
		name:        "InitMaster",
		description: "Initializes the first Kubernetes master node with kubeadm",
	}
}

// Name returns the task name.
func (t *InitMasterTask) Name() string {
	return t.name
}

// Description returns the task description.
func (t *InitMasterTask) Description() string {
	return t.description
}

// IsRequired determines if master initialization is needed.
func (t *InitMasterTask) IsRequired(ctx task.TaskContext) (bool, error) {
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig == nil {
		return false, fmt.Errorf("cluster config is nil")
	}

	// Check if we have master nodes to initialize
	masterNodes, err := ctx.GetHostsByRole("master")
	if err != nil {
		return false, fmt.Errorf("failed to get master nodes: %w", err)
	}

	return len(masterNodes) > 0, nil
}

// Plan generates an execution fragment for initializing the master node.
func (t *InitMasterTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	fragment := task.NewExecutionFragment("init-master")
	logger := ctx.GetLogger().With("task", t.Name())

	clusterConfig := ctx.GetClusterConfig()

	// Get the first master node (primary)
	masterNodes, err := ctx.GetHostsByRole("master")
	if err != nil {
		return nil, fmt.Errorf("failed to get master nodes: %w", err)
	}

	if len(masterNodes) == 0 {
		logger.Warn("No master nodes found")
		return task.NewEmptyFragment("init-master-empty"), nil
	}

	primaryMaster := masterNodes[0]
	logger.Info("Planning master initialization", "primary_master", primaryMaster.GetName())

	// 1. Render kubeadm config
	configNodeID := plan.NodeID("render-kubeadm-config")
	configData := map[string]interface{}{
		"ClusterName":          clusterConfig.Name,
		"KubernetesVersion":   clusterConfig.Spec.Kubernetes.Version,
		"ControlPlaneEndpoint": clusterConfig.Spec.ControlPlaneEndpoint,
		"PodSubnet":           clusterConfig.Spec.Network.PodSubnet,
		"ServiceSubnet":       clusterConfig.Spec.Network.ServiceSubnet,
		"Etcd":                clusterConfig.Spec.Etcd,
		"APIServer":           clusterConfig.Spec.Kubernetes.APIServer,
		"ControllerManager":   clusterConfig.Spec.Kubernetes.ControllerManager,
		"Scheduler":           clusterConfig.Spec.Kubernetes.Scheduler,
	}

	configStep := common.NewRenderTemplateStep(
		"render-kubeadm-config",
		"kubeadm/kubeadm-init.yaml.tmpl",
		configData,
		"/tmp/kubeadm-config.yaml",
		"0644",
		false, // no sudo needed for /tmp
	)

	configNode := &plan.ExecutionNode{
		Name:         "Render kubeadm config",
		Step:         configStep,
		Hosts:        []connector.Host{primaryMaster},
		Dependencies: []plan.NodeID{},
		StepName:     configStep.Meta().Name,
	}

	_, err = fragment.AddNode(configNode, configNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to add config node: %w", err)
	}

	// 2. Initialize the cluster with kubeadm
	initNodeID := plan.NodeID("kubeadm-init")
	initStep := kubernetes.NewKubeadmInitStep(
		"kubeadm-init-master",
		"/tmp/kubeadm-config.yaml",
		"", // ignore preflight errors (could be configurable)
		true, // sudo
	)

	initNode := &plan.ExecutionNode{
		Name:         "Initialize Kubernetes cluster",
		Step:         initStep,
		Hosts:        []connector.Host{primaryMaster},
		Dependencies: []plan.NodeID{configNodeID},
		StepName:     initStep.Meta().Name,
	}

	_, err = fragment.AddNode(initNode, initNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to add init node: %w", err)
	}

	// 3. Wait for API server to become ready
	waitNodeID := plan.NodeID("wait-apiserver")
	apiServerEndpoint := fmt.Sprintf("https://%s", clusterConfig.Spec.ControlPlaneEndpoint)
	if clusterConfig.Spec.ControlPlaneEndpoint == "" {
		// Fallback to primary master address with default port
		apiServerEndpoint = fmt.Sprintf("https://%s:6443", primaryMaster.GetAddress())
	}

	waitStep := kubernetes.NewAPIServerReadyStep(
		"wait-apiserver-ready",
		apiServerEndpoint,
		5*time.Minute,
	)

	waitNode := &plan.ExecutionNode{
		Name:         "Wait for API server to be ready",
		Step:         waitStep,
		Hosts:        []connector.Host{primaryMaster},
		Dependencies: []plan.NodeID{initNodeID},
		StepName:     waitStep.Meta().Name,
	}

	_, err = fragment.AddNode(waitNode, waitNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to add wait node: %w", err)
	}

	// 4. Setup kubeconfig for admin user
	kubeconfigNodeID := plan.NodeID("setup-kubeconfig")
	kubeconfigStep := command.NewCommandStep(
		"setup-kubeconfig",
		`mkdir -p $HOME/.kube && cp -i /etc/kubernetes/admin.conf $HOME/.kube/config && chown $(id -u):$(id -g) $HOME/.kube/config`,
		false, // no sudo - run as regular user
		false, // don't ignore errors
		0,     // default timeout
		nil,   // no env vars
		0,     // expect exit code 0
		"test -f $HOME/.kube/config", // check command
		false, // check without sudo
		0,     // expect check exit code 0
		"rm -f $HOME/.kube/config", // rollback
		false, // rollback without sudo
	)

	kubeconfigNode := &plan.ExecutionNode{
		Name:         "Setup kubeconfig for admin user",
		Step:         kubeconfigStep,
		Hosts:        []connector.Host{primaryMaster},
		Dependencies: []plan.NodeID{waitNodeID},
		StepName:     kubeconfigStep.Meta().Name,
	}

	_, err = fragment.AddNode(kubeconfigNode, kubeconfigNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to add kubeconfig node: %w", err)
	}

	// Set entry and exit nodes
	fragment.EntryNodes = []plan.NodeID{configNodeID}
	fragment.ExitNodes = []plan.NodeID{kubeconfigNodeID}

	logger.Info("Master initialization plan created", "nodes", len(fragment.Nodes))
	return fragment, nil
}

var _ task.Task = (*InitMasterTask)(nil)