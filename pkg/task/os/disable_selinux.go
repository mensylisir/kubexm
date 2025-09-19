package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DisableSelinuxTask struct {
	task.Base
}

func NewDisableSelinuxTask() task.Task {
	return &DisableSelinuxTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DisableSelinux",
				Description: "Disable SELinux on all nodes",
			},
		},
	}
}

func (t *DisableSelinuxTask) Name() string {
	return t.Meta.Name
}

func (t *DisableSelinuxTask) Description() string {
	return t.Meta.Description
}

func (t *DisableSelinuxTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return *ctx.GetClusterConfig().Spec.Preflight.DisableSelinux, nil
}

func (t *DisableSelinuxTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableSelinuxStep := osstep.NewDisableSelinuxStepBuilder(*runtimeCtx, "DisableSelinux").Build()

	node := &plan.ExecutionNode{
		Name:  "DisableSelinux",
		Step:  disableSelinuxStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
