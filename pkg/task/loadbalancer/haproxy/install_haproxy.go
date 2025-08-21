package haproxy

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	haproxystep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployHAProxyAsDaemonTask struct {
	task.Base
}

func NewDeployHAProxyAsDaemonTask() task.Task {
	return &DeployHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyAsDaemon",
				Description: "Deploy HAProxy as a system service on master nodes",
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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKH) {
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

	generateConfig := haproxystep.NewGenerateHAProxyConfigStepBuilder(*runtimeCtx, "GenerateHAProxyDaemonConfig").Build()
	enableService := haproxystep.NewEnableHAProxyStepBuilder(*runtimeCtx, "EnableHAProxyService").Build()
	restartService := haproxystep.NewRestartHAProxyStepBuilder(*runtimeCtx, "RestartHAProxyService").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyDaemonConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartHAProxyService", Step: restartService, Hosts: loadbalancerHosts})

	fragment.AddDependency("GenerateHAProxyDaemonConfig", "EnableHAProxyService")
	fragment.AddDependency("EnableHAProxyService", "RestartHAProxyService")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading HAProxy artifacts.")
		installPackage := haproxystep.NewInstallHAProxyStepBuilder(*runtimeCtx, "InstallHAProxyPackage").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "InstallHAProxyPackage", Step: installPackage, Hosts: loadbalancerHosts})
		fragment.AddDependency("InstallHAProxyPackage", "GenerateHAProxyDaemonConfig")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping HAProxy package installation.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
