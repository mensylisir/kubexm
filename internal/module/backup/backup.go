package backup

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	backuptask "github.com/mensylisir/kubexm/internal/task/backup"
)

// BackupModule handles cluster backup operations.
type BackupModule struct {
	module.BaseModule
	backupType string
}

func NewBackupModule(backupType string) module.Module {
	var tasks []task.Task

	switch backupType {
	case "all":
		tasks = []task.Task{
			&backupTaskWrapper{backuptask.NewBackupPKITask()},
			&backupTaskWrapper{backuptask.NewBackupEtcdTask()},
			&backupTaskWrapper{backuptask.NewBackupK8sConfigsTask()},
		}
	case "pki":
		tasks = []task.Task{&backupTaskWrapper{backuptask.NewBackupPKITask()}}
	case "etcd":
		tasks = []task.Task{&backupTaskWrapper{backuptask.NewBackupEtcdTask()}}
	case "kubernetes":
		tasks = []task.Task{&backupTaskWrapper{backuptask.NewBackupK8sConfigsTask()}}
	default:
		tasks = []task.Task{
			&backupTaskWrapper{backuptask.NewBackupPKITask()},
			&backupTaskWrapper{backuptask.NewBackupEtcdTask()},
			&backupTaskWrapper{backuptask.NewBackupK8sConfigsTask()},
		}
	}

	return &BackupModule{
		BaseModule: module.NewBaseModule("Backup", tasks),
		backupType: backupType,
	}
}

func (m *BackupModule) Name() string        { return "Backup" }
func (m *BackupModule) Description() string { return fmt.Sprintf("Backup cluster data (%s)", m.backupType) }

func (m *BackupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "backup_type", m.backupType)
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
			logger.Info("Backup task is not required, skipping", "task", t.Name())
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
		logger.Info("Backup module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("Backup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

type backupTaskWrapper struct{ task.Task }

var _ module.Module = (*BackupModule)(nil)
