package pre

import (
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	prestep "github.com/mensylisir/kubexm/internal/step/pre"
	"github.com/mensylisir/kubexm/internal/task"
)

type ConfirmTask struct {
	task.Base
	Prompt    string
	AssumeYes bool
}

func NewConfirmTask(instanceName, prompt string, assumeYes bool) task.Task {
	return &ConfirmTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        instanceName,
				Description: "Prompt the user for confirmation before proceeding",
			},
		},
		Prompt:    prompt,
		AssumeYes: assumeYes,
	}
}

func (t *ConfirmTask) Name() string {
	return t.Meta.Name
}

func (t *ConfirmTask) Description() string {
	return t.Meta.Description
}

func (t *ConfirmTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// The confirmation task is always required unless AssumeYes is true at the pipeline level,
	// which is handled by the step's precheck.
	return true, nil
}

func (t *ConfirmTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	// This step runs on the control node, which is where the CLI is executed.
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	confirmStep, err := prestep.NewConfirmStepBuilder(runtimeCtx, "ConfirmAction", t.Prompt).
		WithAssumeYes(t.AssumeYes).
		Build()
	if err != nil {
		return nil, err
	}

	node := &plan.ExecutionNode{
		Name:  "ConfirmActionNode",
		Step:  confirmStep,
		Hosts: []remotefw.Host{controlNode},
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
