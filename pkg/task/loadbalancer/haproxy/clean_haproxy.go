package haproxy

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	haproxystep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanHAProxyAsDaemonTask struct {
	task.Base
}

func NewCleanHAProxyAsDaemonTask() task.Task {
	return &CleanHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanHAProxyAsDaemon",
				Description: "Clean up HAProxy system service and related resources on master nodes",
			},
		},
	}
}

func (t *CleanHAProxyAsDaemonTask) Name() string {
	return t.Meta.Name
}

func (t *CleanHAProxyAsDaemonTask) Description() string {
	return t.Meta.Description
}

func (t *CleanHAProxyAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubexmKH) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *CleanHAProxyAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	stopService := haproxystep.NewStopHAProxyStepBuilder(*runtimeCtx, "StopHAProxyService").Build()
	disableService := haproxystep.NewDisableHAProxyStepBuilder(*runtimeCtx, "DisableHAProxyService").Build()
	cleanFilesAndPackage := haproxystep.NewCleanHAProxyStepBuilder(*runtimeCtx, "CleanHAProxyFilesAndPackage").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "StopHAProxyService", Step: stopService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableHAProxyService", Step: disableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanHAProxyFilesAndPackage", Step: cleanFilesAndPackage, Hosts: loadbalancerHosts})

	fragment.AddDependency("StopHAProxyService", "DisableHAProxyService")
	fragment.AddDependency("DisableHAProxyService", "CleanHAProxyFilesAndPackage")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
