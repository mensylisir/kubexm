package crio

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/crio"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanCrioTask struct {
	task.Base
}

func NewCleanCrioTask() task.Task {
	return &CleanCrioTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanCrio",
				Description: "Stop, disable, and remove CRI-O and its related components",
			},
		},
	}
}

func (t *CleanCrioTask) Name() string {
	return t.Meta.Name
}

func (t *CleanCrioTask) Description() string {
	return t.Meta.Description
}

func (t *CleanCrioTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanCrioTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return fragment, nil
	}

	stopCrio, err := crio.NewStopCrioStepBuilder(runtimeCtx, "StopCrio").Build()
	if err != nil {
		return nil, err
	}
	disableCrio, err := crio.NewDisableCrioStepBuilder(runtimeCtx, "DisableCrio").Build()
	if err != nil {
		return nil, err
	}

	cleanCrioFiles, err := crio.NewCleanCrioStepBuilder(runtimeCtx, "CleanCrioFiles").
		WithPurgeData(false).
		Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopCrio", Step: stopCrio, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableCrio", Step: disableCrio, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanCrioFiles", Step: cleanCrioFiles, Hosts: deployHosts})

	fragment.AddDependency("StopCrio", "DisableCrio")
	fragment.AddDependency("DisableCrio", "CleanCrioFiles")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
