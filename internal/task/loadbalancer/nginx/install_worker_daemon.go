package nginx

import (
	"fmt"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	nginxstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	"github.com/mensylisir/kubexm/internal/task"
)

// DeployNginxOnWorkersTask deploys NGINX as a systemd service on worker nodes.
// This task composes atomic steps:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render NGINX configuration
// 3. copy_config - copy config to remote hosts
// 4. enable_service - enable nginx service
// 5. restart_service - restart nginx service
type DeployNginxOnWorkersTask struct {
	task.Base
}

func NewDeployNginxOnWorkersTask() task.Task {
	return &DeployNginxOnWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxOnWorkers",
				Description: "Deploy NGINX as a systemd service on worker nodes",
			},
		},
	}
}

func (t *DeployNginxOnWorkersTask) Name() string        { return t.Meta.Name }
func (t *DeployNginxOnWorkersTask) Description() string { return t.Meta.Description }

func (t *DeployNginxOnWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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
	if ha.Internal.Type != string(pkgcommon.InternalLBTypeNginx) {
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

func (t *DeployNginxOnWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{pkgcommon.DefaultNginxConfigDir}
	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareNginxWorkerDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render NGINX config
	renderConfig, err := nginxstep.NewRenderNginxConfigStepBuilder(runtimeCtx, "RenderNginxWorkerConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyNginxWorkerConfig",
		pkgcommon.DefaultNginxConfigFilePath,
		"nginx_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Install package
	installPackage, err := nginxstep.NewInstallNginxStepBuilder(runtimeCtx, "InstallNginxWorkerPackage").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create install package step: %w", err)
	}

	// Step 5: Enable service
	enableService, err := nginxstep.NewEnableNginxStepBuilder(runtimeCtx, "EnableNginxWorkerService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create enable service step: %w", err)
	}

	// Step 6: Restart service
	restartService, err := nginxstep.NewRestartNginxStepBuilder(runtimeCtx, "RestartNginxWorkerService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create restart service step: %w", err)
	}

	// Add nodes
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareNginxWorkerDirs", Step: prepareDirs, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderNginxWorkerConfig", Step: renderConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyNginxWorkerConfig", Step: copyConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNginxWorkerPackage", Step: installPackage, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableNginxWorkerService", Step: enableService, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartNginxWorkerService", Step: restartService, Hosts: workerHosts})

	// Dependencies
	fragment.AddDependency("RenderNginxWorkerConfig", "PrepareNginxWorkerDirs")
	fragment.AddDependency("CopyNginxWorkerConfig", "RenderNginxWorkerConfig")
	fragment.AddDependency("InstallNginxWorkerPackage", "CopyNginxWorkerConfig")
	fragment.AddDependency("EnableNginxWorkerService", "InstallNginxWorkerPackage")
	fragment.AddDependency("RestartNginxWorkerService", "EnableNginxWorkerService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployNginxOnWorkersTask)(nil)
