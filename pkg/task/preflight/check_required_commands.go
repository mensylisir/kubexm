package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckRequiredCommandsTask struct {
	task.Base
}

func NewCheckRequiredCommandsTask() task.Task {
	return &CheckRequiredCommandsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckRequiredCommands",
				Description: "Check for the existence of required command-line tools",
			},
		},
	}
}

func (t *CheckRequiredCommandsTask) Name() string {
	return t.Meta.Name
}

func (t *CheckRequiredCommandsTask) Description() string {
	return t.Meta.Description
}

func (t *CheckRequiredCommandsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckRequiredCommandsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkCommandsStep := preflightstep.NewCheckRequiredCommandsStepBuilder(*runtimeCtx, "CheckRequiredCommands").Build()

	node := &plan.ExecutionNode{
		Name:  "CheckRequiredCommands",
		Step:  checkCommandsStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
