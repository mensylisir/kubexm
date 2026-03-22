package calico

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/calico"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployCalicoTask struct {
	task.Base
}

func NewDeployCalicoTask() task.Task {
	return &DeployCalicoTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCalico",
				Description: "Deploy Calico CNI network addon",
			},
		},
	}
}

func (t *DeployCalicoTask) Name() string {
	return t.Meta.Name
}

func (t *DeployCalicoTask) Description() string {
	return t.Meta.Description
}

func (t *DeployCalicoTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCalico), nil
}

func (t *DeployCalicoTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy calico")
	}
	executionHost := masterHosts[0]

	generateConfig, err := calico.NewGenerateCalicoValuesStepBuilder(runtimeCtx, "GenerateCalicoValues").Build()
	if err != nil {
		return nil, err
	}
	distributeCalico, err := calico.NewDistributeCalicoArtifactsStepBuilder(runtimeCtx, "DistributeCalico").Build()
	if err != nil {
		return nil, err
	}
	installCalico, err := calico.NewInstallCalicoHelmChartStepBuilder(runtimeCtx, "InstallCalico").Build()
	if err != nil {
		return nil, err
	}
	installCalicoctl, err := calico.NewInstallCalicoctlStepBuilder(runtimeCtx, "InstallCalicoctl").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateCalicoValues", Step: generateConfig, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeCalico", Step: distributeCalico, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalico", Step: installCalico, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalicoctl", Step: installCalicoctl, Hosts: masterHosts})

	fragment.AddDependency("GenerateCalicoValues", "DistributeCalico")
	fragment.AddDependency("DistributeCalico", "InstallCalico")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
