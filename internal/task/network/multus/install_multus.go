package multus

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	multusstep "github.com/mensylisir/kubexm/internal/step/network/multus"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployMultusTask struct {
	task.Base
}

func NewDeployMultusTask() task.Task {
	return &DeployMultusTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployMultus",
				Description: "Deploy Multus CNI meta-plugin using Helm",
			},
		},
	}
}

func (t *DeployMultusTask) Name() string {
	return t.Meta.Name
}

func (t *DeployMultusTask) Description() string {
	return t.Meta.Description
}

func (t *DeployMultusTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus != nil &&
		cfg.Spec.Network.Multus.Installation != nil &&
		cfg.Spec.Network.Multus.Installation.Enabled != nil {
		return *cfg.Spec.Network.Multus.Installation.Enabled, nil
	}
	return false, nil
}

func (t *DeployMultusTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy multus")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := multusstep.NewGenerateMultusValuesStepBuilder(runtimeCtx, "GenerateMultusValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := multusstep.NewDistributeMultusArtifactsStepBuilder(runtimeCtx, "DistributeMultusArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := multusstep.NewInstallMultusHelmChartStepBuilder(runtimeCtx, "InstallMultusChart").Build()
	if err != nil {
		return nil, err
	}

	if generateValues == nil || distributeArtifacts == nil || installChart == nil {
		ctx.GetLogger().Info("Skipping Multus deployment task because one or more steps could not be constructed (check BOM and config).")
		return fragment, nil
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateMultusValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeMultusArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallMultusChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateMultusValues", "DistributeMultusArtifacts")
	fragment.AddDependency("DistributeMultusArtifacts", "InstallMultusChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
