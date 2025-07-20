package greeting

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/common" // For PrintMessageStep
	"github.com/mensylisir/kubexm/pkg/task"
)

// GreetingTask displays a welcome logo/message.
type GreetingTask struct {
	name        string
	description string
	LogoMessage string
}

// NewGreetingTask creates a new GreetingTask.
func NewGreetingTask() task.Task {
	return &GreetingTask{
		name:        "DisplayWelcomeGreeting",
		description: "Displays a welcome logo and message to the user.",
		LogoMessage: DefaultLogo,
	}
}

// Name returns the task name.
func (t *GreetingTask) Name() string {
	return t.name
}

// Description returns the task description.
func (t *GreetingTask) Description() string {
	return t.description
}

// IsRequired for GreetingTask is always true as it's a cosmetic step.
func (t *GreetingTask) IsRequired(ctx task.TaskContext) (bool, error) {
	return true, nil
}

// Plan generates the execution fragment for displaying the greeting.
func (t *GreetingTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	fragment := task.NewExecutionFragment()

	controlHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for greeting task: %w", err)
	}

	printLogoStep := common.NewPrintMessageStep("PrintWelcomeLogo", t.LogoMessage)
	nodeID := plan.NodeID("print-welcome-logo-node")

	fragment.Nodes[nodeID] = &plan.ExecutionNode{
		Name:         "PrintWelcomeLogoNode",
		Step:         printLogoStep,
		Hosts:        []connector.Host{controlHost}, // This step runs on the control node
		StepName:     printLogoStep.Meta().Name,
		Dependencies: []plan.NodeID{},
	}

	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.ExitNodes = []plan.NodeID{nodeID}

	return fragment, nil
}

var _ task.Task = (*GreetingTask)(nil)
