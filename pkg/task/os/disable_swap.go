package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DisableSwapTask struct {
	task.Base
}

func NewDisableSwapTask() task.Task {
	return &DisableSwapTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DisableSwap",
				Description: "Disable swap on all nodes",
			},
		},
	}
}

func (t *DisableSwapTask) Name() string {
	return t.Meta.Name
}

func (t *DisableSwapTask) Description() string {
	return t.Meta.Description
}

func (t *DisableSwapTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return *ctx.GetClusterConfig().Spec.Preflight.DisableSwap, nil
}

func (t *DisableSwapTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableSwapStep := osstep.NewDisableSwapStepBuilder(*runtimeCtx, "DisableSwap").Build()

	node := &plan.ExecutionNode{
		Name:  "DisableSwap",
		Step:  disableSwapStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
