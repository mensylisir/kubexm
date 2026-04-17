package argocd

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	argocdstep "github.com/mensylisir/kubexm/internal/step/cd/argocd"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployArgoCDTask struct {
	task.Base
}

func NewDeployArgoCDTask() task.Task {
	return &DeployArgoCDTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployArgoCD",
				Description: "Deploy Argo CD using Helm",
			},
		},
	}
}

func (t *DeployArgoCDTask) Name() string {
	return t.Meta.Name
}

func (t *DeployArgoCDTask) Description() string {
	return t.Meta.Description
}

func (t *DeployArgoCDTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	for _, v := range cfg.Spec.Addons {
		if v.Name == "argocd" {
			return true, nil
		}
	}
	return false, nil
}

func (t *DeployArgoCDTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy argo-cd")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := argocdstep.NewGenerateArgoCDValuesStepBuilder(runtimeCtx, "GenerateArgoCDValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := argocdstep.NewDistributeArgoCDArtifactsStepBuilder(runtimeCtx, "DistributeArgoCDArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := argocdstep.NewInstallArgoCDHelmChartStepBuilder(runtimeCtx, "InstallArgoCDChart").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateArgoCDValues", Step: generateValues, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeArgoCDArtifacts", Step: distributeArtifacts, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallArgoCDChart", Step: installChart, Hosts: []remotefw.Host{executionHost}})

	fragment.AddDependency("GenerateArgoCDValues", "DistributeArgoCDArtifacts")
	fragment.AddDependency("DistributeArgoCDArtifacts", "InstallArgoCDChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
