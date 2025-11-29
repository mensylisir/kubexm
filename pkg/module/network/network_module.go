package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskNetwork "github.com/mensylisir/kubexm/pkg/task/network"
)

// NetworkModule is responsible for deploying the CNI network plugin.
type NetworkModule struct {
	module.BaseModule
}

// NewNetworkModule creates a new NetworkModule.
func NewNetworkModule() module.Module {
	base := module.NewBaseModule("KubernetesNetworkSetup", nil) // Tasks are now fetched via GetTasks
	return &NetworkModule{BaseModule: base}
}

// GetTasks returns the list of tasks for the NetworkModule.
func (m *NetworkModule) GetTasks(ctx runtime.ModuleContext) ([]task.Task, error) {
	// For this module, the task is static.
	// More complex modules might determine tasks dynamically based on ctx.
	return []task.Task{
		taskNetwork.NewInstallNetworkPluginTask(),
	}, nil
}

func (m *NetworkModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	definedTasks, err := m.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for module %s: %w", m.Name(), err)
	}
	if len(definedTasks) == 0 { // Should not happen for this static module if constructor is right
		logger.Info("No tasks defined for NetworkModule. Skipping.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	installPluginTask := definedTasks[0] // Assuming only one task for this simple module

	isRequired, err := installPluginTask.IsRequired(taskCtx) // Pass module.ModuleContext
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", installPluginTask.Name(), err)
	}

	if !isRequired {
		logger.Info("Network plugin installation is not required by configuration/logic. Skipping module.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Planning task", "task_name", installPluginTask.Name())
	taskFrag, err := installPluginTask.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan task %s: %w", installPluginTask.Name(), err)
	}

	if err := moduleFragment.MergeFragment(taskFrag); err != nil {
		return nil, err
	}

	// The entry and exit nodes of this module are directly those of the InstallNetworkPluginTask's fragment.
	moduleFragment.EntryNodes = taskFrag.EntryNodes
	moduleFragment.ExitNodes = taskFrag.ExitNodes

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("NetworkModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Network module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*NetworkModule)(nil)
