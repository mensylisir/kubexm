package registry

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	registrytask "github.com/mensylisir/kubexm/internal/task/registry"
)

// RegistryModule handles registry deployment and cleanup operations.
type RegistryModule struct {
	module.BaseModule
	operation string // "install" or "uninstall"
}

func NewRegistryModule(operation string) module.Module {
	var tasks []task.Task

	switch operation {
	case "install":
		tasks = []task.Task{
			&registryTaskWrapper{registrytask.NewInstallRegistryTask()},
			&registryTaskWrapper{registrytask.NewGenerateRegistryConfigTask()},
			&registryTaskWrapper{registrytask.NewDistributeRegistryConfigTask()},
			&registryTaskWrapper{registrytask.NewSetupRegistryServiceTask()},
			&registryTaskWrapper{registrytask.NewStartRegistryServiceTask()},
		}
	case "uninstall":
		tasks = []task.Task{
			&registryTaskWrapper{registrytask.NewStopRegistryServiceTask()},
			&registryTaskWrapper{registrytask.NewDisableRegistryServiceTask()},
			&registryTaskWrapper{registrytask.NewRemoveRegistryArtifactsTask()},
			&registryTaskWrapper{registrytask.NewRemoveRegistryDataTask()},
		}
	default:
		tasks = []task.Task{
			&registryTaskWrapper{registrytask.NewInstallRegistryTask()},
			&registryTaskWrapper{registrytask.NewGenerateRegistryConfigTask()},
			&registryTaskWrapper{registrytask.NewDistributeRegistryConfigTask()},
			&registryTaskWrapper{registrytask.NewSetupRegistryServiceTask()},
			&registryTaskWrapper{registrytask.NewStartRegistryServiceTask()},
		}
	}

	return &RegistryModule{
		BaseModule: module.NewBaseModule("Registry", tasks),
		operation:  operation,
	}
}

func (m *RegistryModule) Name() string        { return "Registry" }
func (m *RegistryModule) Description() string { return fmt.Sprintf("Registry %s module", m.operation) }

func (m *RegistryModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "operation", m.operation)
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	var previousTaskExitNodes []plan.NodeID

	for _, t := range m.Tasks() {
		isRequired, err := t.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required: %w", t.Name(), err)
		}
		if !isRequired {
			logger.Debug("Registry task is not required, skipping", "task", t.Name())
			continue
		}

		taskFrag, err := t.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s: %w", t.Name(), err)
		}

		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for task %s: %w", t.Name(), err)
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		logger.Info("Registry module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("Registry module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

type registryTaskWrapper struct{ task.Task }

var _ module.Module = (*RegistryModule)(nil)
