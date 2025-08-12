package crio

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/crio"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployCrioTask struct {
	task.Base
}

func NewDeployCrioTask() task.Task {
	return &DeployCrioTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCrio",
				Description: "Download, extract, install, and configure CRI-O and its dependencies",
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to deploy CRI-O")
	}

	extractCrio := crio.NewExtractCrioStepBuilder(*runtimeCtx, "ExtractCrio").Build()
	installCrio := crio.NewInstallCrioStepBuilder(*runtimeCtx, "InstallCrio").Build()
	installCni := crio.NewInstallCniStepBuilder(*runtimeCtx, "InstallCni").Build()
	installCrictl := crio.NewInstallCrictlStepBuilder(*runtimeCtx, "InstallCrictl").Build()
	installPolicyJson := crio.NewInstallPolicyJsonStepBuilder(*runtimeCtx, "InstallPolicyJson").Build()
	installCrioEnv := crio.NewInstallCrioEnvStepBuilder(*runtimeCtx, "InstallCrioEnv").Build()
	configureCrio := crio.NewConfigureCrioStepBuilder(*runtimeCtx, "ConfigureCrio").Build()
	configureRegistries := crio.NewConfigureRegistriesStepBuilder(*runtimeCtx, "ConfigureRegistries").Build()
	configureAuth := crio.NewConfigureAuthStepBuilder(*runtimeCtx, "ConfigureAuth").Build()
	installService := crio.NewInstallCrioServiceStepBuilder(*runtimeCtx, "InstallCrioService").Build()
	enableCrio := crio.NewEnableCrioStepBuilder(*runtimeCtx, "EnableCrio").Build()
	startCrio := crio.NewStartCrioStepBuilder(*runtimeCtx, "StartCrio").Build()

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

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download steps for containerd dependencies.")
		downloadCrio := crio.NewDownloadCrioStepBuilder(*runtimeCtx, "DownloadCrio").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCrio", Step: downloadCrio, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadCrio", "ExtractCrio")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download steps.")
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
