package argocd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	argocdstep "github.com/mensylisir/kubexm/pkg/step/cd/argocd"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy argo-cd")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := argocdstep.NewGenerateArgoCDValuesStepBuilder(*runtimeCtx, "GenerateArgoCDValues").Build()
	distributeArtifacts := argocdstep.NewDistributeArgoCDArtifactsStepBuilder(*runtimeCtx, "DistributeArgoCDArtifacts").Build()
	installChart := argocdstep.NewInstallArgoCDHelmChartStepBuilder(*runtimeCtx, "InstallArgoCDChart").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateArgoCDValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeArgoCDArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallArgoCDChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateArgoCDValues", "DistributeArgoCDArtifacts")
	fragment.AddDependency("DistributeArgoCDArtifacts", "InstallArgoCDChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading Argo CD chart.")
		downloadChart := argocdstep.NewDownloadArgoCDChartStepBuilder(*runtimeCtx, "DownloadArgoCDChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadArgoCDChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadArgoCDChart", "GenerateArgoCDValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download for Argo CD artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
