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

type DeployTrustBundleRollingTask struct {
	task.Base
}

func NewDeployTrustBundleRollingTask() task.Task {
	return &DeployTrustBundleRollingTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployTrustBundleRolling",
				Description: "Deploys the CA trust bundle to the ETCD cluster using a rolling update strategy",
			},
		},
	}
}

func (t *DeployTrustBundleRollingTask) Name() string {
	return t.Meta.Name
}

func (t *DeployTrustBundleRollingTask) Description() string {
	return t.Meta.Description
}

func (t *DeployTrustBundleRollingTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	caRenewVal, _ := ctx.GetModuleCache().Get(common.CacheKubexmEtcdCACertRenew)
	caRenew, _ := caRenewVal.(bool)
	return caRenew, nil
}

func (t *DeployTrustBundleRollingTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return fragment, nil
	}

	prepareBundleStep := etcdstep.NewPrepareCATransitionStepBuilder(*runtimeCtx, "PrepareCATransitionBundle").Build()
	prepareBundleNodeID := plan.NodeID("PrepareCATransitionBundle")
	fragment.AddNode(&plan.ExecutionNode{Name: string(prepareBundleNodeID), Step: prepareBundleStep, Hosts: []connector.Host{controlNode}})

	lastNodeWaitID := prepareBundleNodeID

	for _, node := range etcdNodes {
		nodeName := node.GetName()

		backupStep := etcdstep.NewBackupRemoteEtcdCertsStepBuilder(*runtimeCtx, fmt.Sprintf("BackupRemoteCerts_%s", nodeName)).Build()
		backupNodeID := plan.NodeID(fmt.Sprintf("BackupRemoteCertsForBundle_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(backupNodeID), Step: backupStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(lastNodeWaitID, backupNodeID)

		distBundleStep := etcdstep.NewDistributeCAStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeCABundle_%s", nodeName)).Build()
		distBundleNodeID := plan.NodeID(fmt.Sprintf("DistributeCABundle_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(distBundleNodeID), Step: distBundleStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(backupNodeID, distBundleNodeID)

		restartStep := etcdstep.NewRestartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("RestartEtcd_%s", nodeName)).Build()
		restartNodeID := plan.NodeID(fmt.Sprintf("RestartEtcdForBundle_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(restartNodeID), Step: restartStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(distBundleNodeID, restartNodeID)

		waitStep := etcd.NewWaitClusterHealthyStepBuilder(*runtimeCtx, fmt.Sprintf("WaitClusterHealthy_%s", nodeName)).Build()
		waitNodeID := plan.NodeID(fmt.Sprintf("WaitClusterHealthyForBundle_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(waitNodeID), Step: waitStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(restartNodeID, waitNodeID)

		lastNodeWaitID = waitNodeID
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployTrustBundleRollingTask)(nil)
