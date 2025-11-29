package addon

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskAddon "github.com/mensylisir/kubexm/pkg/task/addon"
)

// AddonsModule is responsible for deploying specified addons to the cluster.
type AddonsModule struct {
	module.BaseModule
}

// NewAddonsModule creates a new AddonsModule.
func NewAddonsModule() module.Module {
	base := module.NewBaseModule("ClusterAddonsDeployment", nil) // Tasks are dynamic via GetTasks
	return &AddonsModule{BaseModule: base}
}

// GetTasks dynamically generates a list of InstallAddonTask instances
// based on the addons specified in the cluster configuration.
func (m *AddonsModule) GetTasks(ctx runtime.ModuleContext) ([]task.Task, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "phase", "GetTasks")
	clusterCfg := ctx.GetClusterConfig()

	if len(clusterCfg.Spec.Addons) == 0 {
		logger.Info("No addons specified in cluster configuration.")
		return []task.Task{}, nil
	}

	addonTasks := make([]task.Task, 0, len(clusterCfg.Spec.Addons))
	for i := range clusterCfg.Spec.Addons {
		addon := &clusterCfg.Spec.Addons[i]
		logger.Debug("Creating task for addon", "addon_name", addon.Name)
		// Ensure Enabled is set if nil, though SetDefaults should handle this.
		if addon.Enabled == nil {
			enabled := true
			addon.Enabled = &enabled
		}
		addonTasks = append(addonTasks, taskAddon.NewInstallAddonTask(addon))
	}
	return addonTasks, nil
}

func (m *AddonsModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	// runtime.Context implements both module.ModuleContext and runtime.TaskContext
	// so direct use of ctx for task methods is fine.
	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	definedTasks, err := m.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for module %s: %w", m.Name(), err)
	}

	if len(definedTasks) == 0 {
		logger.Info("No addon tasks to plan. Skipping addons deployment.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	var allAddonEntryNodes []plan.NodeID
	var allAddonExitNodes []plan.NodeID

	for _, addonTask := range definedTasks {
		// Note: addonTask.Name() might be generic like "InstallAddon".
		// Logging with a more specific identifier if possible, e.g., from task's internal state if it stores addonName.
		// For now, addonTask.Name() is what BaseTask provides.
		// The NewInstallAddonTask should ideally set a unique name in its StepMeta.
		// Let's assume NewInstallAddonTask sets a descriptive name.
		addonInstanceName := addonTask.Name() // Or get specific addon name from the task if available

		isRequired, err := addonTask.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check IsRequired for addon task %s: %w", addonInstanceName, err)
		}
		if !isRequired {
			logger.Info("Skipping addon task as it's not required", "addon_task_name", addonInstanceName)
			continue
		}

		logger.Info("Planning addon task", "addon_task_name", addonInstanceName)
		taskFrag, err := addonTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan addon task %s: %w", addonInstanceName, err)
		}

		if taskFrag.IsEmpty() {
			logger.Info("Addon task returned an empty fragment, skipping merge.", "addon_name", addonInstanceName)
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment from addon task %s: %w", addonInstanceName, err)
		}

		// Addons can typically be installed in parallel.
		// Their entry nodes become part of the module's entry nodes (if not dependent on prior module stages).
		// Their exit nodes all contribute to the module's exit nodes.
		allAddonEntryNodes = append(allAddonEntryNodes, taskFrag.EntryNodes...)
		allAddonExitNodes = append(allAddonExitNodes, taskFrag.ExitNodes...)
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(allAddonEntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(allAddonExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("AddonsModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Addons module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*AddonsModule)(nil)
