package openebs_local

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	openebsstep "github.com/mensylisir/kubexm/internal/step/storage/openebs-local"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy openebs")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := openebsstep.NewGenerateOpenEBSValuesStepBuilder(runtimeCtx, "GenerateOpenEBSValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := openebsstep.NewDistributeOpenEBSArtifactsStepBuilder(runtimeCtx, "DistributeOpenEBSArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := openebsstep.NewInstallOpenEBSHelmChartStepBuilder(runtimeCtx, "InstallOpenEBSChart").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateOpenEBSValues", Step: generateValues, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeOpenEBSArtifacts", Step: distributeArtifacts, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallOpenEBSChart", Step: installChart, Hosts: []remotefw.Host{executionHost}})

	fragment.AddDependency("GenerateOpenEBSValues", "DistributeOpenEBSArtifacts")
	fragment.AddDependency("DistributeOpenEBSArtifacts", "InstallOpenEBSChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
