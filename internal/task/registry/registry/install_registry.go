package registry

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	registrystep "github.com/mensylisir/kubexm/internal/step/registry"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployRegistryTask struct {
	task.Base
}

func NewDeployRegistryTask() task.Task {
	return &DeployRegistryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployRegistry",
				Description: "Deploy a local Docker Registry service on registry nodes",
			},
		},
	}
}

func (t *DeployRegistryTask) Name() string {
	return t.Meta.Name
}

func (t *DeployRegistryTask) Description() string {
	return t.Meta.Description
}

func (t *DeployRegistryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Registry.LocalDeployment == nil || cfg.Spec.Registry.LocalDeployment.Type != "registry" {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleRegistry)) > 0, nil
}

func (t *DeployRegistryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return fragment, nil
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateConfig, err := registrystep.NewGenerateRegistryConfigStepBuilder(runtimeCtx, "GenerateRegistryConfig").Build()
	if err != nil {
		return nil, err
	}
	extractBinary, err := registrystep.NewExtractRegistryStepBuilder(runtimeCtx, "ExtractRegistryBinary").Build()
	if err != nil {
		return nil, err
	}

	distributeConfig, err := registrystep.NewDistributeRegistryConfigStepBuilder(runtimeCtx, "DistributeRegistryConfig").Build()
	if err != nil {
		return nil, err
	}
	installBinary, err := registrystep.NewInstallRegistryStepBuilder(runtimeCtx, "InstallRegistryBinary").Build()
	if err != nil {
		return nil, err
	}
	setupService, err := registrystep.NewSetupRegistryServiceStepBuilder(runtimeCtx, "SetupRegistryService").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := registrystep.NewEnableRegistryServiceStepBuilder(runtimeCtx, "EnableRegistryService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := registrystep.NewRestartRegistryServiceStepBuilder(runtimeCtx, "RestartRegistryService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateRegistryConfig", Step: generateConfig, Hosts: []connector.Host{controlNode}})
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractRegistryBinary", Step: extractBinary, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeRegistryConfig", Step: distributeConfig, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallRegistryBinary", Step: installBinary, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "SetupRegistryService", Step: setupService, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableRegistryService", Step: enableService, Hosts: registryHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartRegistryService", Step: restartService, Hosts: registryHosts})

	fragment.AddDependency("ExtractRegistryBinary", "InstallRegistryBinary")
	fragment.AddDependency("GenerateRegistryConfig", "DistributeRegistryConfig")
	fragment.AddDependency("DistributeRegistryConfig", "SetupRegistryService")
	fragment.AddDependency("InstallRegistryBinary", "SetupRegistryService")
	fragment.AddDependency("SetupRegistryService", "EnableRegistryService")
	fragment.AddDependency("EnableRegistryService", "RestartRegistryService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
