package nginx

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	nginxstep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/pkg/task"
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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKN) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleLoadBalancer)) > 0, nil
}

func (t *DeployNginxAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	generateConfig := nginxstep.NewGenerateNginxConfigStepBuilder(*runtimeCtx, "GenerateNginxDaemonConfig").Build()
	enableService := nginxstep.NewEnableNginxStepBuilder(*runtimeCtx, "EnableNginxService").Build()
	restartService := nginxstep.NewRestartNginxStepBuilder(*runtimeCtx, "RestartNginxService").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxDaemonConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableNginxService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartNginxService", Step: restartService, Hosts: loadbalancerHosts})

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Installing NGINX package.")
		installPackage := nginxstep.NewInstallNginxStepBuilder(*runtimeCtx, "InstallNginxPackage").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "InstallNginxPackage", Step: installPackage, Hosts: loadbalancerHosts})
		fragment.AddDependency("InstallNginxPackage", "GenerateNginxDaemonConfig")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping NGINX package installation.")
	}

	fragment.AddDependency("GenerateNginxDaemonConfig", "EnableNginxService")
	fragment.AddDependency("EnableNginxService", "RestartNginxService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
