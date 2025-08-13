package flannel

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/flannel"
	"github.com/mensylisir/kubexm/pkg/task"
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
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeFlannel), nil
}

func (t *CleanFlannelTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]
	allHosts := append(masterHosts, ctx.GetHostsByRole(common.RoleWorker)...)
	if len(allHosts) == 0 {
		return fragment, nil
	}

	uninstallChartStep := flannel.NewUninstallFlannelHelmChartStepBuilder(*runtimeCtx, "UninstallFlannelChart").Build()
	cleanFilesStep := flannel.NewCleanFlannelNodeFilesStepBuilder(*runtimeCtx, "CleanFlannelNodeFiles").Build()

	nodeUninstallChart := &plan.ExecutionNode{Name: "UninstallFlannelChart", Step: uninstallChartStep, Hosts: []connector.Host{executionHost}}
	fragment.AddNode(nodeUninstallChart)

	nodeCleanFiles := &plan.ExecutionNode{Name: "CleanFlannelNodeFiles", Step: cleanFilesStep, Hosts: allHosts}
	fragment.AddNode(nodeCleanFiles)

	fragment.AddDependency("UninstallFlannelChart", "CleanFlannelNodeFiles")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
