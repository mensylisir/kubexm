package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/docker" // 确保引入了 docker 的 step 包
	"github.com/mensylisir/kubexm/pkg/task"
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
		return nil, fmt.Errorf("no master or worker hosts found to deploy Docker")
	}

	extractDocker := docker.NewExtractDockerStepBuilder(*runtimeCtx, "ExtractDocker").Build()
	extractCriDockerd := docker.NewExtractCriDockerdStepBuilder(*runtimeCtx, "ExtractCriDockerd").Build()
	installDocker := docker.NewInstallDockerStepBuilder(*runtimeCtx, "InstallDocker").Build()
	configureDocker := docker.NewConfigureDockerStepBuilder(*runtimeCtx, "ConfigureDocker").Build()
	installDockerSvc := docker.NewSetupDockerServiceStepBuilder(*runtimeCtx, "InstallDockerService").Build()
	startDocker := docker.NewStartDockerStepBuilder(*runtimeCtx, "StartDocker").Build()
	installCriDockerd := docker.NewInstallCriDockerdStepBuilder(*runtimeCtx, "InstallCriDockerd").Build()
	installCriDockerdSvc := docker.NewSetupCriDockerdServiceStepBuilder(*runtimeCtx, "InstallCriDockerdService").Build()
	startCriDockerd := docker.NewStartCriDockerdStepBuilder(*runtimeCtx, "StartCriDockerd").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractDocker", Step: extractDocker, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCriDockerd", Step: extractCriDockerd, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallDocker", Step: installDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureDocker", Step: configureDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallDockerService", Step: installDockerSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartDocker", Step: startDocker, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCriDockerd", Step: installCriDockerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCriDockerdService", Step: installCriDockerdSvc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartCriDockerd", Step: startCriDockerd, Hosts: deployHosts})

	fragment.AddDependency("ExtractDocker", "InstallDocker")
	fragment.AddDependency("ExtractCriDockerd", "InstallCriDockerd")
	fragment.AddDependency("InstallDocker", "ConfigureDocker")
	fragment.AddDependency("ConfigureDocker", "InstallDockerService")
	fragment.AddDependency("InstallDockerService", "StartDocker")
	fragment.AddDependency("InstallCriDockerd", "InstallCriDockerdService")
	fragment.AddDependency("StartDocker", "InstallCriDockerdService")
	fragment.AddDependency("InstallCriDockerdService", "StartCriDockerd")

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download steps for containerd dependencies.")
		downloadDocker := docker.NewDownloadDockerStepBuilder(*runtimeCtx, "DownloadDocker").Build()
		downloadCriDockerd := docker.NewDownloadCriDockerdStepBuilder(*runtimeCtx, "DownloadCriDockerd").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadDocker", Step: downloadDocker, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadDocker", "ExtractDocker")
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCriDockerd", Step: downloadCriDockerd, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadCriDockerd", "ExtractCriDockerd")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download steps.")
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
