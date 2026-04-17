package nginx

import (
	"fmt"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	lbstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/nginx"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	"github.com/mensylisir/kubexm/internal/task"
)

// DeployNginxAsDaemonTask deploys NGINX as a systemd service on load balancer nodes.
// This task composes atomic steps:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render NGINX configuration
// 3. copy_config - copy config to remote hosts
// 4. enable_service - enable nginx service
// 5. restart_service - restart nginx service
type DeployNginxAsDaemonTask struct {
	task.Base
}

func NewDeployNginxAsDaemonTask() task.Task {
	return &DeployNginxAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxAsDaemon",
				Description: "Deploy NGINX as a systemd service on load balancer nodes",
			},
		},
	}
}

func (t *DeployNginxAsDaemonTask) Name() string        { return t.Meta.Name }
func (t *DeployNginxAsDaemonTask) Description() string { return t.Meta.Description }

func (t *DeployNginxAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(pkgcommon.ExternalLBTypeKubexmKN) {
		return false, nil
	}
	if len(ctx.GetHostsByRole(pkgcommon.RoleMaster)) <= 1 {
		return false, nil
	}
	return len(ctx.GetHostsByRole(pkgcommon.RoleLoadBalancer)) > 0, nil
}

func (t *DeployNginxAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(pkgcommon.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{pkgcommon.DefaultNginxConfigDir}
	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareNginxDaemonDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render NGINX config
	renderConfig, err := lbstep.NewRenderNginxConfigStepBuilder(runtimeCtx, "RenderNginxDaemonConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyNginxDaemonConfig",
		pkgcommon.DefaultNginxConfigFilePath,
		"nginx_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Install package
	installPackage, err := lbstep.NewInstallNginxStepBuilder(runtimeCtx, "InstallNginxPackage").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create install package step: %w", err)
	}

	// Step 5: Enable service
	enableService, err := lbstep.NewEnableNginxStepBuilder(runtimeCtx, "EnableNginxService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create enable service step: %w", err)
	}

	// Step 6: Restart service
	restartService, err := lbstep.NewRestartNginxStepBuilder(runtimeCtx, "RestartNginxService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create restart service step: %w", err)
	}

	// Add nodes
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareNginxDaemonDirs", Step: prepareDirs, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderNginxDaemonConfig", Step: renderConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyNginxDaemonConfig", Step: copyConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallNginxPackage", Step: installPackage, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableNginxService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartNginxService", Step: restartService, Hosts: loadbalancerHosts})

	// Dependencies
	fragment.AddDependency("RenderNginxDaemonConfig", "PrepareNginxDaemonDirs")
	fragment.AddDependency("CopyNginxDaemonConfig", "RenderNginxDaemonConfig")
	fragment.AddDependency("InstallNginxPackage", "CopyNginxDaemonConfig")
	fragment.AddDependency("EnableNginxService", "InstallNginxPackage")
	fragment.AddDependency("RestartNginxService", "EnableNginxService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployNginxAsDaemonTask)(nil)
