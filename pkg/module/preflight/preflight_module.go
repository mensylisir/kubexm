package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module" // For module.Module, module.BaseModule, module.ModuleContext
	"github.com/mensylisir/kubexm/pkg/plan"
	// "github.com/mensylisir/kubexm/pkg/runtime" // Removed
	"github.com/mensylisir/kubexm/pkg/task" // For task.Task, task.ExecutionFragment, task.TaskContext
	"github.com/mensylisir/kubexm/pkg/task/greeting"
	"github.com/mensylisir/kubexm/pkg/task/pre"
	// taskPreflight "github.com/mensylisir/kubexm/pkg/task/preflight" // Keep if SystemChecksTask etc. are still used
)

// PreflightModule defines the module for preflight checks and setup.
type PreflightModule struct {
	// No need to embed BaseModule if we directly implement Name() and GetTasks()
	name      string
	assumeYes bool
	// tasks can be stored here if they are static for this module type
	staticTasks []task.Task
}

// NewPreflightModule creates a new PreflightModule.
func NewPreflightModule(assumeYes bool) module.Module {
	// Define the static sequence of tasks for this module.
	// If tasks were dynamic, this logic would be in GetTasks().
	stTasks := []task.Task{
		greeting.NewGreetingTask(),
		pre.NewConfirmTask("InitialConfirmation", "Proceed with KubeXM operations?", assumeYes),
		// Assuming these constructors are available from pkg/task/preflight or this package
		// For the purpose of this refactor, let's assume they are defined in this package
		// to avoid resolving their actual location if they are specific to preflight.
		NewSystemChecksTask(nil),      // Placeholder, might need specific roles or config
		NewInitialNodeSetupTask(),     // Placeholder
		NewSetupKernelTask(),          // Placeholder
		pre.NewVerifyArtifactsTask(),  // From pkg/task/pre
		pre.NewCreateRepositoryTask(), // From pkg/task/pre
	}

	return &PreflightModule{
		name:        "PreflightChecksAndSetup",
		assumeYes:   assumeYes,
		staticTasks: stTasks,
	}
}

// Name returns the name of the module.
func (m *PreflightModule) Name() string {
	return m.name
}

// GetTasks returns the list of tasks for this module.
// For PreflightModule, the tasks are statically defined but could be dynamic in other modules.
func (m *PreflightModule) GetTasks(ctx module.ModuleContext) ([]task.Task, error) {
	// Example of dynamic task determination (though not strictly needed for this static preflight)
	// if ctx.GetClusterConfig().Spec.SomeFlag {
	//     return aDifferentSetOfTasks, nil
	// }
	// For PreflightModule, tasks are fixed by assumeYes, but confirmTask is handled in Plan.
	// We can return all potential tasks and let Plan use IsRequired.

	// Re-create confirm task here if its `assumeYes` needs to be dynamic from module state
	// or just return the statically configured one.
	// The current NewPreflightModule already configures ConfirmTask with assumeYes.
	return m.staticTasks, nil
}

// Plan generates the execution fragment for the preflight module.
func (m *PreflightModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	// The ModuleContext (ctx) should be directly usable by tasks if runtime.Context implements both.
	// No need to assert to task.TaskContext if method signatures align.
	// task.Plan(ctx task.TaskContext) means ctx must satisfy task.TaskContext.
	// If module.ModuleContext is a superset or compatible, it's fine.
	// runtime.Context implements all these context interfaces.

	definedTasks, err := m.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for module %s: %w", m.Name(), err)
	}

	var previousTaskExitNodes []plan.NodeID
	firstTaskProcessed := false

	for _, currentTask := range definedTasks {
		isRequired, err := currentTask.IsRequired(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !isRequired {
			logger.Debug("Skipping non-required task", "task", currentTask.Name())
			continue
		}

		taskFrag, err := currentTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if taskFrag == nil || taskFrag.IsEmpty() {
			logger.Debug("Task planned an empty fragment, skipping merge/link", "task", currentTask.Name())
			// If it's empty but was the first, its (non-existent) exits are still the module's entries for now.
			// If it's empty and not first, previousTaskExitNodes remain unchanged.
			continue
		}

		err = moduleFragment.MergeFragment(taskFrag)
		if err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s into module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if !firstTaskProcessed {
			// All entry nodes of the first processed task fragment become entry nodes for the module fragment.
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFrag.EntryNodes...)
			firstTaskProcessed = true
		} else {
			// Link current task's entry nodes to previous task's exit nodes
			if len(previousTaskExitNodes) > 0 && len(taskFrag.EntryNodes) > 0 {
				for _, entryNodeID := range taskFrag.EntryNodes {
					targetNode, exists := moduleFragment.Nodes[entryNodeID]
					if !exists {
						// This should not happen if MergeFragment worked correctly
						return nil, fmt.Errorf("internal error: entry node %s from task %s not found in merged module fragment", entryNodeID, currentTask.Name())
					}
					// Append dependencies, ensuring no duplicates if a node is somehow re-linked
					targetNode.Dependencies = append(targetNode.Dependencies, previousTaskExitNodes...)
					targetNode.Dependencies = plan.UniqueNodeIDs(targetNode.Dependencies)
				}
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	// After all tasks are processed and linked, set the module's exit nodes.
	// If previousTaskExitNodes is empty (e.g., all tasks were empty or skipped),
	// and the module itself had some initial entry points (e.g. from a task that was all entry points),
	// those entry points might also be exit points if nothing followed.
	// However, CalculateEntryAndExitNodes should handle this correctly.
	// If all tasks were skipped resulting in an empty moduleFragment, Entry/Exit nodes will be empty.
	// If there were tasks, previousTaskExitNodes holds the exits of the last processed task.
	// moduleFragment.ExitNodes = previousTaskExitNodes // This direct assignment is only for linear chains.

	// Recalculate final entry/exit nodes for the entire module fragment based on its final graph structure.
	moduleFragment.CalculateEntryAndExitNodes()

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Preflight module planned no executable nodes.")
	} else {
		logger.Info("Preflight module planning complete.", "totalNodes", len(moduleFragment.Nodes), "entryNodes", moduleFragment.EntryNodes, "exitNodes", moduleFragment.ExitNodes)
	}

	return moduleFragment, nil
}

// Ensure PreflightModule implements the module.Module interface.
var _ module.Module = (*PreflightModule)(nil)

// Placeholders for task constructors assumed to be in this package or preflight task package
// These would typically be in their own files within pkg/task/preflight/ or similar.
func NewSystemChecksTask(roles []string) task.Task {
	// Example implementation or return a mock/nil for structure
	return &mockPreflightTask{name: "SystemChecks"}
}
func NewInitialNodeSetupTask() task.Task {
	return &mockPreflightTask{name: "InitialNodeSetup"}
}
func NewSetupKernelTask() task.Task {
	return &mockPreflightTask{name: "SetupKernel"}
}

type mockPreflightTask struct {
	name string
	description string
}
func (m *mockPreflightTask) Name() string { return m.name }
func (m *mockPreflightTask) Description() string { return m.description }
func (m *mockPreflightTask) IsRequired(ctx task.TaskContext) (bool, error) { return true, nil }
func (m *mockPreflightTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	frag := task.NewExecutionFragment(m.name)
	nodeID := plan.NodeID(m.name + "-node")
	frag.AddNode(&plan.ExecutionNode{Name: m.name + "-node", StepName: m.name + "-step"}, nodeID)
	frag.EntryNodes = []plan.NodeID{nodeID}
	frag.ExitNodes = []plan.NodeID{nodeID}
	return frag, nil
}
var _ task.Task = (*mockPreflightTask)(nil)
