package haproxy

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployHAProxyAsDaemonTask struct {
	task.Base
}

func NewDeployHAProxyAsDaemonTask() task.Task {
	return &DeployHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyAsDaemon",
				Description: "Deploy HAProxy as a system service on load balancer nodes",
			},
		},
	}
}

func (t *DeployHAProxyAsDaemonTask) Name() string {
	return t.Meta.Name
}

func (t *DeployHAProxyAsDaemonTask) Description() string {
	return t.Meta.Description
}

func (t *DeployHAProxyAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKH) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *DeployHAProxyAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := haproxystep.NewGenerateHAProxyConfigStepBuilder(runtimeCtx, "GenerateHAProxyDaemonConfig").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := haproxystep.NewEnableHAProxyStepBuilder(runtimeCtx, "EnableHAProxyService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := haproxystep.NewRestartHAProxyStepBuilder(runtimeCtx, "RestartHAProxyService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyDaemonConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartHAProxyService", Step: restartService, Hosts: loadbalancerHosts})

	fragment.AddDependency("GenerateHAProxyDaemonConfig", "EnableHAProxyService")
	fragment.AddDependency("EnableHAProxyService", "RestartHAProxyService")

	installPackage, err := haproxystep.NewInstallHAProxyStepBuilder(runtimeCtx, "InstallHAProxyPackage").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHAProxyPackage", Step: installPackage, Hosts: loadbalancerHosts})
	fragment.AddDependency("InstallHAProxyPackage", "GenerateHAProxyDaemonConfig")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
