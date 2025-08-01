package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// JoinControlPlaneNodesTask joins additional master nodes to the cluster.
type JoinControlPlaneNodesTask struct {
	task.BaseTask
}

// NewJoinControlPlaneNodesTask creates a new JoinControlPlaneNodesTask.
func NewJoinControlPlaneNodesTask() task.Task {
	return &JoinControlPlaneNodesTask{
		BaseTask: task.NewBaseTask(
			"JoinControlPlaneNodes",
			"Joins additional control plane nodes to the Kubernetes cluster.",
			[]string{common.RoleMaster},
			nil,
			false,
		),
	}
}

func (t *JoinControlPlaneNodesTask) IsRequired(ctx task.TaskContext) (bool, error) {
	masters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return false, err
	}
	// This task is required if there is more than one master node.
	return len(masters) > 1, nil
}

func (t *JoinControlPlaneNodesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	clusterCfg := ctx.GetClusterConfig()

	// Retrieve join info from cache, which should have been populated by InitFirstMasterTask
	token, tokenFound := ctx.GetTaskCache().Get(common.KubeadmTokenCacheKey)
	discoveryHash, hashFound := ctx.GetTaskCache().Get(common.KubeadmDiscoveryHashCacheKey)
	certKey, certKeyFound := ctx.GetTaskCache().Get(common.KubeadmCertificateKeyCacheKey)

	if !tokenFound || !hashFound || !certKeyFound {
		return nil, fmt.Errorf("kubeadm join parameters (token, hash, or cert key) not found in cache. InitFirstMasterTask must run first")
	}

	joinCmd := fmt.Sprintf("sudo kubeadm join %s:%d --token %s --discovery-token-ca-cert-hash %s --control-plane --certificate-key %s",
		clusterCfg.Spec.ControlPlaneEndpoint.Domain,
		clusterCfg.Spec.ControlPlaneEndpoint.Port,
		token,
		discoveryHash,
		certKey,
	)

	// This task should run on all master nodes except the first one.
	allMasters, err := ctx.GetHostsByRole(common.RoleMaster)
	if err != nil {
		return nil, err
	}
	if len(allMasters) <= 1 {
		logger.Info("No additional control plane nodes to join.")
		return task.NewEmptyFragment(), nil
	}
	targetHosts := allMasters[1:]

	var allJoinNodes []plan.NodeID
	for _, host := range targetHosts {
		step := commonstep.NewCommandStep(
			fmt.Sprintf("KubeadmJoinControlPlane-%s", host.GetName()),
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

	logger.Info("Control plane join task planning complete.")
	return fragment, nil
}

var _ task.Task = (*JoinControlPlaneNodesTask)(nil)
