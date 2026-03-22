package nginx

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nginxstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployNginxAsDaemonTask struct {
	task.Base
}

func NewDeployNginxAsDaemonTask() task.Task {
	return &DeployNginxAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxAsDaemon",
				Description: "Deploy NGINX as a system service on load balancer nodes",
			},
		},
	}
}

func (t *DeployNginxAsDaemonTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNginxAsDaemonTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNginxAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKN) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(common.RoleMaster)) <= 1 {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleLoadBalancer)) > 0, nil
}

func (t *DeployNginxAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := nginxstep.NewGenerateNginxConfigStepBuilder(runtimeCtx, "GenerateNginxDaemonConfig").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := nginxstep.NewEnableNginxStepBuilder(runtimeCtx, "EnableNginxService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := nginxstep.NewRestartNginxStepBuilder(runtimeCtx, "RestartNginxService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxDaemonConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableNginxService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartNginxService", Step: restartService, Hosts: loadbalancerHosts})

	installPackage, err := nginxstep.NewInstallNginxStepBuilder(runtimeCtx, "InstallNginxPackage").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNginxPackage", Step: installPackage, Hosts: loadbalancerHosts})
	fragment.AddDependency("InstallNginxPackage", "GenerateNginxDaemonConfig")

	fragment.AddDependency("GenerateNginxDaemonConfig", "EnableNginxService")
	fragment.AddDependency("EnableNginxService", "RestartNginxService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
