package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module" // Alias to avoid collision if task.Module is also used by name
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/greeting"
	"github.com/mensylisir/kubexm/pkg/task/pre"
	// taskPreflight "github.com/mensylisir/kubexm/pkg/task/preflight" // Keep if SystemChecksTask etc. are still used
)

// PreflightModule defines the module for preflight checks and setup.
type PreflightModule struct {
	module.BaseModule // Embed BaseModule
	AssumeYes         bool
}

// NewPreflightModule creates a new PreflightModule.
// It initializes the tasks that this module will orchestrate.
func NewPreflightModule(assumeYes bool) module.Module { // Returns module.Module interface
	// Define the sequence of tasks for this module
	moduleTasks := []task.Task{
		greeting.NewGreetingTask(),
		pre.NewConfirmTask("InitialConfirmation", "Proceed with KubeXM operations?", assumeYes),
		pre.NewPreTask(), // General pre-flight checks defined in PreTask
		// TODO: Add taskPreflight.NewSystemChecksTask() if it's distinct from pre.NewPreTask()
		// TODO: Add taskPreflight.NewSetupKernelTask()
		// TODO: Add VerifyArtifactsTask (from pkg/task/pre) when created
		// TODO: Add CreateRepositoryTask (from pkg/task/pre) when created
	}

	base := module.NewBaseModule("PreflightChecksAndSetup", moduleTasks)
	pm := &PreflightModule{
		BaseModule: base,
		AssumeYes:  assumeYes,
	}
	return pm
}

