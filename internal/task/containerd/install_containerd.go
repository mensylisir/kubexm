package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/containerd"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployContainerdTask struct {
	task.Base
}

func NewDeployContainerdTask() task.Task {
	return &DeployContainerdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployContainerd",
				Description: "Extract, install, and configure containerd and its dependencies (assets prepared in Preflight)",
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

	extractContainerd, err := containerd.NewExtractContainerdStepBuilder(runtimeCtx, "ExtractContainerd").Build()
	if err != nil {
		return nil, err
	}
	extractCNI, err := containerd.NewExtractCNIPluginsStepBuilder(runtimeCtx, "ExtractCNI").Build()
	if err != nil {
		return nil, err
	}
	installRunc, err := containerd.NewInstallRuncStepBuilder(runtimeCtx, "InstallRunc").Build()
	if err != nil {
		return nil, err
	}
	installCNI, err := containerd.NewInstallCNIPluginsStepBuilder(runtimeCtx, "InstallCNI").Build()
	if err != nil {
		return nil, err
	}
	installContainerd, err := containerd.NewInstallContainerdStepBuilder(runtimeCtx, "InstallContainerd").Build()
	if err != nil {
		return nil, err
	}
	configureContainerd, err := containerd.NewConfigureContainerdStepBuilder(runtimeCtx, "ConfigureContainerd").Build()
	if err != nil {
		return nil, err
	}
	installService, err := containerd.NewInstallContainerdServiceStepBuilder(runtimeCtx, "InstallContainerdService").Build()
	if err != nil {
		return nil, err
	}
	startContainerd, err := containerd.NewStartContainerdStepBuilder(runtimeCtx, "StartContainerd").Build()
	if err != nil {
		return nil, err
	}

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

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
