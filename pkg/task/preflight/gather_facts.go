package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// GatherFactsTask collects system information from all hosts.
// This should be the first task in any pipeline to establish 
// the runtime environment state.
type GatherFactsTask struct {
	name        string
	description string
}

// NewGatherFactsTask creates a new GatherFactsTask.
func NewGatherFactsTask() task.Task {
	return &GatherFactsTask{
		name:        "GatherFacts",
		description: "Collects system facts from all cluster hosts",
	}
}

// Name returns the task name.
func (t *GatherFactsTask) Name() string {
	return t.name
}

// Description returns the task description.
func (t *GatherFactsTask) Description() string {
	return t.description
}

// IsRequired determines if fact gathering is needed.
// This task is always required as it establishes baseline state.
func (t *GatherFactsTask) IsRequired(ctx task.TaskContext) (bool, error) {
	return true, nil
}

// Plan generates an execution fragment for gathering facts from all hosts.
func (t *GatherFactsTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	fragment := task.NewExecutionFragment("gather-facts")
	logger := ctx.GetLogger().With("task", t.Name())

	// Get all hosts that need fact gathering
	allHosts, err := ctx.GetHostsByRole("all")
	if err != nil {
		logger.Error("Failed to get hosts for fact gathering", "error", err)
		return nil, fmt.Errorf("failed to get all hosts for fact gathering: %w", err)
	}

	if len(allHosts) == 0 {
		logger.Warn("No hosts found for fact gathering")
		return task.NewEmptyFragment("gather-facts-empty"), nil
	}

	logger.Info("Planning fact gathering", "host_count", len(allHosts))

	// Create a gather facts step for each host
	// These can all run in parallel since they're independent
	for i, host := range allHosts {
		stepName := fmt.Sprintf("gather-facts-%s", host.GetName())
		nodeID := plan.NodeID(fmt.Sprintf("gather-facts-node-%d", i))

		gatherStep := common.NewGatherFactsStep(stepName)

		node := &plan.ExecutionNode{
			Name:         fmt.Sprintf("Gather facts from %s", host.GetName()),
			Step:         gatherStep,
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{}, // No dependencies - can run in parallel
			StepName:     gatherStep.Meta().Name,
		}

		_, err := fragment.AddNode(node, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to add gather facts node for host %s: %w", host.GetName(), err)
		}

		// All nodes are both entry and exit nodes since they're independent
		fragment.EntryNodes = append(fragment.EntryNodes, nodeID)
		fragment.ExitNodes = append(fragment.ExitNodes, nodeID)
	}

	logger.Info("Fact gathering plan created", "nodes", len(fragment.Nodes))
	return fragment, nil
}

var _ task.Task = (*GatherFactsTask)(nil)