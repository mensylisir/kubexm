package kubernetes

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	stepkube "github.com/mensylisir/kubexm/pkg/step/kube"
	kubernetessteps "github.com/mensylisir/kubexm/pkg/step/kubernetes" // For cache key constants
	"github.com/mensylisir/kubexm/pkg/task"
)

// JoinMastersTask joins additional master nodes to the Kubernetes cluster.
type JoinMastersTask struct {
	task.BaseTask
	// This task relies on join information (token, hash, cert key) being in TaskCache
	// from a previous KubeadmInitStep.
}

// NewJoinMastersTask creates a new JoinMastersTask.
func NewJoinMastersTask() task.Task {
	return &JoinMastersTask{
		BaseTask: task.NewBaseTask(
			"JoinAdditionalMasterNodes",
			"Joins additional master nodes to the Kubernetes cluster using kubeadm.",
			[]string{common.RoleMaster}, // Targets master role
			nil,                         // HostFilter will be applied effectively by logic below
			false,                       // IgnoreError
		),
	}
}

func (t *JoinMastersTask) IsRequired(ctx task.TaskContext) (bool, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	masters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return false, fmt.Errorf("failed to get master nodes for task %s: %w", t.Name(), err)
	}
	if len(masters) <= 1 {
		logger.Info("No additional master nodes to join or no masters defined.")
		return false, nil
	}
	// TODO: Check if cluster is initialized and if these nodes are already part of it.
	logger.Info("Additional master nodes found, join task is required.", "count", len(masters)-1)
	return true, nil
}

func (t *JoinMastersTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	taskFragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	allMasters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return nil, fmt.Errorf("failed to get master nodes for task %s: %w", t.Name(), err)
	}

	if len(allMasters) <= 1 {
		logger.Info("No additional master nodes to join.")
		return task.NewEmptyFragment(), nil
	}

	// Assume first master is already initialized by InitMasterTask
	// Subsequent masters will be joined.
	additionalMasters := allMasters[1:]

	// Retrieve necessary info from cache, put there by KubeadmInitStep (from InitMasterTask)
	token, tokenFound := ctx.GetTaskCache().Get(kubernetessteps.KubeadmTokenCacheKey)
	discoveryHash, hashFound := ctx.GetTaskCache().Get(kubernetessteps.KubeadmDiscoveryHashCacheKey)
	certKey, certKeyFound := ctx.GetTaskCache().Get(kubernetessteps.KubeadmCertificateKeyCacheKey)

	if !tokenFound || !hashFound || !certKeyFound {
		errMsg := "kubeadm join parameters (token, discovery-hash, or certificate-key) not found in cache. InitMasterTask must run first."
		logger.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	controlPlaneEndpoint := fmt.Sprintf("%s:%d", clusterCfg.Spec.ControlPlaneEndpoint.Domain, clusterCfg.Spec.ControlPlaneEndpoint.Port)
	if clusterCfg.Spec.ControlPlaneEndpoint.Domain == "" || clusterCfg.Spec.ControlPlaneEndpoint.Port == 0 {
		return nil, fmt.Errorf("ControlPlaneEndpoint domain or port not configured, cannot join masters")
	}

	var ignorePreflightErrors string
	if clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.SkipPreflight {
		ignorePreflightErrors = "all"
	} else if clusterCfg.Spec.Kubernetes.IgnorePreflightErrors != "" {
		ignorePreflightErrors = clusterCfg.Spec.Kubernetes.IgnorePreflightErrors
	}

	var criSocket string
	if clusterCfg.Spec.Kubernetes.ContainerRuntime != nil {
		switch clusterCfg.Spec.Kubernetes.ContainerRuntime.Type {
		case v1alpha1.ContainerRuntimeContainerd:
			criSocket = common.ContainerdSocketPath
		case v1alpha1.ContainerRuntimeDocker:
			criSocket = common.CriDockerdSocketPath
		}
	}


	for _, masterHost := range additionalMasters {
		logger.Info("Planning to join additional master node.", "host", masterHost.GetName())

		// KubeadmJoinStep constructor:
		// NewKubeadmJoinStep(instanceName string, isControlPlane bool, controlPlaneAddress, token, discoveryHash, certKey, ignoreErrors, criSocket string, sudo, joinCommandFromCache bool)
		joinStep := stepkube.NewKubeadmJoinStep(
			"KubeadmJoinControlPlane-"+masterHost.GetName(),
			true, // IsControlPlane
			controlPlaneEndpoint,
			token.(string),
			discoveryHash.(string),
			certKey.(string),
			ignorePreflightErrors,
			criSocket,
			true,  // Sudo
			false, // Don't rely on full join command from cache, use individual components
		)

		nodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
			Name:  joinStep.Meta().Name,
			Step:  joinStep,
			Hosts: []connector.Host{masterHost},
			// Dependencies: This task implicitly depends on InitMasterTask completing and populating cache.
			// No explicit inter-node dependencies for joining masters themselves, they can join in parallel
			// once the first master is up and token is available.
		})
		// All join steps can be entry points for this fragment as they only depend on cached info.
		taskFragment.EntryNodes = append(taskFragment.EntryNodes, nodeID)
		taskFragment.ExitNodes = append(taskFragment.ExitNodes, nodeID)
	}

	taskFragment.EntryNodes = plan.UniqueNodeIDs(taskFragment.EntryNodes)
	taskFragment.ExitNodes = plan.UniqueNodeIDs(taskFragment.ExitNodes)

	if taskFragment.IsEmpty() {
		logger.Info("JoinMastersTask planned no executable nodes.")
	} else {
		logger.Info("JoinMastersTask planning complete.")
	}
	return taskFragment, nil
}

var _ task.Task = (*JoinMastersTask)(nil)
