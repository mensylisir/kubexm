package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	networktask "github.com/mensylisir/kubexm/internal/task/network"
)

// NetworkUpgradeModule handles CNI plugin upgrades during cluster upgrade.
// It reuses the InstallNetworkPluginTask since helm upgrade --install handles both install and upgrade.
type NetworkUpgradeModule struct {
	module.BaseModule
}

func NewNetworkUpgradeModule() module.Module {
	return &NetworkUpgradeModule{
		BaseModule: module.NewBaseModule("NetworkUpgrade", nil),
	}
}

func (m *NetworkUpgradeModule) Name() string {
	return "NetworkUpgrade"
}

func (m *NetworkUpgradeModule) Description() string {
	return "Upgrades the configured CNI network plugin to a new version"
}

func (m *NetworkUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name()+"-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	// Reuse InstallNetworkPluginTask since helm upgrade --install handles both install and upgrade
	upgradeTask := networktask.NewInstallNetworkPluginTask()
	taskFrag, err := upgradeTask.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan network upgrade task: %w", err)
	}

	if taskFrag.IsEmpty() {
		logger.Info("No network plugin to upgrade, skipping")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	if err := moduleFragment.MergeFragment(taskFrag); err != nil {
		return nil, fmt.Errorf("failed to merge fragment from network upgrade task: %w", err)
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("NetworkUpgrade module planning complete", "nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

func (m *NetworkUpgradeModule) GetBase() *module.BaseModule {
	return &m.BaseModule
}

var _ module.Module = (*NetworkUpgradeModule)(nil)
