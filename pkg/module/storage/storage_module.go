package storage

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/task"
	longhornstorage "github.com/mensylisir/kubexm/pkg/task/storage/longhorn"
	nfsstorage "github.com/mensylisir/kubexm/pkg/task/storage/nfs"
	openebsstorage "github.com/mensylisir/kubexm/pkg/task/storage/openebs-local"
)

// StorageModule handles storage class deployment based on configuration
// Supports: Longhorn, NFS, OpenEBS Local PV, OpenEBS Local PV (Rook Ceph)
type StorageModule struct {
	module.BaseModule
}

// NewStorageModule creates a new StorageModule
func NewStorageModule() module.Module {
	tasks := []task.Task{
		NewDeployStorageTask(),
	}
	base := module.NewBaseModule("StorageSetup", tasks)
	return &StorageModule{BaseModule: base}
}

func (m *StorageModule) Name() string {
	return "StorageSetup"
}

func (m *StorageModule) Description() string {
	return "Deploys storage class (Longhorn/NFS/OpenEBS) based on cluster configuration"
}

func (m *StorageModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		logger.Info("No cluster config found, skipping storage")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	storageSpec := clusterCfg.Spec.Storage
	if storageSpec == nil {
		logger.Info("No storage config found, skipping storage")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	// Check if any storage provider is enabled
	hasEnabledStorage := (storageSpec.Longhorn != nil && storageSpec.Longhorn.Enabled != nil && *storageSpec.Longhorn.Enabled) ||
		(storageSpec.NFS != nil && storageSpec.NFS.Enabled != nil && *storageSpec.NFS.Enabled) ||
		(storageSpec.OpenEBS != nil && storageSpec.OpenEBS.Enabled != nil && *storageSpec.OpenEBS.Enabled) ||
		(storageSpec.RookCeph != nil && storageSpec.RookCeph.Enabled != nil && *storageSpec.RookCeph.Enabled)

	if !hasEnabledStorage {
		logger.Info("No storage provider enabled, skipping")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	storageTask := NewDeployStorageTask()
	storageFrag, err := storageTask.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan storage task: %w", err)
	}

	if storageFrag.IsEmpty() {
		logger.Info("Storage task returned empty fragment, skipping")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	if err := moduleFragment.MergeFragment(storageFrag); err != nil {
		return nil, err
	}

	moduleFragment.EntryNodes = storageFrag.EntryNodes
	moduleFragment.ExitNodes = storageFrag.ExitNodes

	return moduleFragment, nil
}

func (m *StorageModule) GetBase() *module.BaseModule {
	return &m.BaseModule
}

// DeployStorageTask orchestrates storage deployment
type DeployStorageTask struct {
	task.Base
}

func NewDeployStorageTask() task.Task {
	return &DeployStorageTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployStorage",
				Description: "Deploys storage class based on cluster configuration",
			},
		},
	}
}

func (t *DeployStorageTask) Name() string {
	return t.Meta.Name
}

func (t *DeployStorageTask) Description() string {
	return t.Meta.Description
}

func (t *DeployStorageTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg == nil || cfg.Spec.Storage == nil {
		return false, nil
	}

	storageSpec := cfg.Spec.Storage
	// Check if any storage provider is enabled
	return (storageSpec.Longhorn != nil && storageSpec.Longhorn.Enabled != nil && *storageSpec.Longhorn.Enabled) ||
		(storageSpec.NFS != nil && storageSpec.NFS.Enabled != nil && *storageSpec.NFS.Enabled) ||
		(storageSpec.OpenEBS != nil && storageSpec.OpenEBS.Enabled != nil && *storageSpec.OpenEBS.Enabled) ||
		(storageSpec.RookCeph != nil && storageSpec.RookCeph.Enabled != nil && *storageSpec.RookCeph.Enabled), nil
}

func (t *DeployStorageTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	cfg := ctx.GetClusterConfig()
	if cfg == nil || cfg.Spec.Storage == nil {
		return plan.NewEmptyFragment(t.Name()), nil
	}

	storageSpec := cfg.Spec.Storage

	// Check if any storage provider is enabled
	hasEnabledStorage := (storageSpec.Longhorn != nil && storageSpec.Longhorn.Enabled != nil && *storageSpec.Longhorn.Enabled) ||
		(storageSpec.NFS != nil && storageSpec.NFS.Enabled != nil && *storageSpec.NFS.Enabled) ||
		(storageSpec.OpenEBS != nil && storageSpec.OpenEBS.Enabled != nil && *storageSpec.OpenEBS.Enabled) ||
		(storageSpec.RookCeph != nil && storageSpec.RookCeph.Enabled != nil && *storageSpec.RookCeph.Enabled)

	if !hasEnabledStorage {
		return plan.NewEmptyFragment(t.Name()), nil
	}

	var tasks []task.Task

	// Add tasks for each enabled storage provider
	if storageSpec.Longhorn != nil && storageSpec.Longhorn.Enabled != nil && *storageSpec.Longhorn.Enabled {
		tasks = append(tasks, longhornstorage.NewDeployLonghornTask())
	}
	if storageSpec.NFS != nil && storageSpec.NFS.Enabled != nil && *storageSpec.NFS.Enabled {
		tasks = append(tasks, nfsstorage.NewDeployNfsTask())
	}
	if storageSpec.OpenEBS != nil && storageSpec.OpenEBS.Enabled != nil && *storageSpec.OpenEBS.Enabled {
		tasks = append(tasks, openebsstorage.NewDeployOpenebsTask())
	}
	// RookCeph would be added here when its task package is available

	var previousTaskExitNodes []plan.NodeID

	for _, st := range tasks {
		taskFrag, err := st.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan storage task %s: %w", st.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}
		if err := fragment.MergeFragment(taskFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(fragment, previousTaskExitNodes, taskFrag.EntryNodes)
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		return plan.NewEmptyFragment(t.Name()), nil
	}

	return fragment, nil
}

func (t *DeployStorageTask) GetBase() *task.Base {
	return &t.Base
}
