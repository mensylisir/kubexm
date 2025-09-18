package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskos "github.com/mensylisir/kubexm/pkg/task/os"
	taskpre "github.com/mensylisir/kubexm/pkg/task/pre"
	taskpreflight "github.com/mensylisir/kubexm/pkg/task/preflight"
)

// PreflightModule defines the module for preflight checks and setup.
type PreflightModule struct {
	module.BaseModule
	assumeYes bool
}

// NewPreflightModule creates a new PreflightModule.
func NewPreflightModule(assumeYes bool) module.Module {
	return &PreflightModule{
		BaseModule: module.NewBaseModule("PreflightChecksAndSetup", nil), // Tasks are now fetched via GetTasks
		assumeYes:  assumeYes,
	}
}

// GetTasks returns the list of tasks for this module.
func (m *PreflightModule) GetTasks(ctx module.ModuleContext) ([]task.Task, error) {
	return []task.Task{
		taskpreflight.NewGreetingTask(),
		taskpre.NewConfirmTask("InitialConfirmation", "Proceed with KubeXM operations?", m.assumeYes),
		taskpreflight.NewPreflightChecksTask(),
		taskos.NewPrepareOSNodesTask(),
		taskpre.NewVerifyArtifactsTask(),
		taskpre.NewCreateRepositoryTask(),
	}, nil
}

// Plan generates the execution fragment for the preflight module.
func (m *PreflightModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	definedTasks, err := m.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for module %s: %w", m.Name(), err)
	}

	var previousTaskExitNodes []plan.NodeID

	for i, currentTask := range definedTasks {
		isRequired, err := currentTask.IsRequired(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !isRequired {
			logger.Debug("Skipping non-required task", "task", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task_name", currentTask.Name())
		taskFrag, err := currentTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if taskFrag == nil || taskFrag.IsEmpty() {
			logger.Debug("Task planned an empty fragment, skipping merge/link", "task", currentTask.Name())
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s into module %s: %w", currentTask.Name(), m.Name(), err)
		}

		// Link current task's entry nodes to previous task's exit nodes
		if i > 0 && len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes)
		}

		previousTaskExitNodes = taskFrag.ExitNodes
	}

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
