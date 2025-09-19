package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type LoadKernelModulesTask struct {
	task.Base
}

func NewLoadKernelModulesTask() task.Task {
	return &LoadKernelModulesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "LoadKernelModules",
				Description: "Load required kernel modules on all nodes",
			},
		},
	}
}

func (t *LoadKernelModulesTask) Name() string {
	return t.Meta.Name
}

func (t *LoadKernelModulesTask) Description() string {
	return t.Meta.Description
}

func (t *LoadKernelModulesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *LoadKernelModulesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	loadKernelModulesStep := osstep.NewLoadKernelModulesStepBuilder(*runtimeCtx, "LoadKernelModules").Build()

	node := &plan.ExecutionNode{
		Name:  "LoadKernelModules",
		Step:  loadKernelModulesStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
