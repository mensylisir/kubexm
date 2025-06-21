package pre

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

const DefaultConfirmationPrompt = "Are you sure you want to proceed with the operation?"

// ConfirmTask prompts the user for confirmation before proceeding.
type ConfirmTask struct {
	task.BaseTask
	PromptMessage string
	AssumeYes     bool // If true, task will be skipped or auto-confirmed.
}

// NewConfirmTask creates a new ConfirmTask.
// If assumeYes is true, the task's Precheck will likely cause it to be skipped.
func NewConfirmTask(instanceName, promptMessage string, assumeYes bool) task.Task {
	name := instanceName
	if name == "" {
		name = "UserConfirmation"
	}
	if promptMessage == "" {
		promptMessage = DefaultConfirmationPrompt
	}
	return &ConfirmTask{
		BaseTask: task.BaseTask{
			TaskName: name,
			TaskDesc: "Prompts the user for confirmation before proceeding.",
		},
		PromptMessage: promptMessage,
		AssumeYes:     assumeYes,
	}
}

// IsRequired for ConfirmTask is always true, but its UserInputStep's Precheck
// will return 'done' if AssumeYes is true, effectively skipping the prompt.
func (t *ConfirmTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

// Plan generates the execution fragment for the confirmation prompt.
func (t *ConfirmTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	fragment := task.NewExecutionFragment()

	controlHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for confirm task: %w", err)
	}

	// The AssumeYes flag from the task is passed to the UserInputStep.
	// The UserInputStep's Precheck will use this to determine if it should be skipped.
	userInputStep := commonsteps.NewUserInputStep(
		"PromptUserForConfirmation",
		t.PromptMessage,
		t.AssumeYes,
	)
	nodeID := plan.NodeID(fmt.Sprintf("user-confirmation-node-%s", t.TaskName))

	fragment.Nodes[nodeID] = &plan.ExecutionNode{
		Name:         "UserConfirmationNode",
		Step:         userInputStep,
		Hosts:        []connector.Host{controlHost}, // Runs on the control node
		StepName:     userInputStep.Meta().Name,
		Dependencies: []plan.NodeID{},
	}

	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.ExitNodes = []plan.NodeID{nodeID}

	return fragment, nil
}

var _ task.Task = (*ConfirmTask)(nil)
