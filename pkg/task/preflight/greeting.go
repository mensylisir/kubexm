package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GreetingTask struct {
	task.Base
}

func NewGreetingTask() task.Task {
	return &GreetingTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "Greeting",
				Description: "Print a welcome message or logo at the beginning of the process",
			},
		},
	}
}

func (t *GreetingTask) Name() string {
	return t.Meta.Name
}

func (t *GreetingTask) Description() string {
	return t.Meta.Description
}

func (t *GreetingTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GreetingTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node to print greeting message: %w", err)
	}

	printLogoStep := preflightstep.NewPrintMessageStepBuilder(runtimeCtx, "PrintWelcomeLogo").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "PrintWelcomeLogo", Step: printLogoStep, Hosts: []connector.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
