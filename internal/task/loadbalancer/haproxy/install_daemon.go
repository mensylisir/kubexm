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

// DeployHAProxyAsDaemonTask deploys HAProxy as a systemd service on load balancer nodes.
// This task composes atomic steps:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render HAProxy configuration
// 3. copy_config - copy config to remote hosts
// 4. enable_service - enable haproxy service
// 5. restart_service - restart haproxy service
type DeployHAProxyAsDaemonTask struct {
	task.Base
}

func NewDeployHAProxyAsDaemonTask() task.Task {
	return &DeployHAProxyAsDaemonTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyAsDaemon",
				Description: "Deploy HAProxy as a systemd service on load balancer nodes",
			},
		},
	}
}

func (t *DeployHAProxyAsDaemonTask) Name() string        { return t.Meta.Name }
func (t *DeployHAProxyAsDaemonTask) Description() string { return t.Meta.Description }

func (t *DeployHAProxyAsDaemonTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(pkgcommon.ExternalLBTypeKubexmKH) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(pkgcommon.RoleMaster)) > 1 && len(ctx.GetHostsByRole(pkgcommon.RoleLoadBalancer)) > 0, nil
}

func (t *DeployHAProxyAsDaemonTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	loadbalancerHosts := ctx.GetHostsByRole(pkgcommon.RoleLoadBalancer)
	if len(loadbalancerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{pkgcommon.HAProxyDefaultConfDirTarget}
	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareHaproxyDaemonDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render HAProxy config
	renderConfig, err := haproxystep.NewRenderHAProxyConfigStepBuilder(runtimeCtx, "RenderHAProxyDaemonConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyHAProxyDaemonConfig",
		pkgcommon.HAProxyDefaultConfigFileTarget,
		"haproxy_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Install package
	installPackage, err := haproxystep.NewInstallHAProxyStepBuilder(runtimeCtx, "InstallHAProxyPackage").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create install package step: %w", err)
	}

	// Step 5: Enable service
	enableService, err := haproxystep.NewEnableHAProxyStepBuilder(runtimeCtx, "EnableHAProxyService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create enable service step: %w", err)
	}

	// Step 6: Restart service
	restartService, err := haproxystep.NewRestartHAProxyStepBuilder(runtimeCtx, "RestartHAProxyService").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create restart service step: %w", err)
	}

	// Add nodes
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareHaproxyDaemonDirs", Step: prepareDirs, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderHAProxyDaemonConfig", Step: renderConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyHAProxyDaemonConfig", Step: copyConfig, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHAProxyPackage", Step: installPackage, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableHAProxyService", Step: enableService, Hosts: loadbalancerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartHAProxyService", Step: restartService, Hosts: loadbalancerHosts})

	// Dependencies
	fragment.AddDependency("RenderHAProxyDaemonConfig", "PrepareHaproxyDaemonDirs")
	fragment.AddDependency("CopyHAProxyDaemonConfig", "RenderHAProxyDaemonConfig")
	fragment.AddDependency("InstallHAProxyPackage", "CopyHAProxyDaemonConfig")
	fragment.AddDependency("EnableHAProxyService", "InstallHAProxyPackage")
	fragment.AddDependency("RestartHAProxyService", "EnableHAProxyService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployHAProxyAsDaemonTask)(nil)
