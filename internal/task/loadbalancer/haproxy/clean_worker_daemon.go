package haproxy

import (
	"fmt"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	"github.com/mensylisir/kubexm/internal/task"
)

// CleanHAProxyOnWorkersTask cleans up HAProxy systemd service from worker nodes.
type CleanHAProxyOnWorkersTask struct {
	task.Base
}

func NewCleanHAProxyOnWorkersTask() task.Task {
	return &CleanHAProxyOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanHAProxyOnWorkers",
				Description: "Clean up HAProxy systemd service from worker nodes",
			},
		},
	}
}

func (t *CleanHAProxyOnWorkersTask) Name() string        { return t.Meta.Name }
func (t *CleanHAProxyOnWorkersTask) Description() string { return t.Meta.Description }

func (t *CleanHAProxyOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return false, nil
	}
	// Only required for kubexm internal LB mode
	if cfg.Spec.Kubernetes == nil || cfg.Spec.Kubernetes.Type != string(pkgcommon.KubernetesDeploymentTypeKubexm) {
		return false, nil
	}
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled {
		return false, nil
	}
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(pkgcommon.InternalLBTypeHAProxy) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(pkgcommon.RoleWorker)) > 0, nil
}

func (t *CleanHAProxyOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Stop and disable service first
	stopService, err := lbcommon.NewStopLBServiceStepBuilder(runtimeCtx, "StopHAProxyWorkerService", "haproxy").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create stop service step: %w", err)
	}
	disableService, err := lbcommon.NewDisableLBServiceStepBuilder(runtimeCtx, "DisableHAProxyWorkerService", "haproxy").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create disable service step: %w", err)
	}

	// Remove config and service files
	removeConfig, err := lbcommon.NewRemoveLBFileStepBuilder(runtimeCtx, "RemoveHAProxyWorkerConfig", pkgcommon.HAProxyDefaultConfigFileTarget).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create remove config step: %w", err)
	}
	removeServiceFile, err := lbcommon.NewRemoveLBFileStepBuilder(runtimeCtx, "RemoveHAProxyWorkerServiceFile", pkgcommon.HAProxyDefaultSystemdFile).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create remove service file step: %w", err)
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopHAProxyWorkerService", Step: stopService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableHAProxyWorkerService", Step: disableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHAProxyWorkerConfig", Step: removeConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveHAProxyWorkerServiceFile", Step: removeServiceFile, Hosts: workerHosts})

	fragment.AddDependency("StopHAProxyWorkerService", "DisableHAProxyWorkerService")
	fragment.AddDependency("DisableHAProxyWorkerService", "RemoveHAProxyWorkerConfig")
	fragment.AddDependency("RemoveHAProxyWorkerConfig", "RemoveHAProxyWorkerServiceFile")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*CleanHAProxyOnWorkersTask)(nil)
