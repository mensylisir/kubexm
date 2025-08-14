package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	certsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/certs"
	proxystep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	kubeletstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	distributeCACerts := certsstep.NewDistributeKubeCACertsStepBuilder(*runtimeCtx, "DistributeCACertsToWorkers").Build()
	distributeKubeletConfig := kubeletstep.NewDistributeKubeletConfigForAllNodesStepBuilder(*runtimeCtx, "DistributeKubeletCredentials").Build()
	installKubelet := kubeletstep.NewInstallKubeletStepBuilder(*runtimeCtx, "InstallKubeletBinary").Build()
	installKubeProxy := proxystep.NewInstallKubeProxyStepBuilder(*runtimeCtx, "InstallKubeProxyBinary").Build()

	createKubeletConfig := kubeletstep.NewCreateKubeletConfigYAMLStepBuilder(*runtimeCtx, "CreateKubeletConfigYAMLForWorkers").Build()
	createKubeProxyConfig := proxystep.NewCreateKubeProxyConfigYAMLStepBuilder(*runtimeCtx, "CreateKubeProxyConfigYAMLForWorkers").Build()

	installKubeletService := kubeletstep.NewInstallKubeletServiceStepBuilder(*runtimeCtx, "InstallKubeletService").Build()
	installKubeletDropIn := kubeletstep.NewInstallKubeletDropInStepBuilder(*runtimeCtx, "InstallKubeletDropIn").Build()
	enableKubelet := kubeletstep.NewEnableKubeletStepBuilder(*runtimeCtx, "EnableKubeletService").Build()
	startKubelet := kubeletstep.NewStartKubeletStepBuilder(*runtimeCtx, "StartKubeletService").Build()
	installProxyService := proxystep.NewInstallKubeProxyServiceStepBuilder(*runtimeCtx, "InstallKubeProxyService").Build()
	enableProxy := proxystep.NewEnableKubeProxyStepBuilder(*runtimeCtx, "EnableKubeProxyService").Build()
	startProxy := proxystep.NewStartKubeProxyStepBuilder(*runtimeCtx, "StartKubeProxyService").Build()

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
