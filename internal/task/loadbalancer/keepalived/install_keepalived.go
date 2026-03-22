package keepalived

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	keepalivedstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/keepalived"
	"github.com/mensylisir/kubexm/internal/task"
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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		return false, nil
	}
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKH) &&
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKN) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(common.RoleMaster)) <= 1 {
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

	generateConfig, err := keepalivedstep.NewGenerateKeepalivedConfigStepBuilder(runtimeCtx, "GenerateKeepalivedConfig").Build()
	if err != nil {
		return nil, err
	}
	enableService, err := keepalivedstep.NewEnableKeepalivedStepBuilder(runtimeCtx, "EnableKeepalivedService").Build()
	if err != nil {
		return nil, err
	}
	restartService, err := keepalivedstep.NewRestartKeepalivedStepBuilder(runtimeCtx, "RestartKeepalivedService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKeepalivedConfig", Step: generateConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKeepalivedService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartKeepalivedService", Step: restartService, Hosts: loadbalancerHosts})

	fragment.AddDependency("GenerateKeepalivedConfig", "EnableKeepalivedService")
	fragment.AddDependency("EnableKeepalivedService", "RestartKeepalivedService")

	installPackage, err := keepalivedstep.NewInstallKeepalivedStepBuilder(runtimeCtx, "InstallKeepalivedPackage").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKeepalivedPackage", Step: installPackage, Hosts: loadbalancerHosts})
	fragment.AddDependency("InstallKeepalivedPackage", "GenerateKeepalivedConfig")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
