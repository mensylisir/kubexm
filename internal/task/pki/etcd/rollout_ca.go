package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/etcd"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	etcdstep "github.com/mensylisir/kubexm/internal/step/pki/etcd"
	"github.com/mensylisir/kubexm/internal/task"
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return false, fmt.Errorf("ctx is not *runtime.Context")
	}
	cacheKey := fmt.Sprintf(common.CacheKubexmEtcdCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	caRenewVal, _ := ctx.GetModuleCache().Get(cacheKey)
	isCARenewal, _ := caRenewVal.(bool)
	if !isCARenewal {
		return false, nil
	}

	logger.Info("Final CA has not been deployed yet. Task is required.")
	return true, nil
}

func (t *DeployFinalCARollingTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx := ctx.ForTask(t.Name())

	fragment := plan.NewExecutionFragment(t.Name())

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return fragment, nil
	}

	var lastNodeWaitID plan.NodeID
	for _, node := range etcdNodes {
		nodeName := node.GetName()

		distCAStep, err := etcdstep.NewDistributeCAStepBuilder(runtimeCtx, fmt.Sprintf("DistributeFinalCA_%s", nodeName)).Build()
		if err != nil {
			return nil, err
		}
		distCANodeID := plan.NodeID(fmt.Sprintf("DistributeFinalCAFor%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(distCANodeID), Step: distCAStep, Hosts: []remotefw.Host{node}})
		if lastNodeWaitID != "" {
			fragment.AddDependency(lastNodeWaitID, distCANodeID)
		}

		restartStep, err := etcdstep.NewRestartEtcdStepBuilder(runtimeCtx, fmt.Sprintf("RestartEtcdForFinalCA_%s", nodeName)).Build()
		if err != nil {
			return nil, err
		}
		restartNodeID := plan.NodeID(fmt.Sprintf("RestartEtcdForFinalCA_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(restartNodeID), Step: restartStep, Hosts: []remotefw.Host{node}})
		fragment.AddDependency(distCANodeID, restartNodeID)

		waitStep, err := etcd.NewWaitClusterHealthyStepBuilder(runtimeCtx, fmt.Sprintf("WaitClusterHealthyForFinalCA_%s", nodeName)).Build()
		if err != nil {
			return nil, err
		}
		waitNodeID := plan.NodeID(fmt.Sprintf("WaitClusterHealthyForFinalCA_%s", nodeName))
		fragment.AddNode(&plan.ExecutionNode{Name: string(waitNodeID), Step: waitStep, Hosts: []remotefw.Host{node}})
		fragment.AddDependency(restartNodeID, waitNodeID)

		lastNodeWaitID = waitNodeID
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*DeployFinalCARollingTask)(nil)
