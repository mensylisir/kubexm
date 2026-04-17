package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/preflight"
)

// PreflightConnectivityModule defines the module for checking SSH connectivity to all hosts.
// This module should run before any other module in every pipeline.
type PreflightConnectivityModule struct {
	module.BaseModule
}

// NewPreflightConnectivityModule creates a new PreflightConnectivityModule.
func NewPreflightConnectivityModule() module.Module {
	tasks := []task.Task{
		preflight.NewCheckConnectivityTask(),
	}
	return &PreflightConnectivityModule{
		BaseModule: module.NewBaseModule("PreflightConnectivity", tasks),
	}
}

// Tasks returns the list of tasks for this module.
func (m *PreflightConnectivityModule) Tasks() []task.Task {
	return m.ModuleTasks
}

// Plan generates the execution fragment for the connectivity check module.
func (m *PreflightConnectivityModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	moduleFragment, _, err := m.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan preflight connectivity module: %w", err)
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Preflight connectivity module planned no executable nodes.")
	} else {
		logger.Info("Preflight connectivity module planning complete.", "totalNodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

// Ensure PreflightConnectivityModule implements the module.Module interface.
var _ module.Module = (*PreflightConnectivityModule)(nil)