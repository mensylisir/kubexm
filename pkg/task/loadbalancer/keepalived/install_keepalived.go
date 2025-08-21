package keepalived

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	keepalivedstep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/keepalived"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployKeepalivedTask struct {
	task.Base
}

func NewDeployKeepalivedTask() task.Task {
	return &DeployKeepalivedTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployKeepalived",
				Description: "Deploy Keepalived as a system service on load balancer nodes for VIP",
			},
		},
	}
}

func (t *DeployKeepalivedTask) Name() string {
	return t.Meta.Name
}

func (t *DeployKeepalivedTask) Description() string {
	return t.Meta.Description
}

func (t *DeployKeepalivedTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleLoadBalancer)) > 0, nil
}

func (t *DeployKeepalivedTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	generateConfig := keepalivedstep.NewGenerateKeepalivedConfigStepBuilder(*runtimeCtx, "GenerateKeepalivedConfig").Build()
	enableService := keepalivedstep.NewEnableKeepalivedStepBuilder(*runtimeCtx, "EnableKeepalivedService").Build()
	restartService := keepalivedstep.NewRestartKeepalivedStepBuilder(*runtimeCtx, "RestartKeepalivedService").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKeepalivedConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKeepalivedService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartKeepalivedService", Step: restartService, Hosts: loadbalancerHosts})

	fragment.AddDependency("GenerateKeepalivedConfig", "EnableKeepalivedService")
	fragment.AddDependency("EnableKeepalivedService", "RestartKeepalivedService")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Installing Keepalived package.")
		installPackage := keepalivedstep.NewInstallKeepalivedStepBuilder(*runtimeCtx, "InstallKeepalivedPackage").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "InstallKeepalivedPackage", Step: installPackage, Hosts: loadbalancerHosts})
		fragment.AddDependency("InstallKeepalivedPackage", "GenerateKeepalivedConfig")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping Keepalived package installation.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
