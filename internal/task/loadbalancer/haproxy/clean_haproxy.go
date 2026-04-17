package haproxy

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	stopService, err := haproxystep.NewStopHAProxyStepBuilder(runtimeCtx, "StopHAProxyService").Build()
	if err != nil {
		return nil, err
	}
	disableService, err := haproxystep.NewDisableHAProxyStepBuilder(runtimeCtx, "DisableHAProxyService").Build()
	if err != nil {
		return nil, err
	}
	cleanFilesAndPackage, err := haproxystep.NewCleanHAProxyStepBuilder(runtimeCtx, "CleanHAProxyFilesAndPackage").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopHAProxyService", Step: stopService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableHAProxyService", Step: disableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CleanHAProxyFilesAndPackage", Step: cleanFilesAndPackage, Hosts: loadbalancerHosts})

	fragment.AddDependency("StopHAProxyService", "DisableHAProxyService")
	fragment.AddDependency("DisableHAProxyService", "CleanHAProxyFilesAndPackage")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
