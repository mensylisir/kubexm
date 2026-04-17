package packages

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/packages"
)

// DownloadModule handles downloading all required assets for offline installation.
type DownloadModule struct {
	module.BaseModule
}

// NewDownloadModule creates a new DownloadModule.
func NewDownloadModule() module.Module {
	tasks := []task.Task{
		packages.NewDownloadAssetsTask(),
	}
	return &DownloadModule{
		BaseModule: module.NewBaseModule("Download", tasks),
	}
}

// Tasks returns the list of tasks for this module.
func (m *DownloadModule) Tasks() []task.Task {
	return m.ModuleTasks
}

// Plan generates the execution fragment for the download module.
func (m *DownloadModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	moduleFragment, _, err := m.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan download module: %w", err)
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Download module planned no executable nodes.")
	} else {
		logger.Info("Download module planning complete.", "totalNodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

// Ensure DownloadModule implements the module.Module interface.
var _ module.Module = (*DownloadModule)(nil)
