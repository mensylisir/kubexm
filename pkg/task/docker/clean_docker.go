package docker

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/docker"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanDockerTask struct {
	task.Base
}

func NewCleanDockerTask() task.Task {
	return &CleanDockerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanDocker",
				Description: "Stop, disable, and remove Docker and cri-dockerd components",
			},
		},
	}
}

func (t *CleanDockerTask) Name() string {
	return t.Meta.Name
}

func (t *CleanDockerTask) Description() string {
	return t.Meta.Description
}

func (t *CleanDockerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanDockerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return fragment, nil
	}

	stopCriDockerd := docker.NewStopCriDockerdStepBuilder(*runtimeCtx, "StopCriDockerd").Build()
	disableCriDockerd := docker.NewDisableCriDockerdStepBuilder(*runtimeCtx, "DisableCriDockerd").Build()
	removeCriDockerd := docker.NewRemoveCriDockerdStepBuilder(*runtimeCtx, "RemoveCriDockerd").Build()

	stopDocker := docker.NewStopDockerStepBuilder(*runtimeCtx, "StopDocker").Build()
	disableDocker := docker.NewDisableDockerStepBuilder(*runtimeCtx, "DisableDocker").Build()
	removeDocker := docker.NewRemoveDockerStepBuilder(*runtimeCtx, "RemoveDocker").Build()

	cleanDockerFiles := docker.NewCleanDockerStepBuilder(*runtimeCtx, "CleanDockerFiles").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopCriDockerd", Step: stopCriDockerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableCriDockerd", Step: disableCriDockerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCriDockerd", Step: removeCriDockerd, Hosts: deployHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "StopDocker", Step: stopDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableDocker", Step: disableDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveDocker", Step: removeDocker, Hosts: deployHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanDockerFiles", Step: cleanDockerFiles, Hosts: deployHosts})

	fragment.AddDependency("StopCriDockerd", "StopDocker")

	fragment.AddDependency("StopCriDockerd", "DisableCriDockerd")
	fragment.AddDependency("DisableCriDockerd", "RemoveCriDockerd")

	fragment.AddDependency("StopDocker", "DisableDocker")
	fragment.AddDependency("DisableDocker", "RemoveDocker")

	fragment.AddDependency("RemoveCriDockerd", "CleanDockerFiles")
	fragment.AddDependency("RemoveDocker", "CleanDockerFiles")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
