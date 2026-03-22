package docker

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/docker"
	"github.com/mensylisir/kubexm/internal/task"
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

	stopCriDockerd, err := docker.NewStopCriDockerdStepBuilder(runtimeCtx, "StopCriDockerd").Build()
	if err != nil {
		return nil, err
	}
	disableCriDockerd, err := docker.NewDisableCriDockerdStepBuilder(runtimeCtx, "DisableCriDockerd").Build()
	if err != nil {
		return nil, err
	}
	removeCriDockerd, err := docker.NewRemoveCriDockerdStepBuilder(runtimeCtx, "RemoveCriDockerd").Build()
	if err != nil {
		return nil, err
	}

	stopDocker, err := docker.NewStopDockerStepBuilder(runtimeCtx, "StopDocker").Build()
	if err != nil {
		return nil, err
	}
	disableDocker, err := docker.NewDisableDockerStepBuilder(runtimeCtx, "DisableDocker").Build()
	if err != nil {
		return nil, err
	}
	removeDocker, err := docker.NewRemoveDockerStepBuilder(runtimeCtx, "RemoveDocker").Build()
	if err != nil {
		return nil, err
	}

	cleanDockerFiles, err := docker.NewCleanDockerStepBuilder(runtimeCtx, "CleanDockerFiles").Build()
	if err != nil {
		return nil, err
	}

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
