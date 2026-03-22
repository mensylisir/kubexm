package nginx

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	nginxstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/internal/task"
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

	cleanStep, err := nginxstep.NewCleanNginxStaticPodStepBuilder(runtimeCtx, "CleanNginxStaticPodResources").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanNginxStaticPodResources", Step: cleanStep, Hosts: workerHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
