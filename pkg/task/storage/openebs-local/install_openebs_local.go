package openebs_local

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	openebsstep "github.com/mensylisir/kubexm/pkg/step/storage/openebs-local"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployOpenebsTask struct {
	task.Base
}

func NewDeployOpenebsTask() task.Task {
	return &DeployOpenebsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployOpenebs",
				Description: "Deploy OpenEBS using Helm",
			},
		},
	}
}

func (t *DeployOpenebsTask) Name() string {
	return t.Meta.Name
}

func (t *DeployOpenebsTask) Description() string {
	return t.Meta.Description
}

func (t *DeployOpenebsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.OpenEBS == nil || cfg.Spec.Storage.OpenEBS.Enabled == nil {
		return false, nil
	}
	return *cfg.Spec.Storage.OpenEBS.Enabled, nil
}

func (t *DeployOpenebsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy openebs")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := openebsstep.NewGenerateOpenEBSValuesStepBuilder(*runtimeCtx, "GenerateOpenEBSValues").Build()
	distributeArtifacts := openebsstep.NewDistributeOpenEBSArtifactsStepBuilder(*runtimeCtx, "DistributeOpenEBSArtifacts").Build()
	installChart := openebsstep.NewInstallOpenEBSHelmChartStepBuilder(*runtimeCtx, "InstallOpenEBSChart").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateOpenEBSValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeOpenEBSArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallOpenEBSChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateOpenEBSValues", "DistributeOpenEBSArtifacts")
	fragment.AddDependency("DistributeOpenEBSArtifacts", "InstallOpenEBSChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading OpenEBS chart.")
		downloadChart := openebsstep.NewDownloadOpenEBSChartStepBuilder(*runtimeCtx, "DownloadOpenEBSChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadOpenEBSChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadOpenEBSChart", "GenerateOpenEBSValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download for OpenEBS artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
