package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/docker"
	"github.com/mensylisir/kubexm/internal/step/network/cni"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployDockerTask struct {
	task.Base
}

func NewDeployDockerTask() task.Task {
	return &DeployDockerTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployDocker",
				Description: "Install Docker and cri-dockerd as the container runtime",
			},
		},
	}
}

func (t *DeployDockerTask) Name() string {
	return t.Meta.Name
}

func (t *DeployDockerTask) Description() string {
	return t.Meta.Description
}

func (t *DeployDockerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type == common.RuntimeTypeDocker, nil
}

func (t *DeployDockerTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to deploy Docker")
	}

	extractDocker, err := docker.NewExtractDockerStepBuilder(runtimeCtx, "ExtractDocker").Build()
	if err != nil {
		return nil, err
	}
	extractCriDockerd, err := docker.NewExtractCriDockerdStepBuilder(runtimeCtx, "ExtractCriDockerd").Build()
	if err != nil {
		return nil, err
	}
	installDocker, err := docker.NewInstallDockerStepBuilder(runtimeCtx, "InstallDocker").Build()
	if err != nil {
		return nil, err
	}
	configureDocker, err := docker.NewConfigureDockerStepBuilder(runtimeCtx, "ConfigureDocker").Build()
	if err != nil {
		return nil, err
	}
	installDockerSvc, err := docker.NewSetupDockerServiceStepBuilder(runtimeCtx, "InstallDockerService").Build()
	if err != nil {
		return nil, err
	}
	startDocker, err := docker.NewStartDockerStepBuilder(runtimeCtx, "StartDocker").Build()
	if err != nil {
		return nil, err
	}
	installCriDockerd, err := docker.NewInstallCriDockerdStepBuilder(runtimeCtx, "InstallCriDockerd").Build()
	if err != nil {
		return nil, err
	}
	installCriDockerdSvc, err := docker.NewSetupCriDockerdServiceStepBuilder(runtimeCtx, "InstallCriDockerdService").Build()
	if err != nil {
		return nil, err
	}
	startCriDockerd, err := docker.NewStartCriDockerdStepBuilder(runtimeCtx, "StartCriDockerd").Build()
	if err != nil {
		return nil, err
	}

	// CNI binary distribution (required for Docker runtime too)
	installCni, err := cni.NewInstallCNIPluginsStepBuilder(runtimeCtx, "InstallCNIPlugins").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractDocker", Step: extractDocker, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCriDockerd", Step: extractCriDockerd, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallDocker", Step: installDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureDocker", Step: configureDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallDockerService", Step: installDockerSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartDocker", Step: startDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCriDockerd", Step: installCriDockerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCriDockerdService", Step: installCriDockerdSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartCriDockerd", Step: startCriDockerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCNIPlugins", Step: installCni, Hosts: deployHosts})

	fragment.AddDependency("ExtractDocker", "InstallDocker")
	fragment.AddDependency("ExtractCriDockerd", "InstallCriDockerd")
	fragment.AddDependency("InstallDocker", "ConfigureDocker")
	fragment.AddDependency("ConfigureDocker", "InstallDockerService")
	fragment.AddDependency("InstallDockerService", "StartDocker")
	fragment.AddDependency("InstallCriDockerd", "InstallCriDockerdService")
	fragment.AddDependency("StartDocker", "InstallCriDockerdService")
	fragment.AddDependency("InstallCriDockerdService", "StartCriDockerd")
	fragment.AddDependency("StartCriDockerd", "InstallCNIPlugins")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
