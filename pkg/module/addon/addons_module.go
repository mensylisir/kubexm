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
	// The actual tasks will be dynamically generated in the Plan method
	// based on the cluster configuration.
	base := module.NewBaseModule("ClusterAddonsDeployment", nil)
	return &AddonsModule{BaseModule: base}
}

func (m *AddonsModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(task.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to task.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	if len(clusterCfg.Spec.Addons) == 0 {
		logger.Info("No addons specified in cluster configuration. Skipping addons deployment.")
		return task.NewEmptyFragment(), nil
	}

	var allAddonEntryNodes []plan.NodeID
	var allAddonExitNodes []plan.NodeID

	for _, addonName := range clusterCfg.Spec.Addons {
		// Here, we would ideally fetch specific configuration for this addon if it exists,
		// e.g., from clusterCfg.Spec.AddonConfigs[addonName] or similar.
		// For now, NewInstallAddonTask might take just the name and derive paths/URLs,
		// or expect addon manifests to be available locally (prepared by pkg/resource).

		// Assuming NewInstallAddonTask exists in pkg/task/addon and takes addonName and a generic config map
		// For a more typed approach, specific addon configs could be defined in v1alpha1
		// and passed to specific addon tasks.
		// For now, using a generic placeholder for addon-specific config.
		var addonSpecificConfig map[string]interface{} // Placeholder
		// addonSpecificConfig = clusterCfg.Spec.AddonConfiguration[addonName] // If such a field existed

		addonTask := taskAddon.NewInstallAddonTask(addonName, addonSpecificConfig)

		isRequired, err := addonTask.IsRequired(taskCtx) // Addon task can have its own logic
		if err != nil { return nil, fmt.Errorf("failed to check IsRequired for addon task %s: %w", addonName, err) }
		if !isRequired {
			logger.Info("Skipping addon task as it's not required", "addon_name", addonName)
			continue
		}

		logger.Info("Planning addon task", "addon_name", addonName)
		taskFrag, err := addonTask.Plan(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to plan addon task %s: %w", addonName, err) }

		if taskFrag.IsEmpty() {
			logger.Info("Addon task returned an empty fragment, skipping merge.", "addon_name", addonName)
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil { return nil, fmt.Errorf("failed to merge fragment from addon task %s: %w", addonName, err) }

		// Addons can typically be installed in parallel.
		// Their entry nodes become part of the module's entry nodes (if not dependent on prior module stages).
		// Their exit nodes all contribute to the module's exit nodes.
		allAddonEntryNodes = append(allAddonEntryNodes, taskFrag.EntryNodes...)
		allAddonExitNodes = append(allAddonExitNodes, taskFrag.ExitNodes...)
	}

	moduleFragment.EntryNodes = task.UniqueNodeIDs(allAddonEntryNodes)
	moduleFragment.ExitNodes = task.UniqueNodeIDs(allAddonExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("AddonsModule planned no executable nodes.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Addons module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*AddonsModule)(nil)
