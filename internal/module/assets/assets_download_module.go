package assets

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskpre "github.com/mensylisir/kubexm/internal/task/pre"
	taskpreflight "github.com/mensylisir/kubexm/internal/task/preflight"
)

// AssetsDownloadModule downloads and packages offline assets on the control node.
type AssetsDownloadModule struct {
	module.BaseModule
	outputPath string
}

func NewAssetsDownloadModule(outputPath string) module.Module {
	return &AssetsDownloadModule{
		BaseModule: module.NewBaseModule("AssetsDownload", nil),
		outputPath: outputPath,
	}
}

func (m *AssetsDownloadModule) GetTasks() []task.Task {
	return []task.Task{
		taskpreflight.NewPrepareAssetsTask(),
		taskpre.NewVerifyArtifactsTask(),
		taskpreflight.NewPackageAssetsTask(m.outputPath),
	}
}

func (m *AssetsDownloadModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	definedTasks := m.GetTasks()

	var previousTaskExitNodes []plan.NodeID
	for i, currentTask := range definedTasks {
		isRequired, err := currentTask.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !isRequired {
			logger.Debug("Skipping non-required task", "task", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task_name", currentTask.Name())
		taskFrag, err := currentTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if taskFrag == nil || taskFrag.IsEmpty() {
			logger.Debug("Task planned an empty fragment, skipping merge/link", "task", currentTask.Name())
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s into module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if i > 0 && len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes)
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	moduleFragment.CalculateEntryAndExitNodes()
	return moduleFragment, nil
}

var _ module.Module = (*AssetsDownloadModule)(nil)
