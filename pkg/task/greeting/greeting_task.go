package greeting

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/common" // For PrintMessageStep
	"github.com/mensylisir/kubexm/pkg/task"
)

const DefaultLogo = `
██╗  ██╗██╗   ██╗██████╗ ███████╗██╗  ██╗███╗   ███╗
██║  ██║██║   ██║██╔══██╗██╔════╝██║  ██║████╗ ████║
███████║██║   ██║██████╔╝█████╗  ███████║██╔████╔██║
██╔══██║██║   ██║██╔══██╗██╔══╝  ██╔══██║██║╚██╔╝██║
██║  ██║╚██████╔╝██████╔╝███████╗██║  ██║██║ ╚═╝ ██║
╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝
Welcome to KubeXM - Kubernetes Xtreme Manager!
`

// GreetingTask displays a welcome logo/message.
type GreetingTask struct {
	task.BaseTask
	LogoMessage string
}

// NewGreetingTask creates a new GreetingTask.
func NewGreetingTask() task.Task {
	return &GreetingTask{
		BaseTask: task.BaseTask{ // Assuming BaseTask constructor or direct field setting
			TaskName: "DisplayWelcomeGreeting",
			TaskDesc: "Displays a welcome logo and message to the user.",
		},
		LogoMessage: DefaultLogo,
	}
}

// Name returns the task's name. (If BaseTask provides it, this can be removed)
func (t *GreetingTask) Name() string {
	return t.BaseTask.TaskName // Or directly "DisplayWelcomeGreeting"
}

// Description returns the task's description. (If BaseTask provides it, this can be removed)
func (t *GreetingTask) Description() string {
	return t.BaseTask.TaskDesc // Or directly the description string
}

// IsRequired for GreetingTask is always true as it's a cosmetic step.
func (t *GreetingTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

// Plan generates the execution fragment for displaying the greeting.
func (t *GreetingTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
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
