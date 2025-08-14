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

type CleanNginxAsDaemonTask struct {
	task.Base
}

func NewCleanNginxAsDaemonTask() task.Task {
	return &CleanNginxAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNginxAsDaemon",
				Description: "Clean up NGINX system service and related resources on load balancer nodes",
			},
		},
	}
}

func (t *CleanNginxAsDaemonTask) Name() string {
	return t.Meta.Name
}

func (t *CleanNginxAsDaemonTask) Description() string {
	return t.Meta.Description
}

func (t *CleanNginxAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKN) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleLoadBalancer)) > 0, nil
}

func (t *CleanNginxAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	stopService := nginxstep.NewStopNginxStepBuilder(*runtimeCtx, "StopNginxService").Build()
	disableService := nginxstep.NewDisableNginxStepBuilder(*runtimeCtx, "DisableNginxService").Build()
	cleanFilesAndPackage := nginxstep.NewCleanNginxStepBuilder(*runtimeCtx, "CleanNginxPackage").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopNginxService", Step: stopService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableNginxService", Step: disableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanNginxPackage", Step: cleanFilesAndPackage, Hosts: loadbalancerHosts})

	fragment.AddDependency("StopNginxService", "DisableNginxService")
	fragment.AddDependency("DisableNginxService", "CleanNginxPackage")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
