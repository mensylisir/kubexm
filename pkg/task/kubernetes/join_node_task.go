package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// JoinNodeTask joins a node to the Kubernetes cluster.
type JoinNodeTask struct {
	task.BaseTask
	IsControlPlane bool
}

// NewJoinNodeTask creates a new JoinNodeTask.
func NewJoinNodeTask(isControlPlane bool) task.Task {
	var taskName, taskDesc string
	var runOnRoles []string

	if isControlPlane {
		taskName = "JoinControlPlaneNode"
		taskDesc = "Joins a control plane node to the Kubernetes cluster."
		runOnRoles = []string{common.RoleMaster}
	} else {
		taskName = "JoinWorkerNode"
		taskDesc = "Joins a worker node to the Kubernetes cluster."
		runOnRoles = []string{common.RoleWorker}
	}

	return &JoinNodeTask{
		BaseTask: task.NewBaseTask(
			taskName,
			taskDesc,
			runOnRoles,
			nil,
			false,
		),
		IsControlPlane: isControlPlane,
	}
}

func (t *JoinNodeTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	// Retrieve join info from cache
	token, tokenFound := ctx.GetTaskCache().Get(common.KubeadmTokenCacheKey)
	discoveryHash, hashFound := ctx.GetTaskCache().Get(common.KubeadmDiscoveryHashCacheKey)
	certKey, certKeyFound := ctx.GetTaskCache().Get(common.KubeadmCertificateKeyCacheKey)

	if !tokenFound || !hashFound {
		return nil, fmt.Errorf("kubeadm token or discovery hash not found in cache. InitMasterTask must run first")
	}

	joinCmd := fmt.Sprintf("sudo kubeadm join %s:%d --token %s --discovery-token-ca-cert-hash %s",
		clusterCfg.Spec.ControlPlaneEndpoint.Domain,
		clusterCfg.Spec.ControlPlaneEndpoint.Port,
		token,
		discoveryHash,
	)

	if t.IsControlPlane {
		if !certKeyFound {
			return nil, fmt.Errorf("certificate key not found in cache, required for joining a control plane node")
		}
		joinCmd += fmt.Sprintf(" --control-plane --certificate-key %s", certKey)
	}

	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, err
	}

	// For masters, we skip the first one which was the init node.
	if t.IsControlPlane && len(targetHosts) > 0 {
		targetHosts = targetHosts[1:]
	}

	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for this join task, skipping.")
		return task.NewEmptyFragment(), nil
	}


	var allJoinNodes []plan.NodeID
	for _, host := range targetHosts {
		step := commonstep.NewCommandStep(
			fmt.Sprintf("KubeadmJoin-%s", host.GetName()),
			joinCmd,
			true, // sudo
			false,
			0, nil, 0, "", false, 0, "", false,
		)
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  step.Meta().Name,
			Step:  step,
			Hosts: []connector.Host{host},
			// This task implicitly depends on the output of InitMasterTask.
			// An explicit dependency could be added if InitMasterTask produced a cache key that this task's IsRequired checks.
		})
		allJoinNodes = append(allJoinNodes, nodeID)
	}

	fragment.EntryNodes = allJoinNodes
	fragment.ExitNodes = allJoinNodes

	logger.Info("Node join task planning complete.")
	return fragment, nil
}

var _ task.Task = (*JoinNodeTask)(nil)
