package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/etcd"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployNewLeafCertsRollingTask struct {
	task.Base
}

func NewDeployNewLeafCertsRollingTask() task.Task {
	return &DeployNewLeafCertsRollingTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNewLeafCertsRolling",
				Description: "Activates and deploys new leaf certificates to the ETCD cluster using a rolling update strategy",
			},
		},
	}
}

func (t *DeployNewLeafCertsRollingTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNewLeafCertsRollingTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNewLeafCertsRollingTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	caRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyCARequiresRenewal)
	caRenew, _ := caRenewVal.(bool)
	leafRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyLeafRequiresRenewal)
	leafRenew, _ := leafRenewVal.(bool)

	return caRenew || leafRenew, nil
}

func (t *DeployNewLeafCertsRollingTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context in Plan method")
	}

	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return fragment, nil
	}

	caRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyCARequiresRenewal)
	isCARenewal, _ := caRenewVal.(bool)

	var lastNodeWaitID plan.NodeID

	if isCARenewal {
		moveAssetsStep := etcdstep.NewMoveNewAssetsStepBuilder(*runtimeCtx, "MoveNewAssetsToMainDir").Build()
		moveAssetsNodeID := plan.NodeID("MoveNewAssets")
		fragment.AddNode(&plan.ExecutionNode{Name: string(moveAssetsNodeID), Step: moveAssetsStep, Hosts: []connector.Host{controlNode}})
		lastNodeWaitID = moveAssetsNodeID
	}

	for _, node := range etcdNodes {
		nodeName := node.GetName()

		backupStep := etcdstep.NewBackupRemoteEtcdCertsStepBuilder(*runtimeCtx, fmt.Sprintf("BackupRemoteCerts_%s", nodeName)).Build()
		backupNodeID := plan.NodeID(fmt.Sprintf("BackupRemoteCertsForLeafs_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(backupNodeID), Step: backupStep, Hosts: []connector.Host{node}})
		if lastNodeWaitID != "" {
			fragment.AddDependency(lastNodeWaitID, backupNodeID)
		}

		distLeafsStep := etcdstep.NewDistributeLeafCertsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeLeafCerts_%s", nodeName)).Build()
		distLeafsNodeID := plan.NodeID(fmt.Sprintf("DistributeLeafCerts_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(distLeafsNodeID), Step: distLeafsStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(backupNodeID, distLeafsNodeID)

		restartStep := etcdstep.NewRestartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("RestartEtcd_%s", nodeName)).Build()
		restartNodeID := plan.NodeID(fmt.Sprintf("RestartEtcdForLeafs_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(restartNodeID), Step: restartStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(distLeafsNodeID, restartNodeID)

		waitStep := etcd.NewWaitClusterHealthyStepBuilder(*runtimeCtx, fmt.Sprintf("WaitClusterHealthy_%s", nodeName)).Build()
		waitNodeID := plan.NodeID(fmt.Sprintf("WaitClusterHealthyForLeafs_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(waitNodeID), Step: waitStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(restartNodeID, waitNodeID)

		if lastNodeWaitID == "" {
			lastNodeWaitID = waitNodeID
		} else {
			lastNodeWaitID = waitNodeID
		}
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*DeployNewLeafCertsRollingTask)(nil)
