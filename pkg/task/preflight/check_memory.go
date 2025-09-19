package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckMemoryTask struct {
	task.Base
}

func NewCheckMemoryTask() task.Task {
	return &CheckMemoryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckMemory",
				Description: "Check for the minimum required amount of memory",
			},
		},
	}
}

func (t *CheckMemoryTask) Name() string {
	return t.Meta.Name
}

func (t *CheckMemoryTask) Description() string {
	return t.Meta.Description
}

func (t *CheckMemoryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckMemoryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkMemoryStep := preflightstep.NewCheckMemoryStepBuilder(*runtimeCtx, "CheckMinMemory").Build()

	node := &plan.ExecutionNode{
		Name:  "CheckMinMemory",
		Step:  checkMemoryStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
