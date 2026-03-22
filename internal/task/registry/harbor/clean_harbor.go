package harbor

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/harbor"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanHarborTask struct {
	task.Base
}

func NewCleanHarborTask() task.Task {
	return &CleanHarborTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanHarbor",
				Description: "Uninstall Harbor and cleanup all related resources and data",
			},
		},
	}
}

func (t *CleanHarborTask) Name() string {
	return t.Meta.Name
}

func (t *CleanHarborTask) Description() string {
	return t.Meta.Description
}

func (t *CleanHarborTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Registry == nil || ctx.GetClusterConfig().Spec.Registry.LocalDeployment == nil {
		return false, nil
	}
	return ctx.GetClusterConfig().Spec.Registry.LocalDeployment.Type == common.RegistryTypeHarbor, nil
}

func (t *CleanHarborTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return fragment, nil
	}
	executionHost := registryHosts[0]
	allClusterHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)

	stopAndRemove, err := harbor.NewStopAndRemoveHarborStepBuilder(runtimeCtx, "StopAndRemoveHarbor").Build()
	if err != nil {
		return nil, err
	}
	removeArtifacts, err := harbor.NewRemoveHarborArtifactsStepBuilder(runtimeCtx, "RemoveHarborArtifacts").Build()
	if err != nil {
		return nil, err
	}
	removeCACerts, err := harbor.NewRemoveHarborCACertStepBuilder(runtimeCtx, "RemoveHarborCACerts").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopAndRemoveHarbor", Step: stopAndRemove, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHarborArtifacts", Step: removeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHarborCACerts", Step: removeCACerts, Hosts: allClusterHosts})

	fragment.AddDependency("StopAndRemoveHarbor", "RemoveHarborArtifacts")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
