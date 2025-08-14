package keepalived

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	keepalivedstep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/keepalived"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanKeepalivedTask struct {
	task.Base
}

func NewCleanKeepalivedTask() task.Task {
	return &CleanKeepalivedTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanKeepalived",
				Description: "Clean up Keepalived system service and related resources on load balancer nodes",
			},
		},
	}
}

func (t *CleanKeepalivedTask) Name() string {
	return t.Meta.Name
}

func (t *CleanKeepalivedTask) Description() string {
	return t.Meta.Description
}

func (t *CleanKeepalivedTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleLoadBalancer)) > 0, nil
}

func (t *CleanKeepalivedTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	stopService := keepalivedstep.NewStopKeepalivedStepBuilder(*runtimeCtx, "StopKeepalivedService").Build()
	disableService := keepalivedstep.NewDisableKeepalivedStepBuilder(*runtimeCtx, "DisableKeepalivedService").Build()
	cleanFilesAndPackage := keepalivedstep.NewCleanKeepalivedStepBuilder(*runtimeCtx, "CleanKeepalivedPackage").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopKeepalivedService", Step: stopService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableKeepalivedService", Step: disableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanKeepalivedPackage", Step: cleanFilesAndPackage, Hosts: loadbalancerHosts})

	fragment.AddDependency("StopKeepalivedService", "DisableKeepalivedService")
	fragment.AddDependency("DisableKeepalivedService", "CleanKeepalivedPackage")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
