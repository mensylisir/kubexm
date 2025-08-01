package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// JoinWorkerNodesTask joins worker nodes to the cluster.
type JoinWorkerNodesTask struct {
	task.BaseTask
}

// NewJoinWorkerNodesTask creates a new JoinWorkerNodesTask.
func NewJoinWorkerNodesTask() task.Task {
	return &JoinWorkerNodesTask{
		BaseTask: task.NewBaseTask(
			"JoinWorkerNodes",
			"Joins worker nodes to the Kubernetes cluster.",
			[]string{common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *JoinWorkerNodesTask) IsRequired(ctx task.TaskContext) (bool, error) {
	workers, err := ctx.GetHostsByRole(common.RoleWorker)
	if err != nil {
		return false, err
	}
	return len(workers) > 0, nil
}

func (t *JoinWorkerNodesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	// Retrieve join info from cache
	token, tokenFound := ctx.GetTaskCache().Get(common.KubeadmTokenCacheKey)
	discoveryHash, hashFound := ctx.GetTaskCache().Get(common.KubeadmDiscoveryHashCacheKey)

	if !tokenFound || !hashFound {
		return nil, fmt.Errorf("kubeadm token or discovery hash not found in cache. InitFirstMasterTask must run first")
	}

	joinCmd := fmt.Sprintf("sudo kubeadm join %s:%d --token %s --discovery-token-ca-cert-hash %s",
		clusterCfg.Spec.ControlPlaneEndpoint.Domain,
		clusterCfg.Spec.ControlPlaneEndpoint.Port,
		token,
		discoveryHash,
	)

	targetHosts, err := ctx.GetHostsByRole(common.RoleWorker)
	if err != nil {
		return nil, err
	}

	if len(targetHosts) == 0 {
		logger.Info("No worker nodes to join.")
		return task.NewEmptyFragment(), nil
	}

	var allJoinNodes []plan.NodeID
	for _, host := range targetHosts {
		step := commonstep.NewCommandStep(
			fmt.Sprintf("KubeadmJoinWorker-%s", host.GetName()),
			joinCmd,
			true, // sudo
			false,
			0, nil, 0, "", false, 0, "", false,
		)
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  step.Meta().Name,
			Step:  step,
			Hosts: []connector.Host{host},
		})
		allJoinNodes = append(allJoinNodes, nodeID)
	}

	fragment.EntryNodes = allJoinNodes
	fragment.ExitNodes = allJoinNodes

	logger.Info("Worker node join task planning complete.")
	return fragment, nil
}

var _ task.Task = (*JoinWorkerNodesTask)(nil)
