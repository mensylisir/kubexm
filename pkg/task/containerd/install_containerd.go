package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployContainerdTask struct {
	task.Base
}

func NewDeployContainerdTask() task.Task {
	return &DeployContainerdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployContainerd",
				Description: "Download, extract, install, and configure containerd and its dependencies",
			},
		},
	}
}

func (t *DeployContainerdTask) Name() string {
	return t.Meta.Name
}

func (t *DeployContainerdTask) Description() string {
	return t.Meta.Description
}

func (t *DeployContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type == common.RuntimeTypeContainerd, nil
}

func (t *DeployContainerdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to deploy containerd")
	}

	extractContainerd := containerd.NewExtractContainerdStepBuilder(*runtimeCtx, "ExtractContainerd").Build()
	extractCNI := containerd.NewExtractCNIPluginsStepBuilder(*runtimeCtx, "ExtractCNI").Build()
	installRunc := containerd.NewInstallRuncStepBuilder(*runtimeCtx, "InstallRunc").Build()
	installCNI := containerd.NewInstallCNIPluginsStepBuilder(*runtimeCtx, "InstallCNI").Build()
	installContainerd := containerd.NewInstallContainerdStepBuilder(*runtimeCtx, "InstallContainerd").Build()
	configureContainerd := containerd.NewConfigureContainerdStepBuilder(*runtimeCtx, "ConfigureContainerd").Build()
	installService := containerd.NewInstallContainerdServiceStepBuilder(*runtimeCtx, "InstallContainerdService").Build()
	startContainerd := containerd.NewStartContainerdStepBuilder(*runtimeCtx, "StartContainerd").Build() // 假设存在

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractContainerd", Step: extractContainerd, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCNI", Step: extractCNI, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallRunc", Step: installRunc, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCNI", Step: installCNI, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallContainerd", Step: installContainerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureContainerd", Step: configureContainerd, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallContainerdService", Step: installService, Hosts: deployHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartContainerd", Step: startContainerd, Hosts: deployHosts})

	fragment.AddDependency("ExtractCNI", "InstallCNI")
	fragment.AddDependency("ExtractContainerd", "InstallContainerd")
	fragment.AddDependency("InstallRunc", "ConfigureContainerd")
	fragment.AddDependency("InstallCNI", "ConfigureContainerd")
	fragment.AddDependency("InstallContainerd", "ConfigureContainerd")
	fragment.AddDependency("ConfigureContainerd", "InstallContainerdService")
	fragment.AddDependency("InstallContainerdService", "StartContainerd")

	fragment.CalculateEntryAndExitNodes()

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download steps for containerd dependencies.")

		downloadContainerd := containerd.NewDownloadContainerdStepBuilder(*runtimeCtx, "DownloadContainerd").Build()
		downloadRunc := containerd.NewDownloadRuncStepBuilder(*runtimeCtx, "DownloadRunc").Build()
		downloadCNI := containerd.NewDownloadCNIPluginsStepBuilder(*runtimeCtx, "DownloadCNI").Build()

		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadContainerd", Step: downloadContainerd, Hosts: []connector.Host{controlNode}})
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadRunc", Step: downloadRunc, Hosts: []connector.Host{controlNode}})
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCNI", Step: downloadCNI, Hosts: []connector.Host{controlNode}})

		fragment.AddDependency("DownloadContainerd", "ExtractContainerd")
		fragment.AddDependency("DownloadCNI", "ExtractCNI")
		fragment.AddDependency("DownloadRunc", "InstallRunc")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download steps.")
	}

	return fragment, nil
}
