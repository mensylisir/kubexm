package haproxy

import (
	"fmt"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	haproxystep "github.com/mensylisir/kubexm/internal/step/loadbalancer/haproxy"
	"github.com/mensylisir/kubexm/internal/task"
)

// DeployHAProxyOnWorkersTask deploys HAProxy as a systemd service on worker nodes.
// This task composes atomic steps:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render HAProxy configuration
// 3. copy_config - copy config to remote hosts
// 4. copy_service - copy systemd service file
// 5. enable_service - enable haproxy service
// 6. restart_service - restart haproxy service
type DeployHAProxyOnWorkersTask struct {
	task.Base
}

func NewDeployHAProxyOnWorkersTask() task.Task {
	return &DeployHAProxyOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyOnWorkers",
				Description: "Deploy HAProxy as a systemd service on worker nodes",
			},
		},
	}
}

func (t *DeployHAProxyOnWorkersTask) Name() string        { return t.Meta.Name }
func (t *DeployHAProxyOnWorkersTask) Description() string { return t.Meta.Description }

func (t *DeployHAProxyOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint == nil || cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil {
		return false, nil
	}
	ha := cfg.Spec.ControlPlaneEndpoint.HighAvailability
	if ha.Enabled == nil || !*ha.Enabled {
		return false, nil
	}
	if ha.Internal == nil || ha.Internal.Enabled == nil || !*ha.Internal.Enabled {
		return false, nil
	}
	if ha.Internal.Type != string(pkgcommon.InternalLBTypeHAProxy) {
		return false, nil
	}
	if cfg.Spec.Kubernetes == nil || cfg.Spec.Kubernetes.Type != string(pkgcommon.KubernetesDeploymentTypeKubexm) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(pkgcommon.RoleMaster)) <= 1 {
		return false, nil
	}
	return len(ctx.GetHostsByRole(pkgcommon.RoleWorker)) > 0, nil
}

func (h *DeployHAProxyOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(h.Name())
	runtimeCtx := ctx.ForTask(h.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{pkgcommon.HAProxyDefaultConfDirTarget}
	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareHaproxyWorkerDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render HAProxy config
	renderConfig, err := haproxystep.NewRenderHAProxyConfigStepBuilder(runtimeCtx, "RenderHAProxyWorkerConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyHAProxyWorkerConfig",
		pkgcommon.HAProxyDefaultConfigFileTarget,
		"haproxy_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Render systemd service file
	renderService, err := haproxystep.NewRenderHAProxyServiceStepBuilder(runtimeCtx, "RenderHAProxyWorkerService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render service step: %w", err)
	}

	// Step 5: Copy systemd service to remote
	copyService, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyHAProxyWorkerService",
		pkgcommon.HAProxyDefaultSystemdFile,
		"haproxy_worker_service",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy service step: %w", err)
	}

	// Step 6: Enable service (using existing step)
	enableService, err := haproxystep.NewEnableHAProxyStepBuilder(runtimeCtx, "EnableHAProxyWorkerService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create enable service step: %w", err)
	}

	// Step 7: Restart service (using existing step)
	restartService, err := haproxystep.NewRestartHAProxyStepBuilder(runtimeCtx, "RestartHAProxyWorkerService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create restart service step: %w", err)
	}

	// Add nodes
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareHaproxyWorkerDirs", Step: prepareDirs, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderHAProxyWorkerConfig", Step: renderConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyHAProxyWorkerConfig", Step: copyConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderHAProxyWorkerService", Step: renderService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyHAProxyWorkerService", Step: copyService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyWorkerService", Step: enableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartHAProxyWorkerService", Step: restartService, Hosts: workerHosts})

	// Dependencies: config chain and service chain
	fragment.AddDependency("RenderHAProxyWorkerConfig", "PrepareHaproxyWorkerDirs")
	fragment.AddDependency("CopyHAProxyWorkerConfig", "RenderHAProxyWorkerConfig")
	fragment.AddDependency("RenderHAProxyWorkerService", "CopyHAProxyWorkerConfig")
	fragment.AddDependency("CopyHAProxyWorkerService", "RenderHAProxyWorkerService")
	fragment.AddDependency("EnableHAProxyWorkerService", "CopyHAProxyWorkerService")
	fragment.AddDependency("RestartHAProxyWorkerService", "EnableHAProxyWorkerService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployHAProxyOnWorkersTask)(nil)
