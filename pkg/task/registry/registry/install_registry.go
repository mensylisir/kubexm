package registry

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	registrystep "github.com/mensylisir/kubexm/pkg/step/registry"
	"github.com/mensylisir/kubexm/pkg/task"
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

	generateConfig := registrystep.NewGenerateRegistryConfigStepBuilder(*runtimeCtx, "GenerateRegistryConfig").Build()
	downloadBinary := registrystep.NewDownloadRegistryStepBuilder(*runtimeCtx, "DownloadRegistryBinary").Build()
	extractBinary := registrystep.NewExtractRegistryStepBuilder(*runtimeCtx, "ExtractRegistryBinary").Build()

	distributeConfig := registrystep.NewDistributeRegistryConfigStepBuilder(*runtimeCtx, "DistributeRegistryConfig").Build()
	installBinary := registrystep.NewInstallRegistryStepBuilder(*runtimeCtx, "InstallRegistryBinary").Build()
	setupService := registrystep.NewSetupRegistryServiceStepBuilder(*runtimeCtx, "SetupRegistryService").Build()
	enableService := registrystep.NewEnableRegistryServiceStepBuilder(*runtimeCtx, "EnableRegistryService").Build()
	restartService := registrystep.NewRestartRegistryServiceStepBuilder(*runtimeCtx, "RestartRegistryService").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateRegistryConfig", Step: generateConfig, Hosts: []connector.Host{controlNode}})

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode, start download registry binary")
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadRegistryBinary", Step: downloadBinary, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadRegistryBinary", "ExtractRegistryBinary")
	} else {
		ctx.GetLogger().Info("Offline mode, skip download registry binary")
	}

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
