package crio

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
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

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to deploy CRI-O")
	}

	// Build steps with nil checks to prevent panic on missing BOM entries
	extractCrio, err := crio.NewExtractCrioStepBuilder(runtimeCtx, "ExtractCrio").Build()
	if err != nil {
		return nil, err
	}
	if extractCrio == nil {
		return nil, fmt.Errorf("extract_crio step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installCrio, err := crio.NewInstallCrioStepBuilder(runtimeCtx, "InstallCrio").Build()
	if err != nil {
		return nil, err
	}
	if installCrio == nil {
		return nil, fmt.Errorf("install_crio step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installCni, err := crio.NewInstallCniStepBuilder(runtimeCtx, "InstallCni").Build()
	if err != nil {
		return nil, err
	}
	if installCni == nil {
		return nil, fmt.Errorf("install_cni step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installCrictl, err := crio.NewInstallCrictlStepBuilder(runtimeCtx, "InstallCrictl").Build()
	if err != nil {
		return nil, err
	}
	if installCrictl == nil {
		return nil, fmt.Errorf("install_crictl step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installPolicyJson, err := crio.NewInstallPolicyJsonStepBuilder(runtimeCtx, "InstallPolicyJson").Build()
	if err != nil {
		return nil, err
	}
	if installPolicyJson == nil {
		return nil, fmt.Errorf("install_policy_json step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installCrioEnv, err := crio.NewInstallCrioEnvStepBuilder(runtimeCtx, "InstallCrioEnv").Build()
	if err != nil {
		return nil, err
	}
	if installCrioEnv == nil {
		return nil, fmt.Errorf("install_crio_env step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	configureCrio, err := crio.NewConfigureCrioStepBuilder(runtimeCtx, "ConfigureCrio").Build()
	if err != nil {
		return nil, err
	}
	if configureCrio == nil {
		return nil, fmt.Errorf("configure_crio step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	configureRegistries, err := crio.NewConfigureRegistriesStepBuilder(runtimeCtx, "ConfigureRegistries").Build()
	if err != nil {
		return nil, err
	}
	if configureRegistries == nil {
		return nil, fmt.Errorf("configure_registries step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	configureAuth, err := crio.NewConfigureAuthStepBuilder(runtimeCtx, "ConfigureAuth").Build()
	if err != nil {
		return nil, err
	}
	if configureAuth == nil {
		return nil, fmt.Errorf("configure_auth step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	installService, err := crio.NewInstallCrioServiceStepBuilder(runtimeCtx, "InstallCrioService").Build()
	if err != nil {
		return nil, err
	}
	if installService == nil {
		return nil, fmt.Errorf("install_crio_service step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	enableCrio, err := crio.NewEnableCrioStepBuilder(runtimeCtx, "EnableCrio").Build()
	if err != nil {
		return nil, err
	}
	if enableCrio == nil {
		return nil, fmt.Errorf("enable_crio step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}
	startCrio, err := crio.NewStartCrioStepBuilder(runtimeCtx, "StartCrio").Build()
	if err != nil {
		return nil, err
	}
	if startCrio == nil {
		return nil, fmt.Errorf("start_crio step returned nil - ensure CRI-O assets were prepared (kubexm download)")
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCrio", Step: extractCrio, Hosts: []remotefw.Host{controlNode}})
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
