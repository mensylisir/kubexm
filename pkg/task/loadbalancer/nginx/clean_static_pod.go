package nginx

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	nginxstep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanNginxStaticPodTask struct {
	task.Base
}

func NewCleanNginxStaticPodTask() task.Task {
	return &CleanNginxStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNginxStaticPod",
				Description: "Clean up NGINX static pod resources on master nodes",
			},
		},
	}
}

func (t *CleanNginxStaticPodTask) Name() string {
	return t.Meta.Name
}

func (t *CleanNginxStaticPodTask) Description() string {
	return t.Meta.Description
}

func (t *CleanNginxStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(common.InternalLBTypeNginx) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *CleanNginxStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	cleanStep := nginxstep.NewCleanNginxStaticPodStepBuilder(*runtimeCtx, "CleanNginxStaticPodResources").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanNginxStaticPodResources", Step: cleanStep, Hosts: workerHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
