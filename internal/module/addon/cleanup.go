package addon

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskAddon "github.com/mensylisir/kubexm/internal/task/addon"
)

// AddonCleanupModule is responsible for uninstalling addons during cluster deletion.
type AddonCleanupModule struct {
	module.BaseModule
}

// NewAddonCleanupModule creates a new AddonCleanupModule.
func NewAddonCleanupModule() module.Module {
	base := module.NewBaseModule("AddonCleanup", []task.Task{})
	return &AddonCleanupModule{BaseModule: base}
}

func (m *AddonCleanupModule) Name() string {
	return "AddonCleanup"
}

func (m *AddonCleanupModule) Description() string {
	return "Uninstalls addons from the cluster during deletion"
}

// Tasks returns empty slice since tasks are built dynamically in Plan()
func (m *AddonCleanupModule) Tasks() []task.Task {
	return []task.Task{}
}

func (m *AddonCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		logger.Info("No cluster config found, skipping addon cleanup")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	if len(clusterCfg.Spec.Addons) == 0 {
		logger.Info("No addons specified in cluster configuration, skipping cleanup")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	// Build list of cleanup tasks based on configured addons
	var cleanupTasks []task.Task

	for i := range clusterCfg.Spec.Addons {
		addon := &clusterCfg.Spec.Addons[i]
		// Only clean addons that were enabled
		if addon.Enabled != nil && !*addon.Enabled {
			continue
		}
		cleanupTasks = append(cleanupTasks, taskAddon.NewCleanAddonTask(addon))
	}

	if len(cleanupTasks) == 0 {
		logger.Info("No addon cleanup tasks to plan")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	var previousTaskExitNodes []plan.NodeID

	for _, ct := range cleanupTasks {
		taskFrag, err := ct.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan addon cleanup task %s: %w", ct.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}
		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment for task %s: %w", ct.Name(), err)
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for task %s: %w", ct.Name(), err)
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		logger.Info("Addon cleanup module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("AddonCleanup module planning complete", "nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

func (m *AddonCleanupModule) GetBase() *module.BaseModule {
	return &m.BaseModule
}

// Ensure AddonCleanupModule implements module.Module interface
var _ module.Module = (*AddonCleanupModule)(nil)
