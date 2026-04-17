package cni

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskNetwork "github.com/mensylisir/kubexm/internal/task/network"
)

// NetworkCleanupModule defines the module for cleaning up CNI components.
type NetworkCleanupModule struct {
	module.BaseModule
}

// NewNetworkCleanupModule creates a new NetworkCleanupModule.
func NewNetworkCleanupModule() module.Module {
	moduleTasks := []task.Task{
		taskNetwork.NewCleanNetworkPluginTask(),
	}

	base := module.NewBaseModule("NetworkCleanup", moduleTasks)
	m := &NetworkCleanupModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the Network Cleanup module.
func (m *NetworkCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check: Only run if CNI was configured.
	pluginName := "not specified"
	if clusterConfig.Spec.Network == nil || clusterConfig.Spec.Network.Plugin == "" {
		logger.Info("CNI plugin not specified or network spec missing. Skipping CNI Cleanup module planning.")
		return plan.NewEmptyFragment(m.Name()), nil
	}
	pluginName = clusterConfig.Spec.Network.Plugin
	logger.Infof("Planning CNI Cleanup module for plugin: %s...", pluginName)

	moduleFragment, _, err := m.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan network cleanup module: %w", err)
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("CNI Cleanup module planned no executable nodes.")
	} else {
		logger.Info("CNI Cleanup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*NetworkCleanupModule)(nil)
