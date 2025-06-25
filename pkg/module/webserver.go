package module

import (
	"fmt"
	// "github.com/mensylisir/kubexm/pkg/plan" // REMOVED as unused
	// "github.com/mensylisir/kubexm/pkg/runtime" // REMOVED
	"github.com/mensylisir/kubexm/pkg/task" // Updated import path for task.Task, task.TaskContext
	// "github.com/mensylisir/kubexm/pkg/module" // Not needed if ModuleContext is local
)

// WebServerModule is an example module for managing a web server.
type WebServerModule struct {
	tasks []task.Task
}

// NewWebServerModule creates a new WebServerModule.
// It initializes its tasks, in this case, with an InstallNginxTask.
func NewWebServerModule() Module {
	return &WebServerModule{
		tasks: []task.Task{
			// task.NewInstallNginxTask(), // TODO: This task needs to be defined or found
		},
	}
}

func (m *WebServerModule) Name() string {
	return "WebServerModule"
}

func (m *WebServerModule) Tasks() []task.Task {
	return m.tasks
}

// Plan generates the execution plan for all relevant tasks within this module.
func (m *WebServerModule) Plan(ctx ModuleContext) (*task.ExecutionFragment, error) { // Changed return type to *task.ExecutionFragment
	// modulePlan := &plan.ExecutionPlan{Phases: []plan.Phase{}} // Old plan structure
	moduleFragment := task.NewEmptyFragment(m.Name() + "-base") // Initialize with an empty fragment, give it a name

	// The issue description implies ModuleContext can be asserted to TaskContext.
	// This might need refinement if the context hierarchy is different.
	// For now, proceeding with the assumption from the issue.
	// A safer way would be to have ModuleContext provide a way to get a TaskContext
	// or for the Engine to handle context down-casting/conversion when iterating tasks.
	// However, the example shows direct assertion.

	// Let's refine the context handling slightly as direct assertion is risky.
	// The issue implies ModuleContext is a superset or can provide TaskContext.
	// For the purpose of this example structure, we'll assume ModuleContext
	// itself satisfies the requirements of TaskContext or can generate one.
	// If runtime.ModuleContext is different from runtime.TaskContext,
	// this part of the design will need adjustment in the runtime package.
	// For now, let's assume they are compatible enough or TaskContext can be derived.

	// The provided snippet uses: moduleCtx := ctx.(runtime.TaskContext)
	// This means ModuleContext must be an interface that TaskContext implements,
	// or ModuleContext and TaskContext are the same, or TaskContext is embedded.
	// We will proceed with the example's approach.
	// runtime.TaskContext has been moved to task.TaskContext.
	// ModuleContext is now local to pkg/module.
	// task.TaskContext embeds module.ModuleContext.
	// So, the concrete type *runtime.Context implements task.TaskContext.
	// If ctx is module.ModuleContext, we need to assert it to task.TaskContext.

	taskCtx, ok := ctx.(task.TaskContext) // Changed to task.TaskContext
	if !ok {
		// This is a critical design point. If ModuleContext is not a TaskContext,
		// then the way tasks get their specific context needs to be defined.
		// For example, ctx.NewTaskContext(task) or similar.
		// For now, following the example's implication of direct assertability.
		return nil, fmt.Errorf("unable to assert ModuleContext to TaskContext for module %s; context design needs review", m.Name())
	}

	for _, t := range m.Tasks() {
		// Check if the task is required
		required, err := t.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("error checking if task %s is required for module %s: %w", t.Name(), m.Name(), err)
		}
		if !required {
			// ctx.GetLogger().Infof("Task %s is not required for module %s. Skipping.", t.Name(), m.Name())
			continue
		}

		// Get the plan for the task
		taskFragment, err := t.Plan(taskCtx) // t.Plan now returns *task.ExecutionFragment
		if err != nil {
			return nil, fmt.Errorf("planning failed for task %s in module %s: %w", t.Name(), m.Name(), err)
		}

		// Append the task's fragment to the module's fragment
		// This requires merging nodes and managing EntryNodes/ExitNodes, similar to PreflightModule.Plan
		// For now, if tasks are empty, this loop doesn't run.
		// If tasks were present, proper fragment merging would be needed here.
		// Example (simplified, assumes tasks are sequential and no complex linking):
		if taskFragment != nil && len(taskFragment.Nodes) > 0 {
			for id, node := range taskFragment.Nodes {
				if _, exists := moduleFragment.Nodes[id]; exists {
					return nil, fmt.Errorf("duplicate NodeID %s from task %s", id, t.Name())
				}
				moduleFragment.Nodes[id] = node
			}
			// Basic sequential linking:
			// if len(moduleFragment.ExitNodes) > 0 { // If not the first task fragment
			//    for _, entry := range taskFragment.EntryNodes {
			//        // Add deps from moduleFragment.ExitNodes to entry
			//    }
			// } else {
			//    moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFragment.EntryNodes...)
			// }
			// moduleFragment.ExitNodes = taskFragment.ExitNodes // Overwrite with last task's exits
		}
	}
	// TODO: Implement proper fragment merging logic if tasks are added back.
	// For now, with no tasks, it returns an empty fragment.
	return moduleFragment, nil
}
