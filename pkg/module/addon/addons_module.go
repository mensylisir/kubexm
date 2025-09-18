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
func (m *AddonsModule) GetTasks(ctx module.ModuleContext) ([]task.Task, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "phase", "GetTasks")
	clusterCfg := ctx.GetClusterConfig()

	if len(clusterCfg.Spec.Addons) == 0 {
		logger.Info("No addons specified in cluster configuration.")
		return []task.Task{}, nil
	}

	addonTasks := make([]task.Task, 0, len(clusterCfg.Spec.Addons))
	for i := range clusterCfg.Spec.Addons {
		addon := &clusterCfg.Spec.Addons[i] // Get a pointer to the addon struct
		logger.Debug("Creating task for addon", "addon_name", addon.Name)
		// Pass the addon struct pointer to the task constructor
		addonTasks = append(addonTasks, taskAddon.NewInstallAddonTask(addon))
	}
	return addonTasks, nil
}


func (m *AddonsModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	definedTasks, err := m.GetTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for module %s: %w", m.Name(), err)
	}

	if len(definedTasks) == 0 {
		logger.Info("No addon tasks to plan. Skipping addons deployment.")
		return task.NewEmptyFragment(), nil
	}

	var allAddonEntryNodes []plan.NodeID
	var allAddonExitNodes []plan.NodeID

	for _, addonTask := range definedTasks {
		addonInstanceName := addonTask.Name()

		isRequired, err := addonTask.IsRequired(ctx)
		if err != nil { return nil, fmt.Errorf("failed to check IsRequired for addon task %s: %w", addonInstanceName, err) }
		if !isRequired {
			logger.Info("Skipping addon task as it's not required", "addon_task_name", addonInstanceName)
			continue
		}

		logger.Info("Planning addon task", "addon_task_name", addonInstanceName)
		taskFrag, err := addonTask.Plan(ctx)
		if err != nil { return nil, fmt.Errorf("failed to plan addon task %s: %w", addonInstanceName, err) }

		if taskFrag.IsEmpty() {
			logger.Info("Addon task returned an empty fragment, skipping merge.", "addon_name", addonInstanceName)
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil { return nil, fmt.Errorf("failed to merge fragment from addon task %s: %w", addonInstanceName, err) }

		allAddonEntryNodes = append(allAddonEntryNodes, taskFrag.EntryNodes...)
		allAddonExitNodes = append(allAddonExitNodes, taskFrag.ExitNodes...)
	}

	moduleFragment.EntryNodes = task.UniqueNodeIDs(allAddonEntryNodes)
	moduleFragment.ExitNodes = task.UniqueNodeIDs(allAddonExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("AddonsModule planned no executable nodes.")
	} else {
		logger.Info("Addons module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*AddonsModule)(nil)
