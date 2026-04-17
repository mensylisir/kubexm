package calico

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
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
	netSpec := ctx.GetClusterConfig().Spec.Network
	if netSpec == nil {
		return false, nil
	}
	return netSpec.Plugin == string(common.CNITypeCalico), nil
}

func (t *DeployCalicoTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	_, err := ctx.GetControlNode()
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

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateCalicoValues", Step: generateConfig, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeCalico", Step: distributeCalico, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalico", Step: installCalico, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalicoctl", Step: installCalicoctl, Hosts: masterHosts})

	fragment.AddDependency("GenerateCalicoValues", "DistributeCalico")
	fragment.AddDependency("DistributeCalico", "InstallCalico")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
