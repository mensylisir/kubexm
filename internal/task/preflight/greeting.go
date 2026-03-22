package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	preflightstep "github.com/mensylisir/kubexm/internal/step/preflight"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node to print greeting message: %w", err)
	}

	printLogoStep, err := preflightstep.NewPrintMessageStepBuilder(runtimeCtx, "PrintWelcomeLogo").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "PrintWelcomeLogo", Step: printLogoStep, Hosts: []connector.Host{controlNode}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
