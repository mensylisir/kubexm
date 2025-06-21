package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
)

// ContainerdModule installs and configures containerd.
type ContainerdModule struct {
	module.BaseModule
	// cfg *v1alpha1.Cluster // Stored if needed by Plan for IsEnabled logic, or tasks constructed in Plan
}

// NewContainerdModule creates a new module for containerd.
func NewContainerdModule(cfg *v1alpha1.Cluster) module.Module {
	// Define roles where containerd should be installed (typically all nodes that run containers)
	containerdRoles := []string{common.MasterRole, common.WorkerRole} // Adjust if etcd nodes also run kubelet with containerd

	// Instantiate tasks. Assumes NewInstallContainerdTask constructor is updated.
	// Example: taskContainerd.NewInstallContainerdTask(version, arch, zone, downloadDir, checksum, mirrors, insecure, useSystemdCgroup, extraToml, configPath, roles)
	// The specific parameters for NewInstallContainerdTask would be derived from cfg.Spec.ContainerRuntime and cfg.Spec.Hosts, etc.
	// For simplicity, we pass cfg and let the task extract details or assume the task constructor is adapted.
	// The refactored task takes `cfg *v1alpha1.Cluster` and `roles []string`
	// The current `taskContainerd.NewInstallContainerdTask` takes many specific args.
	// This needs to be aligned. Assuming `NewInstallContainerdTask` is refactored to take `cfg` and roles,
	// and derives its specific params internally from `cfg`.
	installTask := taskContainerd.NewInstallContainerdTask(
		cfg.Spec.ContainerRuntime.Version, // Example: Get version from cfg
		"",                                 // Arch (task can derive)
		cfg.Spec.ImageHub.Zone,             // Zone for downloads
		"",                                 // DownloadDir (task can use default)
		cfg.Spec.ContainerRuntime.Checksum, // Checksum
		cfg.Spec.ContainerRuntime.RegistryMirrors,
		cfg.Spec.ContainerRuntime.InsecureRegistries,
		cfg.Spec.ContainerRuntime.UseSystemdCgroup,
		cfg.Spec.ContainerRuntime.ExtraToml,
		cfg.Spec.ContainerRuntime.ConfigPath,
		containerdRoles,
	)

	modTasks := []task.Task{installTask}
	base := module.NewBaseModule("ContainerdRuntime", modTasks)
	// m := &ContainerdModule{BaseModule: base, cfg: cfg} // Store cfg if needed for IsEnabled in Plan
	m := &ContainerdModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the containerd module.
func (m *ContainerdModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig() // Get *v1alpha1.Cluster from context

	// Enablement Check
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != common.Containerd {
		logger.Info("Containerd module is not enabled (ContainerRuntime.Type is not 'containerd'). Skipping.")
		return &task.ExecutionFragment{Nodes: make(map[plan.NodeID]*plan.ExecutionNode)}, nil // Return empty fragment
	}

	moduleFragment := &task.ExecutionFragment{
		Nodes:      make(map[plan.NodeID]*plan.ExecutionNode),
		EntryNodes: []plan.NodeID{},
		ExitNodes:  []plan.NodeID{},
	}

	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	for _, currentTask := range m.Tasks() {
		taskCtx, ok := ctx.(runtime.TaskContext)
		if !ok {
			return nil, fmt.Errorf("module context cannot be asserted to task context for module %s, task %s", m.Name(), currentTask.Name())
		}

		taskIsRequired, err := currentTask.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !taskIsRequired {
			logger.Info("Skipping task as it's not required", "task", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task", currentTask.Name())
		taskFragment, err := currentTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if taskFragment == nil || len(taskFragment.Nodes) == 0 {
			logger.Info("Task returned an empty fragment, skipping", "task", currentTask.Name())
			continue
		}

		for id, node := range taskFragment.Nodes {
			if _, exists := moduleFragment.Nodes[id]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' from task '%s' in module '%s'", id, currentTask.Name(), m.Name())
			}
			moduleFragment.Nodes[id] = node
		}

		if len(previousTaskExitNodes) > 0 {
			for _, entryNodeID := range taskFragment.EntryNodes {
				entryNode, found := moduleFragment.Nodes[entryNodeID]
				if !found {
					return nil, fmt.Errorf("entry node '%s' from task '%s' not found after merge in module '%s'", entryNodeID, currentTask.Name(), m.Name())
				}
				existingDeps := make(map[plan.NodeID]bool)
				for _, depID := range entryNode.Dependencies { existingDeps[depID] = true }
				for _, prevExitNodeID := range previousTaskExitNodes {
					if !existingDeps[prevExitNodeID] {
						entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
					}
				}
			}
		} else if isFirstEffectiveTask {
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFragment.EntryNodes...)
		}

		if len(taskFragment.ExitNodes) > 0 {
			previousTaskExitNodes = taskFragment.ExitNodes
			isFirstEffectiveTask = false
		}
	}

	moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, previousTaskExitNodes...)
	moduleFragment.EntryNodes = uniqueNodeIDs(moduleFragment.EntryNodes) // Deduplicate
	moduleFragment.ExitNodes = uniqueNodeIDs(moduleFragment.ExitNodes)   // Deduplicate

	logger.Info("Containerd module planning complete.", "totalNodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

// uniqueNodeIDs helper (can be moved to a common utility package if used elsewhere)
func uniqueNodeIDs(ids []plan.NodeID) []plan.NodeID {
	if len(ids) == 0 {
		return []plan.NodeID{}
	}
	seen := make(map[plan.NodeID]bool)
	result := []plan.NodeID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// Ensure ContainerdModule implements the module.Module interface.
var _ module.Module = (*ContainerdModule)(nil)
