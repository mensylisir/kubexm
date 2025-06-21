package docker

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common" // For common.MasterRole, common.WorkerRole
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskDocker "github.com/mensylisir/kubexm/pkg/task/docker" // Import the new Docker task
)

// DockerModule installs and configures Docker and cri-dockerd.
type DockerModule struct {
	module.BaseModule
}

// NewDockerModule creates a new module for Docker and cri-dockerd.
func NewDockerModule() module.Module {
	// Define roles where Docker runtime should be installed
	dockerRoles := []string{common.MasterRole, common.WorkerRole}

	installDockerTask := taskDocker.NewInstallDockerTask(dockerRoles)

	modTasks := []task.Task{installDockerTask}
	base := module.NewBaseModule("DockerCriDockerdRuntime", modTasks)
	m := &DockerModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the Docker module.
func (m *DockerModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != v1alpha1.DockerRuntime {
		logger.Info("Docker module is not enabled (ContainerRuntime.Type is not 'docker'). Skipping.")
		return task.NewEmptyFragment(), nil
	}

	moduleFragment := task.NewExecutionFragment()
	var lastTaskExitNodes []plan.NodeID
	firstTask := true

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
			logger.Info("Skipping task as it's not required", "task_name", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task_name", currentTask.Name())
		taskFrag, err := currentTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if taskFrag == nil || len(taskFrag.Nodes) == 0 {
			logger.Info("Task returned an empty fragment, skipping merge", "task_name", currentTask.Name())
			continue
		}

		for id, node := range taskFrag.Nodes {
			if _, exists := moduleFragment.Nodes[id]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' from task '%s' in module '%s'", id, currentTask.Name(), m.Name())
			}
			moduleFragment.Nodes[id] = node
		}

		if !firstTask && len(lastTaskExitNodes) > 0 {
			for _, entryNodeID := range taskFrag.EntryNodes {
				if entryNode, ok := moduleFragment.Nodes[entryNodeID]; ok {
					existingDeps := make(map[plan.NodeID]bool)
					for _, dep := range entryNode.Dependencies { existingDeps[dep] = true }
					for _, prevExitNodeID := range lastTaskExitNodes {
						if !existingDeps[prevExitNodeID] {
							entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
						}
					}
				} else {
                    return nil, fmt.Errorf("entry node ID '%s' from task '%s' not found in module fragment", entryNodeID, currentTask.Name())
                }
			}
		} else if firstTask {
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFrag.EntryNodes...)
		}

		if len(taskFrag.ExitNodes) > 0 {
		    lastTaskExitNodes = taskFrag.ExitNodes
		    firstTask = false
		}
	}

	moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, lastTaskExitNodes...)
	moduleFragment.RemoveDuplicateNodeIDs()

	logger.Info("Docker module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

// Ensure DockerModule implements the module.Module interface.
var _ module.Module = (*DockerModule)(nil)
