package flannel

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/flannel"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanFlannelTask struct {
	task.Base
}

func NewCleanFlannelTask() task.Task {
	return &CleanFlannelTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanFlannel",
				Description: "Uninstall Flannel CNI network addon and cleanup related resources",
			},
		},
	}
}

func (t *CleanFlannelTask) Name() string {
	return t.Meta.Name
}

func (t *CleanFlannelTask) Description() string {
	return t.Meta.Description
}

func (t *CleanFlannelTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Network == nil {
		return false, nil
	}
	netSpec := ctx.GetClusterConfig().Spec.Network
	if netSpec == nil {
		return false, nil
	}
	return netSpec.Plugin == string(common.CNITypeFlannel), nil
}

func (t *CleanFlannelTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]
	allHosts := append(masterHosts, ctx.GetHostsByRole(common.RoleWorker)...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	uninstallChartStep, err := flannel.NewUninstallFlannelHelmChartStepBuilder(runtimeCtx, "UninstallFlannelChart").Build()
	if err != nil {
		return nil, err
	}
	cleanFilesStep, err := flannel.NewCleanFlannelNodeFilesStepBuilder(runtimeCtx, "CleanFlannelNodeFiles").Build()
	if err != nil {
		return nil, err
	}

	nodeUninstallChart := &plan.ExecutionNode{Name: "UninstallFlannelChart", Step: uninstallChartStep, Hosts: []remotefw.Host{executionHost}}
	fragment.AddNode(nodeUninstallChart)

	nodeCleanFiles := &plan.ExecutionNode{Name: "CleanFlannelNodeFiles", Step: cleanFilesStep, Hosts: allHosts}
	fragment.AddNode(nodeCleanFiles)

	fragment.AddDependency("UninstallFlannelChart", "CleanFlannelNodeFiles")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
