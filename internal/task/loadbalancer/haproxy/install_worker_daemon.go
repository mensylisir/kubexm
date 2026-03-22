package haproxy

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployHAProxyOnWorkersTask struct {
	task.Base
}

func NewDeployHAProxyOnWorkersTask() task.Task {
	return &DeployHAProxyOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyOnWorkers",
				Description: "Deploy HAProxy as a system service on worker nodes for internal load balancing",
			},
		},
	}
}

func (t *DeployHAProxyOnWorkersTask) Name() string {
	return t.Meta.Name
}

func (t *DeployHAProxyOnWorkersTask) Description() string {
	return t.Meta.Description
}

func (t *DeployHAProxyOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil {
		return false, nil
	}
	ha := cfg.Spec.ControlPlaneEndpoint.HighAvailability
	if ha.Enabled == nil || !*ha.Enabled {
		return false, nil
	}
	if ha.Internal == nil || ha.Internal.Enabled == nil || !*ha.Internal.Enabled {
		return false, nil
	}
	if ha.Internal.Type != string(common.InternalLBTypeHAProxy) {
		return false, nil
	}
	if cfg.Spec.Kubernetes == nil || cfg.Spec.Kubernetes.Type != string(common.KubernetesDeploymentTypeKubexm) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(common.RoleMaster)) <= 1 {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleWorker)) > 0, nil
}

func (t *DeployHAProxyOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := haproxystep.NewGenerateHAProxyConfigStepBuilder(runtimeCtx, "GenerateHAProxyWorkerConfig").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := haproxystep.NewEnableHAProxyStepBuilder(runtimeCtx, "EnableHAProxyWorkerService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := haproxystep.NewRestartHAProxyStepBuilder(runtimeCtx, "RestartHAProxyWorkerService").Build()
	if err != nil {
		return nil, err
	}
	installPackage, err := haproxystep.NewInstallHAProxyStepBuilder(runtimeCtx, "InstallHAProxyWorkerPackage").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyWorkerConfig", Step: generateConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyWorkerService", Step: enableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartHAProxyWorkerService", Step: restartService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHAProxyWorkerPackage", Step: installPackage, Hosts: workerHosts})

	fragment.AddDependency("InstallHAProxyWorkerPackage", "GenerateHAProxyWorkerConfig")
	fragment.AddDependency("GenerateHAProxyWorkerConfig", "EnableHAProxyWorkerService")
	fragment.AddDependency("EnableHAProxyWorkerService", "RestartHAProxyWorkerService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployHAProxyOnWorkersTask)(nil)
