package storage

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	longhornstorage "github.com/mensylisir/kubexm/internal/task/storage/longhorn"
	nfsstorage "github.com/mensylisir/kubexm/internal/task/storage/nfs"
	openebsstorage "github.com/mensylisir/kubexm/internal/task/storage/openebs-local"
)

// StorageCleanupModule handles storage class cleanup during cluster deletion.
// It orchestrates cleanup tasks for Longhorn, NFS, and OpenEBS storage providers.
type StorageCleanupModule struct {
	module.BaseModule
}

// NewStorageCleanupModule creates a new StorageCleanupModule
func NewStorageCleanupModule() module.Module {
	base := module.NewBaseModule("StorageCleanup", []task.Task{})
	return &StorageCleanupModule{BaseModule: base}
}

func (m *StorageCleanupModule) Name() string {
	return "StorageCleanup"
}

func (m *StorageCleanupModule) Description() string {
	return "Cleans up storage resources (Longhorn/NFS/OpenEBS) during cluster deletion"
}

// Tasks returns empty slice since tasks are built dynamically in Plan()
func (m *StorageCleanupModule) Tasks() []task.Task {
	return []task.Task{}
}

func (m *StorageCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		logger.Info("No cluster config found, skipping storage cleanup")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	storageSpec := clusterCfg.Spec.Storage
	if storageSpec == nil {
		logger.Info("No storage config found, skipping storage cleanup")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	// Build list of cleanup tasks based on what was enabled
	var cleanupTasks []task.Task

	if storageSpec.Longhorn != nil && storageSpec.Longhorn.Enabled != nil && *storageSpec.Longhorn.Enabled {
		cleanupTasks = append(cleanupTasks, longhornstorage.NewCleanLonghornTask())
	}
	if storageSpec.NFS != nil && storageSpec.NFS.Enabled != nil && *storageSpec.NFS.Enabled {
		cleanupTasks = append(cleanupTasks, nfsstorage.NewCleanNfsTask())
	}
	if storageSpec.OpenEBS != nil && storageSpec.OpenEBS.Enabled != nil && *storageSpec.OpenEBS.Enabled {
		cleanupTasks = append(cleanupTasks, openebsstorage.NewCleanOpenebsTask())
	}
	// RookCeph cleanup would be added here when available

	if len(cleanupTasks) == 0 {
		logger.Info("No storage provider cleanup tasks to plan")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	var previousTaskExitNodes []plan.NodeID

	for _, ct := range cleanupTasks {
		taskFrag, err := ct.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan storage cleanup task %s: %w", ct.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}
		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment for task %s: %w", ct.Name(), err)
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for task %s: %w", ct.Name(), err)
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		logger.Info("Storage cleanup module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	return moduleFragment, nil
}

func (m *StorageCleanupModule) GetBase() *module.BaseModule {
	return &m.BaseModule
}

// Ensure StorageCleanupModule implements module.Module interface
var _ module.Module = (*StorageCleanupModule)(nil)
