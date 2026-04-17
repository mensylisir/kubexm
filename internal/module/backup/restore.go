package backup

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	restoretask "github.com/mensylisir/kubexm/internal/task/backup"
)

// RestoreModule handles cluster restore operations.
type RestoreModule struct {
	module.BaseModule
	restoreType   string
	snapshotPath string
}

func NewRestoreModule(restoreType, snapshotPath string) module.Module {
	var tasks []task.Task

	switch restoreType {
	case "all":
		tasks = []task.Task{
			&backupTaskWrapper{restoretask.NewRestorePKITask()},
			&backupTaskWrapper{restoretask.NewRestoreEtcdTask(snapshotPath)},
			&backupTaskWrapper{restoretask.NewRestoreK8sConfigsTask()},
		}
	case "pki":
		tasks = []task.Task{&backupTaskWrapper{restoretask.NewRestorePKITask()}}
	case "etcd":
		tasks = []task.Task{&backupTaskWrapper{restoretask.NewRestoreEtcdTask(snapshotPath)}}
	case "kubernetes":
		tasks = []task.Task{&backupTaskWrapper{restoretask.NewRestoreK8sConfigsTask()}}
	default:
		tasks = []task.Task{
			&backupTaskWrapper{restoretask.NewRestorePKITask()},
			&backupTaskWrapper{restoretask.NewRestoreEtcdTask(snapshotPath)},
			&backupTaskWrapper{restoretask.NewRestoreK8sConfigsTask()},
		}
	}

	return &RestoreModule{
		BaseModule:   module.NewBaseModule("Restore", tasks),
		restoreType:  restoreType,
		snapshotPath: snapshotPath,
	}
}

func (m *RestoreModule) Name() string        { return "Restore" }
func (m *RestoreModule) Description() string { return fmt.Sprintf("Restore cluster data (%s)", m.restoreType) }

func (m *RestoreModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "restore_type", m.restoreType)
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
			logger.Info("Restore task is not required, skipping", "task", t.Name())
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
		logger.Info("Restore module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("Restore module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*RestoreModule)(nil)
