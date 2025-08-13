package harbor

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/harbor"
	"github.com/mensylisir/kubexm/pkg/task"
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return fragment, nil
	}
	executionHost := registryHosts[0]
	allClusterHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)

	stopAndRemove := harbor.NewStopAndRemoveHarborStepBuilder(*runtimeCtx, "StopAndRemoveHarbor").Build()
	removeArtifacts := harbor.NewRemoveHarborArtifactsStepBuilder(*runtimeCtx, "RemoveHarborArtifacts").Build()
	removeCACerts := harbor.NewRemoveHarborCACertStepBuilder(*runtimeCtx, "RemoveHarborCACerts").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopAndRemoveHarbor", Step: stopAndRemove, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHarborArtifacts", Step: removeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHarborCACerts", Step: removeCACerts, Hosts: allClusterHosts})

	fragment.AddDependency("StopAndRemoveHarbor", "RemoveHarborArtifacts")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
