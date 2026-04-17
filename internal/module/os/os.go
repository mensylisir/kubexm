package os

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskos "github.com/mensylisir/kubexm/internal/task/os"
)

// OsModule defines the module for OS-level configuration on all nodes.
type OsModule struct {
	module.BaseModule
}

// NewOsModule creates a new OsModule.
func NewOsModule() module.Module {
	tasks := []task.Task{
		taskos.NewConfigureHostTask(),   // SetHostname + UpdateEtcHosts
		taskos.NewDisableServicesTask(), // DisableSwap + DisableFirewall + DisableSelinux
		taskos.NewConfigureKernelTask(), // LoadKernelModules + ConfigureSysctl
	}
	return &OsModule{
		BaseModule: module.NewBaseModule("OSConfiguration", tasks),
	}
}

// Tasks returns the list of tasks for this module.
func (m *OsModule) Tasks() []task.Task {
	return m.ModuleTasks
}

// Plan generates the execution fragment for the OS module.
func (m *OsModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	moduleFragment, _, err := m.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan OS module: %w", err)
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("OS module planned no executable nodes.")
	} else {
		logger.Info("OS module planning complete.", "totalNodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

// Ensure OsModule implements the module.Module interface.
var _ module.Module = (*OsModule)(nil)
