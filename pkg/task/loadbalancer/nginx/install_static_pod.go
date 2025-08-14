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

type DeployNginxAsStaticPodTask struct {
	task.Base
}

func NewDeployNginxAsStaticPodTask() task.Task {
	return &DeployNginxAsStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxAsStaticPod",
				Description: "Deploy NGINX as a static pod on master nodes for internal load balancing",
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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(common.InternalLBTypeNginx) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *DeployNginxAsStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	workHosts := ctx.GetHostsByRole(common.RoleWorker)

	if len(workHosts) == 0 {
		return fragment, nil
	}

	generateConfig := nginxstep.NewGenerateNginxConfigStepBuilder(*runtimeCtx, "GenerateNginxConfigForPod").Build()
	generatePodManifest := nginxstep.NewGenerateNginxStaticPodStepBuilder(*runtimeCtx, "GenerateNginxStaticPodManifest").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxConfigForPod", Step: generateConfig, Hosts: workHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNginxStaticPodManifest", Step: generatePodManifest, Hosts: workHosts})

	fragment.AddDependency("GenerateNginxConfigForPod", "GenerateNginxStaticPodManifest")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
