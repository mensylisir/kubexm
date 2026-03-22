package cilium

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/cilium"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanCiliumTask struct {
	task.Base
}

func NewCleanCiliumTask() task.Task {
	return &CleanCiliumTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanCilium",
				Description: "Uninstall Cilium CNI network addon and cleanup related resources",
			},
		},
	}
}

func (t *CleanCiliumTask) Name() string {
	return t.Meta.Name
}

func (t *CleanCiliumTask) Description() string {
	return t.Meta.Description
}

func (t *CleanCiliumTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Network == nil {
		return false, nil
	}
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCilium), nil
}

func (t *CleanCiliumTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]
	allHosts := append(masterHosts, ctx.GetHostsByRole(common.RoleWorker)...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	uninstallChartStep, err := cilium.NewUninstallCiliumHelmChartStepBuilder(runtimeCtx, "UninstallCiliumChartAndCRDs").Build()
	if err != nil {
		return nil, err
	}
	cleanStateStep, err := cilium.NewCleanCiliumNodeStateStepBuilder(runtimeCtx, "CleanCiliumNodeState").Build()
	if err != nil {
		return nil, err
	}

	nodeUninstallChart := &plan.ExecutionNode{Name: "UninstallCiliumChartAndCRDs", Step: uninstallChartStep, Hosts: []connector.Host{executionHost}}
	fragment.AddNode(nodeUninstallChart)

	nodeCleanState := &plan.ExecutionNode{Name: "CleanCiliumNodeState", Step: cleanStateStep, Hosts: allHosts}
	fragment.AddNode(nodeCleanState)

	fragment.AddDependency("UninstallCiliumChartAndCRDs", "CleanCiliumNodeState")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
