package nginx

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nginxstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployNginxAsStaticPodTask struct {
	task.Base
}

func NewDeployNginxAsStaticPodTask() task.Task {
	return &DeployNginxAsStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxAsStaticPod",
				Description: "Deploy NGINX as a static pod on worker nodes for internal load balancing",
			},
		},
	}
}

func (t *DeployNginxAsStaticPodTask) Name() string {
	return t.Meta.Name
}

func (t *DeployNginxAsStaticPodTask) Description() string {
	return t.Meta.Description
}

func (t *DeployNginxAsStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(common.InternalLBTypeNginx) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *DeployNginxAsStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workHosts := ctx.GetHostsByRole(common.RoleWorker)

	if len(workHosts) == 0 {
		return fragment, nil
	}

	generateConfig, err := nginxstep.NewGenerateNginxConfigStepBuilder(runtimeCtx, "GenerateNginxConfigForPod").Build()
	if err != nil {
		return nil, err
	}
	generatePodManifest, err := nginxstep.NewGenerateNginxStaticPodStepBuilder(runtimeCtx, "GenerateNginxStaticPodManifest").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxConfigForPod", Step: generateConfig, Hosts: workHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxStaticPodManifest", Step: generatePodManifest, Hosts: workHosts})

	fragment.AddDependency("GenerateNginxConfigForPod", "GenerateNginxStaticPodManifest")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
