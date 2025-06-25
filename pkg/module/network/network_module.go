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
	tasks := []task.Task{
		taskNetwork.NewInstallNetworkPluginTask(), // Task to install CNI
	}
	base := module.NewBaseModule("KubernetesNetworkSetup", tasks)
	return &NetworkModule{BaseModule: base}
}

func (m *NetworkModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(task.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to task.TaskContext for %s", m.Name())
	}

	// This module typically has one primary task: InstallNetworkPluginTask
	installPluginTask := taskNetwork.NewInstallNetworkPluginTask()

	isRequired, err := installPluginTask.IsRequired(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to check IsRequired for %s: %w", installPluginTask.Name(), err) }

	if !isRequired {
		logger.Info("Network plugin installation is not required by configuration/logic. Skipping module.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Planning task", "task_name", installPluginTask.Name())
	taskFrag, err := installPluginTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan task %s: %w", installPluginTask.Name(), err) }

	if err := moduleFragment.MergeFragment(taskFrag); err != nil { return nil, err }

	// The entry and exit nodes of this module are directly those of the InstallNetworkPluginTask's fragment.
	moduleFragment.EntryNodes = taskFrag.EntryNodes
	moduleFragment.ExitNodes = taskFrag.ExitNodes

	// No internal linking needed if it's just one task.
	// CalculateEntryAndExitNodes might not be strictly necessary if just passing through,
	// but good practice if the fragment was modified.
	// moduleFragment.CalculateEntryAndExitNodes() // Already done by taskFrag if it's the only one

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("NetworkModule planned no executable nodes.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Network module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*NetworkModule)(nil)
