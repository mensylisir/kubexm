package nginx

import (
	"fmt"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	"github.com/mensylisir/kubexm/internal/task"
)

// CleanNginxOnWorkersTask cleans up Nginx systemd service from worker nodes.
type CleanNginxOnWorkersTask struct {
	task.Base
}

func NewCleanNginxOnWorkersTask() task.Task {
	return &CleanNginxOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNginxOnWorkers",
				Description: "Clean up Nginx systemd service from worker nodes",
			},
		},
	}
}

func (t *CleanNginxOnWorkersTask) Name() string        { return t.Meta.Name }
func (t *CleanNginxOnWorkersTask) Description() string { return t.Meta.Description }

func (t *CleanNginxOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(pkgcommon.InternalLBTypeNginx) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(pkgcommon.RoleWorker)) > 0, nil
}

func (t *CleanNginxOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Stop and disable service first
	stopService, err := lbcommon.NewStopLBServiceStepBuilder(runtimeCtx, "StopNginxWorkerService", "nginx").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create stop service step: %w", err)
	}
	disableService, err := lbcommon.NewDisableLBServiceStepBuilder(runtimeCtx, "DisableNginxWorkerService", "nginx").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create disable service step: %w", err)
	}

	// Remove config and service files
	removeConfig, err := lbcommon.NewRemoveLBFileStepBuilder(runtimeCtx, "RemoveNginxWorkerConfig", pkgcommon.DefaultNginxConfigFilePathTarget).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create remove config step: %w", err)
	}
	removeServiceFile, err := lbcommon.NewRemoveLBFileStepBuilder(runtimeCtx, "RemoveNginxWorkerServiceFile", "/etc/systemd/system/nginx.service").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create remove service file step: %w", err)
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "StopNginxWorkerService", Step: stopService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableNginxWorkerService", Step: disableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveNginxWorkerConfig", Step: removeConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveNginxWorkerServiceFile", Step: removeServiceFile, Hosts: workerHosts})

	fragment.AddDependency("StopNginxWorkerService", "DisableNginxWorkerService")
	fragment.AddDependency("DisableNginxWorkerService", "RemoveNginxWorkerConfig")
	fragment.AddDependency("RemoveNginxWorkerConfig", "RemoveNginxWorkerServiceFile")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*CleanNginxOnWorkersTask)(nil)
