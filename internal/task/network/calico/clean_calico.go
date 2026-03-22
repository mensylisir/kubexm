package calico

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/calico"
	"github.com/mensylisir/kubexm/internal/task"
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
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanCalicoStep, err := calico.NewCleanCalicoStepBuilder(runtimeCtx, "RemoveCalicoResources").Build()
	if err != nil {
		return nil, err
	}
	removeCalicoctlStep, err := calico.NewRemoveCalicoctlStepBuilder(runtimeCtx, "RemoveCalicoctlBinary").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalicoResources", Step: cleanCalicoStep, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalicoctlBinary", Step: removeCalicoctlStep, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
