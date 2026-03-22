package crio

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/crio"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployCrioTask struct {
	task.Base
}

func NewDeployCrioTask() task.Task {
	return &DeployCrioTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCrio",
				Description: "Extract, install, and configure CRI-O and its dependencies (assets prepared in Preflight)",
			},
		},
	}
}

func (t *DeployCrioTask) Name() string {
	return t.Meta.Name
}

func (t *DeployCrioTask) Description() string {
	return t.Meta.Description
}

func (t *DeployCrioTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type == common.RuntimeTypeCRIO, nil
}

func (t *DeployCrioTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to deploy CRI-O")
	}

	extractCrio, err := crio.NewExtractCrioStepBuilder(runtimeCtx, "ExtractCrio").Build()
	if err != nil {
		return nil, err
	}
	installCrio, err := crio.NewInstallCrioStepBuilder(runtimeCtx, "InstallCrio").Build()
	if err != nil {
		return nil, err
	}
	installCni, err := crio.NewInstallCniStepBuilder(runtimeCtx, "InstallCni").Build()
	if err != nil {
		return nil, err
	}
	installCrictl, err := crio.NewInstallCrictlStepBuilder(runtimeCtx, "InstallCrictl").Build()
	if err != nil {
		return nil, err
	}
	installPolicyJson, err := crio.NewInstallPolicyJsonStepBuilder(runtimeCtx, "InstallPolicyJson").Build()
	if err != nil {
		return nil, err
	}
	installCrioEnv, err := crio.NewInstallCrioEnvStepBuilder(runtimeCtx, "InstallCrioEnv").Build()
	if err != nil {
		return nil, err
	}
	configureCrio, err := crio.NewConfigureCrioStepBuilder(runtimeCtx, "ConfigureCrio").Build()
	if err != nil {
		return nil, err
	}
	configureRegistries, err := crio.NewConfigureRegistriesStepBuilder(runtimeCtx, "ConfigureRegistries").Build()
	if err != nil {
		return nil, err
	}
	configureAuth, err := crio.NewConfigureAuthStepBuilder(runtimeCtx, "ConfigureAuth").Build()
	if err != nil {
		return nil, err
	}
	installService, err := crio.NewInstallCrioServiceStepBuilder(runtimeCtx, "InstallCrioService").Build()
	if err != nil {
		return nil, err
	}
	enableCrio, err := crio.NewEnableCrioStepBuilder(runtimeCtx, "EnableCrio").Build()
	if err != nil {
		return nil, err
	}
	startCrio, err := crio.NewStartCrioStepBuilder(runtimeCtx, "StartCrio").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCrio", Step: extractCrio, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCrio", Step: installCrio, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCni", Step: installCni, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCrictl", Step: installCrictl, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallPolicyJson", Step: installPolicyJson, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCrioEnv", Step: installCrioEnv, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureCrio", Step: configureCrio, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureRegistries", Step: configureRegistries, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureAuth", Step: configureAuth, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCrioService", Step: installService, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableCrio", Step: enableCrio, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartCrio", Step: startCrio, Hosts: deployHosts})

	fragment.AddDependency("ExtractCrio", "InstallCrio")
	fragment.AddDependency("ExtractCrio", "InstallCni")
	fragment.AddDependency("ExtractCrio", "InstallCrictl")
	fragment.AddDependency("ExtractCrio", "InstallPolicyJson")
	fragment.AddDependency("ExtractCrio", "InstallCrioEnv")
	fragment.AddDependency("InstallCrio", "ConfigureCrio")
	fragment.AddDependency("InstallCni", "ConfigureCrio")
	fragment.AddDependency("InstallPolicyJson", "ConfigureCrio")
	fragment.AddDependency("ConfigureCrio", "InstallCrioService")
	fragment.AddDependency("ConfigureRegistries", "InstallCrioService")
	fragment.AddDependency("ConfigureAuth", "InstallCrioService")
	fragment.AddDependency("InstallCrioService", "EnableCrio")
	fragment.AddDependency("EnableCrio", "StartCrio")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
