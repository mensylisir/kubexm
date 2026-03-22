package haproxy

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanHAProxyStaticPodTask struct {
	task.Base
}

func NewCleanHAProxyStaticPodTask() task.Task {
	return &CleanHAProxyStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanHAProxyStaticPod",
				Description: "Clean up HAProxy static pod resources on master nodes",
			},
		},
	}
}

func (t *CleanHAProxyStaticPodTask) Name() string {
	return t.Meta.Name
}

func (t *CleanHAProxyStaticPodTask) Description() string {
	return t.Meta.Description
}

func (t *CleanHAProxyStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(common.InternalLBTypeHAProxy) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *CleanHAProxyStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	cleanStep, err := haproxystep.NewCleanHAProxyStaticPodStepBuilder(runtimeCtx, "CleanHAProxyStaticPodResources").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanHAProxyStaticPodResources", Step: cleanStep, Hosts: workerHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
