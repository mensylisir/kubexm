package multus

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	multusstep "github.com/mensylisir/kubexm/pkg/step/network/multus"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy multus")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := multusstep.NewGenerateMultusValuesStepBuilder(*runtimeCtx, "GenerateMultusValues").Build()
	distributeArtifacts := multusstep.NewDistributeMultusArtifactsStepBuilder(*runtimeCtx, "DistributeMultusArtifacts").Build()
	installChart := multusstep.NewInstallMultusHelmChartStepBuilder(*runtimeCtx, "InstallMultusChart").Build()

	if generateValues == nil || distributeArtifacts == nil || installChart == nil {
		ctx.GetLogger().Info("Skipping Multus deployment task because one or more steps could not be constructed (check BOM and config).")
		return fragment, nil
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateMultusValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeMultusArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallMultusChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateMultusValues", "DistributeMultusArtifacts")
	fragment.AddDependency("DistributeMultusArtifacts", "InstallMultusChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading Multus chart.")
		downloadChart := multusstep.NewDownloadMultusChartStepBuilder(*runtimeCtx, "DownloadMultusChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadMultusChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadMultusChart", "GenerateMultusValues")

	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download for Multus artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
