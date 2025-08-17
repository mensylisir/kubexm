package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployFinalCARollingTask struct {
	task.Base
}

func NewDeployFinalCARollingTask() task.Task {
	return &DeployFinalCARollingTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployFinalCARolling",
				Description: "Deploys the final, new CA-only certificate via rolling update to complete the CA rotation",
			},
		},
	}
}

func (t *DeployFinalCARollingTask) Name() string {
	return t.Meta.Name
}

func (t *DeployFinalCARollingTask) Description() string {
	return t.Meta.Description
}

func (t *DeployFinalCARollingTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "IsRequired")

	caRenewVal, _ := ctx.GetModuleCache().Get(etcdstep.CacheKeyCARequiresRenewal)
	isCARenewal, _ := caRenewVal.(bool)
	if !isCARenewal {
		return false, nil
	}

	logger.Info("Final CA has not been deployed yet. Task is required.")
	return true, nil
}

func (t *DeployFinalCARollingTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context in Plan method")
	}

	fragment := plan.NewExecutionFragment(t.Name())

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return fragment, nil
	}

	var lastNodeWaitID plan.NodeID
	for _, node := range etcdNodes {
		nodeName := node.GetName()

		distCAStep := etcdstep.NewDistributeCAStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeFinalCA_%s", nodeName)).Build()
		distCANodeID := plan.NodeID(fmt.Sprintf("DistributeFinalCAFor%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(distCANodeID), Step: distCAStep, Hosts: []connector.Host{node}})
		if lastNodeWaitID != "" {
			fragment.AddDependency(lastNodeWaitID, distCANodeID)
		}

		restartStep := etcdstep.NewRestartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("RestartEtcdForFinalCA_%s", nodeName)).Build()
		restartNodeID := plan.NodeID(fmt.Sprintf("RestartEtcdForFinalCA_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(restartNodeID), Step: restartStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(distCANodeID, restartNodeID)

		waitStep := etcdstep.NewWaitClusterHealthyStepBuilder(*runtimeCtx, fmt.Sprintf("WaitClusterHealthyForFinalCA_%s", nodeName)).Build()
		waitNodeID := plan.NodeID(fmt.Sprintf("WaitClusterHealthyForFinalCA_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(waitNodeID), Step: waitStep, Hosts: []connector.Host{node}})
		fragment.AddDependency(restartNodeID, waitNodeID)

		lastNodeWaitID = waitNodeID
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*DeployFinalCARollingTask)(nil)
