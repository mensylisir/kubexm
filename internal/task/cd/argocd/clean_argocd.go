package argocd

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	argocdstep "github.com/mensylisir/kubexm/internal/step/cd/argocd"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanArgoCDTask struct {
	task.Base
}

func NewCleanArgoCDTask() task.Task {
	return &CleanArgoCDTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanArgoCD",
				Description: "Clean up Argo CD using Helm",
			},
		},
	}
}

func (t *CleanArgoCDTask) Name() string {
	return t.Meta.Name
}

func (t *CleanArgoCDTask) Description() string {
	return t.Meta.Description
}

func (t *CleanArgoCDTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	for _, v := range cfg.Spec.Addons {
		if v.Name == "argocd" {
			return true, nil
		}
	}
	return false, nil
}

func (t *CleanArgoCDTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := argocdstep.NewCleanArgoCDStepBuilder(runtimeCtx, "UninstallArgoCDRelease").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallArgoCDRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