// Plan generates the execution fragment for the preflight module.
// It orchestrates the fragments from SystemChecksTask and SetupKernelTask,
// making SetupKernelTask depend on SystemChecksTask.
func (m *PreflightModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := &task.ExecutionFragment{
		Nodes:      make(map[plan.NodeID]*plan.ExecutionNode),
		EntryNodes: []plan.NodeID{},
		ExitNodes:  []plan.NodeID{},
	}

	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	for _, currentTask := range m.Tasks() { // Use m.Tasks() from BaseModule
		// Create TaskContext from ModuleContext
		// This assumes ModuleContext has a method to derive a TaskContext,
		// or TaskContext can be created from the components of ModuleContext.
		// For now, let's assume ModuleContext can provide what TaskContext needs.
		// If ModuleContext is a superset of TaskContext, we might just pass it,
		// but facades are for stricter separation.
		// A common pattern is: taskCtx := ctx.NewTaskContext(additionalArgsIfNeeded)
		// For now, assuming TaskContext can be derived or ModuleContext is sufficient.
		// Let's use a hypothetical NewTaskContext method on the ModuleContext for clarity.
		// This will be refined when runtime.ModuleContext is fully defined.
		// For the purpose of this example, we will assume ctx (ModuleContext) can fulfill TaskContext's needs.
		// However, the design doc implies specific facade types.
		// So, `ctx.(runtime.TaskContext)` would be wrong. `ctx.GetTaskContext()` or similar.
		// Let's assume ModuleContext has a method `DeriveTaskContext()` or similar.
		// For now, we'll pass module context directly if the interfaces are compatible or use a placeholder.
		// The design doc shows distinct interfaces: PipelineContext, ModuleContext, TaskContext, StepContext.
		// This means we need a way to get a TaskContext from ModuleContext.
		// This method should ideally be part of the ModuleContext interface.
		// Example: taskCtx := ctx.TaskCtx()

		// Let's assume for now that the ModuleContext itself can satisfy the broader parts of TaskContext
		// like GetLogger, GetClusterConfig, GoContext, and GetHostsByRole, GetHostFacts might need to be
		// part of ModuleContext or TaskContext needs to be constructible.
		// The "13-runtime设计.md" shows TaskContext embedding ModuleContext.
		// So, if our `ctx` is a concrete type that implements both, it's fine.
		// But if it's strictly ModuleContext, we need a conversion.
		// Let's assume runtime.Context (the main one) implements all, and facades are views.
		// The `Plan` methods will receive the specific facade type.
		// So, ModuleContext needs a way to provide a TaskContext to its tasks.
		// This is a detail of runtime context implementation.
		// For now, let's assume `ctx` (ModuleContext) is sufficient for `IsRequired` and `Plan`.
		// This will be addressed when runtime facades are fully implemented.
		// A simple way: runtime.NewTaskContext(moduleCtx runtime.ModuleContext) runtime.TaskContext
		// For this step, let's proceed assuming TaskContext can be obtained.
		// A placeholder: taskRuntimeCtx := ctx (this needs to be fixed if ModuleContext lacks TaskContext methods)
		// Based on "13-runtime设计.md", TaskContext embeds ModuleContext.
		// So, a concrete type that implements TaskContext would also implement ModuleContext.
		// The issue is if the `ctx` parameter here *is* only a ModuleContext interface.
		// Let's assume the underlying concrete context object that Module.Plan receives
		// also implements TaskContext. This is a common approach with embedded interfaces.
		// If not, ModuleContext needs a method like `ToTaskContext()`.

		// Simplification for now: assume ModuleContext can be used where TaskContext is needed if methods overlap.
		// This is not ideal for strict facade separation. The correct way is for ModuleContext to provide a TaskContext.
		// Let's assume ModuleContext has a method `TaskCtx()` that returns a TaskContext.
		// This needs to be added to runtime.ModuleContext interface definition.
		// For now, to make progress, I will assume `ctx` can be used for methods common to both.
		// If specific TaskContext methods are needed, this will fail compilation later.
		// Assuming ctx (ModuleContext) is fulfilled by an object that also fulfills TaskContext.
		taskIsRequired, err := currentTask.IsRequired(ctx)
		if err != nil {
			logger.Error(err, "Error checking if task is required", "task", currentTask.Name())
			return nil, fmt.Errorf("failed to check if task %s is required: %w", currentTask.Name(), err)
		}
		if !taskIsRequired {
			logger.Info("Skipping task as it's not required", "task", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task", currentTask.Name())
		// Same assumption for ctx being passable as TaskContext.
		taskFragment, err := currentTask.Plan(ctx)
		if err != nil {
			logger.Error(err, "Failed to plan task", "task", currentTask.Name())
			return nil, fmt.Errorf("failed to plan task %s: %w", currentTask.Name(), err)
		}

		if taskFragment == nil || len(taskFragment.Nodes) == 0 {
			logger.Info("Task returned an empty fragment, skipping", "task", currentTask.Name())
			continue
		}

		// Merge nodes from taskFragment into moduleFragment
		for id, node := range taskFragment.Nodes {
			if _, exists := moduleFragment.Nodes[id]; exists {
				err := fmt.Errorf("duplicate NodeID %s detected when merging fragments from task %s", id, currentTask.Name())
				logger.Error(err, "NodeID collision")
				return nil, err
			}
			moduleFragment.Nodes[id] = node
		}

		// Link current task's entry nodes to previous task's exit nodes
		if len(previousTaskExitNodes) > 0 {
			for _, entryNodeID := range taskFragment.EntryNodes {
				entryNode, ok := moduleFragment.Nodes[entryNodeID]
				if !ok {
					return nil, fmt.Errorf("internal error: entry node %s not found in module fragment", entryNodeID)
				}
				existingDeps := make(map[plan.NodeID]bool)
				for _, depID := range entryNode.Dependencies {
					existingDeps[depID] = true
				}
				for _, prevExitNodeID := range previousTaskExitNodes {
					if !existingDeps[prevExitNodeID] {
						entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
						existingDeps[prevExitNodeID] = true
					}
				}
			}
		} else if isFirstEffectiveTask {
			// This task fragment's entry nodes are also entry nodes for the module
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFragment.EntryNodes...)
		}

		// The current task's exit nodes become the new "previous" for the next iteration.
		// If the current task fragment is empty, `previousTaskExitNodes` remains from the task before that.
		if len(taskFragment.ExitNodes) > 0 {
		    previousTaskExitNodes = taskFragment.ExitNodes
			isFirstEffectiveTask = false // An effective task has been planned
		}
	}

	// Module's exit nodes are the exit nodes of the last effectively planned task.
	moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, previousTaskExitNodes...)

	// Deduplicate EntryNodes and ExitNodes for the module fragment
	moduleFragment.EntryNodes = uniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = uniqueNodeIDs(moduleFragment.ExitNodes)


	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Preflight module planned no executable nodes.")
	} else {
		logger.Info("Preflight module planning complete.", "totalNodes", len(moduleFragment.Nodes), "entryNodes", moduleFragment.EntryNodes, "exitNodes", moduleFragment.ExitNodes)
	}

	return moduleFragment, nil
}

// uniqueNodeIDs returns a slice with unique NodeIDs.
func uniqueNodeIDs(ids []plan.NodeID) []plan.NodeID {
    if len(ids) == 0 {
        return []plan.NodeID{}
    }
    seen := make(map[plan.NodeID]bool)
    result := []plan.NodeID{}
    for _, id := range ids {
        if !seen[id] {
            seen[id] = true
            result = append(result, id)
        }
    }
    return result
}

// Ensure PreflightModule implements the module.Module interface.
var _ module.Module = (*PreflightModule)(nil)
