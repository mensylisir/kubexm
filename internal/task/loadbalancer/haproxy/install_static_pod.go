package haproxy

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployHAProxyAsStaticPodTask struct {
	task.Base
}

func NewDeployHAProxyAsStaticPodTask() task.Task {
	return &DeployHAProxyAsStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyAsStaticPod",
				Description: "Deploy HAProxy as a static pod on worker nodes",
			},
		},
	}
}

func (t *DeployHAProxyAsStaticPodTask) Name() string {
	return t.Meta.Name
}

func (t *DeployHAProxyAsStaticPodTask) Description() string {
	return t.Meta.Description
}

func (t *DeployHAProxyAsStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(common.InternalLBTypeHAProxy) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *DeployHAProxyAsStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := haproxystep.NewGenerateHAProxyConfigStepBuilder(runtimeCtx, "GenerateHAProxyConfigForPod").Build()
	if err != nil {
		return nil, err
	}
	generatePodManifest, err := haproxystep.NewGenerateHAProxyStaticPodStepBuilder(runtimeCtx, "GenerateHAProxyStaticPodManifest").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyConfigForPod", Step: generateConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHAProxyStaticPodManifest", Step: generatePodManifest, Hosts: workerHosts})

	fragment.AddDependency("GenerateHAProxyConfigForPod", "GenerateHAProxyStaticPodManifest")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
