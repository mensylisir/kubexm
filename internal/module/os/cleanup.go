package os

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskos "github.com/mensylisir/kubexm/internal/task/os"
)

// OsCleanupModule defines the module for OS-level cleanup on all nodes.
type OsCleanupModule struct {
	module.BaseModule
}

// NewOsCleanupModule creates a new OsCleanupModule.
func NewOsCleanupModule() module.Module {
	tasks := []task.Task{
		taskos.NewCleanOSNodesTask(),
	}
	return &OsCleanupModule{
		BaseModule: module.NewBaseModule("OsCleanup", tasks),
	}
}

// Tasks returns the list of tasks for this module.
func (m *OsCleanupModule) Tasks() []task.Task {
	return m.ModuleTasks
}

// Plan generates the execution fragment for the OS cleanup module.
func (m *OsCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	moduleFragment, _, err := m.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan OS cleanup module: %w", err)
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("OS cleanup module planned no executable nodes.")
	} else {
		logger.Info("OS cleanup module planning complete.", "totalNodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

// Ensure OsCleanupModule implements the module.Module interface.
var _ module.Module = (*OsCleanupModule)(nil)
