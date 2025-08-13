package calico

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/calico"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanCalicoTask struct {
	task.Base
}

func NewCleanCalicoTask() task.Task {
	return &CleanCalicoTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanCalico",
				Description: "Uninstall Calico CNI network addon and cleanup related resources",
			},
		},
	}
}

func (t *CleanCalicoTask) Name() string {
	return t.Meta.Name
}

func (t *CleanCalicoTask) Description() string {
	return t.Meta.Description
}

func (t *CleanCalicoTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCalico), nil
}

func (t *CleanCalicoTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanCalicoStep := calico.NewCleanCalicoStepBuilder(*runtimeCtx, "RemoveCalicoResources").Build()
	removeCalicoctlStep := calico.NewRemoveCalicoctlStepBuilder(*runtimeCtx, "RemoveCalicoctlBinary").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalicoResources", Step: cleanCalicoStep, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalicoctlBinary", Step: removeCalicoctlStep, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
