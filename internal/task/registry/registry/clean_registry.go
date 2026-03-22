package registry

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	registrystep "github.com/mensylisir/kubexm/internal/step/registry"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanRegistryTask struct {
	task.Base
}

func NewCleanRegistryTask() task.Task {
	return &CleanRegistryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanRegistry",
				Description: "Clean up local Docker Registry service and related resources",
			},
		},
	}
}

func (t *CleanRegistryTask) Name() string {
	return t.Meta.Name
}

func (t *CleanRegistryTask) Description() string {
	return t.Meta.Description
}

func (t *CleanRegistryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Registry.LocalDeployment == nil || cfg.Spec.Registry.LocalDeployment.Type != "registry" {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleRegistry)) > 0, nil
}

func (t *CleanRegistryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return fragment, nil
	}

	stopService, err := registrystep.NewStopRegistryServiceStepBuilder(runtimeCtx, "StopRegistryService").Build()
	if err != nil {
		return nil, err
	}
	disableService, err := registrystep.NewDisableRegistryServiceStepBuilder(runtimeCtx, "DisableRegistryService").Build()
	if err != nil {
		return nil, err
	}
	removeArtifacts, err := registrystep.NewRemoveRegistryArtifactsStepBuilder(runtimeCtx, "RemoveRegistryArtifacts").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopRegistryService", Step: stopService, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableRegistryService", Step: disableService, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveRegistryArtifacts", Step: removeArtifacts, Hosts: registryHosts})

	fragment.AddDependency("StopRegistryService", "DisableRegistryService")
	fragment.AddDependency("DisableRegistryService", "RemoveRegistryArtifacts")

	if cfg := ctx.GetClusterConfig().Spec.Registry.LocalDeployment; cfg != nil && cfg.DeleteDataOnClean {
		ctx.GetLogger().Warn("Registry data will be deleted as 'deleteDataOnClean' is true.")
		removeData, err := registrystep.NewRemoveRegistryDataStepBuilder(runtimeCtx, "RemoveRegistryData").Build()
		if err != nil {
			return nil, err
		}
		fragment.AddNode(&plan.ExecutionNode{Name: "RemoveRegistryData", Step: removeData, Hosts: registryHosts})
		fragment.AddDependency("StopRegistryService", "RemoveRegistryData")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
