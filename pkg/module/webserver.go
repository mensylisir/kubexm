package module

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
	"github.com/mensylisir/kubexm/pkg/task"    // Updated import path
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
			task.NewInstallNginxTask(), // Assumes NewInstallNginxTask is available in package task
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
func (m *WebServerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionPlan, error) {
	modulePlan := &plan.ExecutionPlan{Phases: []plan.Phase{}}

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
	// If runtime.ModuleContext is not directly assertable to runtime.TaskContext,
	// this code will fail at runtime. The runtime context design needs to ensure this is valid.

	taskCtx, ok := ctx.(runtime.TaskContext)
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
		taskPlan, err := t.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("planning failed for task %s in module %s: %w", t.Name(), m.Name(), err)
		}

		// Append the task's phases to the module's plan
		if taskPlan != nil && len(taskPlan.Phases) > 0 {
			modulePlan.Phases = append(modulePlan.Phases, taskPlan.Phases...)
		}
	}
	return modulePlan, nil
}
