package kubexm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	certsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/certs"
	proxystep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubeletstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployWorkerNodesTask struct {
	task.Base
}

func NewDeployWorkerNodesTask() task.Task {
	return &DeployWorkerNodesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployWorkerNodes",
				Description: "Deploy kubelet and kube-proxy on all worker nodes",
			},
		},
	}
}

func (t *DeployWorkerNodesTask) Name() string {
	return t.Meta.Name
}

func (t *DeployWorkerNodesTask) Description() string {
	return t.Meta.Description
}

func (t *DeployWorkerNodesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return len(ctx.GetHostsByRole(common.RoleWorker)) > 0, nil
}

// Plan 创建一个执行计划来部署所有 Worker 节点。
func (t *DeployWorkerNodesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	distributeCACerts, err := certsstep.NewDistributeKubeCACertsStepBuilder(runtimeCtx, "DistributeCACertsToWorkers").Build()
	if err != nil {
		return nil, err
	}
	distributeKubeletConfig, err := kubeletstep.NewDistributeKubeletConfigForAllNodesStepBuilder(runtimeCtx, "DistributeKubeletCredentials").Build()
	if err != nil {
		return nil, err
	}
	installKubelet, err := kubeletstep.NewInstallKubeletStepBuilder(runtimeCtx, "InstallKubeletBinary").Build()
	if err != nil {
		return nil, err
	}
	installKubeProxy, err := proxystep.NewInstallKubeProxyStepBuilder(runtimeCtx, "InstallKubeProxyBinary").Build()
	if err != nil {
		return nil, err
	}

	createKubeletConfig, err := kubeletstep.NewCreateKubeletConfigYAMLStepBuilder(runtimeCtx, "CreateKubeletConfigYAMLForWorkers").Build()
	if err != nil {
		return nil, err
	}
	createKubeProxyConfig, err := proxystep.NewCreateKubeProxyConfigYAMLStepBuilder(runtimeCtx, "CreateKubeProxyConfigYAMLForWorkers").Build()
	if err != nil {
		return nil, err
	}

	installKubeletService, err := kubeletstep.NewInstallKubeletServiceStepBuilder(runtimeCtx, "InstallKubeletService").Build()
	if err != nil {
		return nil, err
	}
	installKubeletDropIn, err := kubeletstep.NewInstallKubeletDropInStepBuilder(runtimeCtx, "InstallKubeletDropIn").Build()
	if err != nil {
		return nil, err
	}
	enableKubelet, err := kubeletstep.NewEnableKubeletStepBuilder(runtimeCtx, "EnableKubeletService").Build()
	if err != nil {
		return nil, err
	}
	startKubelet, err := kubeletstep.NewStartKubeletStepBuilder(runtimeCtx, "StartKubeletService").Build()
	if err != nil {
		return nil, err
	}
	installProxyService, err := proxystep.NewInstallKubeProxyServiceStepBuilder(runtimeCtx, "InstallKubeProxyService").Build()
	if err != nil {
		return nil, err
	}
	enableProxy, err := proxystep.NewEnableKubeProxyStepBuilder(runtimeCtx, "EnableKubeProxyService").Build()
	if err != nil {
		return nil, err
	}
	startProxy, err := proxystep.NewStartKubeProxyStepBuilder(runtimeCtx, "StartKubeProxyService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeCACertsToWorkers", Step: distributeCACerts, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeKubeletCredentials", Step: distributeKubeletConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletBinary", Step: installKubelet, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeProxyBinary", Step: installKubeProxy, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeletConfigYAMLForWorkers", Step: createKubeletConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeProxyConfigYAMLForWorkers", Step: createKubeProxyConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletService", Step: installKubeletService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeletDropIn", Step: installKubeletDropIn, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeletService", Step: enableKubelet, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeletService", Step: startKubelet, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeProxyService", Step: installProxyService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeProxyService", Step: enableProxy, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeProxyService", Step: startProxy, Hosts: workerHosts})

	fragment.AddDependency("InstallKubeletBinary", "InstallKubeletService")
	fragment.AddDependency("DistributeCACertsToWorkers", "CreateKubeletConfigYAMLForWorkers")
	fragment.AddDependency("DistributeKubeletCredentials", "InstallKubeletDropIn")
	fragment.AddDependency("CreateKubeletConfigYAMLForWorkers", "InstallKubeletDropIn")
	fragment.AddDependency("InstallKubeletService", "InstallKubeletDropIn")
	fragment.AddDependency("InstallKubeletDropIn", "EnableKubeletService")
	fragment.AddDependency("EnableKubeletService", "StartKubeletService")

	fragment.AddDependency("InstallKubeProxyBinary", "InstallKubeProxyService")
	fragment.AddDependency("CreateKubeProxyConfigYAMLForWorkers", "InstallKubeProxyService")
	fragment.AddDependency("InstallKubeProxyService", "EnableKubeProxyService")
	fragment.AddDependency("EnableKubeProxyService", "StartKubeProxyService")

	fragment.AddDependency("StartKubeletService", "StartKubeProxyService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
