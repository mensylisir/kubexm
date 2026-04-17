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

// DeployHAProxyAsStaticPodTask deploys HAProxy as a static pod on worker nodes.
// This task composes atomic steps to achieve the goal:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render HAProxy configuration
// 3. copy_config - copy config to remote hosts
// 4. render_pod - render static pod manifest
// 5. copy_pod - copy pod manifest to manifest directory
type DeployHAProxyAsStaticPodTask struct {
	task.Base
}

func NewDeployHAProxyAsStaticPodTask() task.Task {
	return &DeployHAProxyAsStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHAProxyAsStaticPod",
				Description: "Deploy HAProxy as a static pod on worker nodes",
			},
		},
	}
}

func (t *DeployHAProxyAsStaticPodTask) Name() string        { return t.Meta.Name }
func (t *DeployHAProxyAsStaticPodTask) Description() string { return t.Meta.Description }

func (t *DeployHAProxyAsStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(pkgcommon.InternalLBTypeHAProxy) {
		return false, nil
	}
	// StaticPod mode only applies to kubeadm type (not kubexm which uses Daemon mode)
	kubernetesType := cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(pkgcommon.KubernetesDeploymentTypeKubexm)
	return !kubernetesType && len(ctx.GetHostsByRole(pkgcommon.RoleMaster)) > 1, nil
}

func (t *DeployHAProxyAsStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{
		pkgcommon.HAProxyDefaultConfDirTarget,
		pkgcommon.HAProxyDefaultConfigFileTarget, // parent dir
	}
	// Use parent dir only
	dirs = []string{pkgcommon.HAProxyDefaultConfDirTarget}

	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareHaproxyDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render HAProxy config
	renderConfig, err := haproxystep.NewRenderHAProxyConfigStepBuilder(runtimeCtx, "RenderHAProxyConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyHAProxyConfig",
		pkgcommon.HAProxyDefaultConfigFileTarget,
		"haproxy_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Render pod manifest
	renderPod, err := haproxystep.NewRenderHAProxyPodManifestStepBuilder(runtimeCtx, "RenderHAProxyPodManifest").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render pod step: %w", err)
	}

	// Step 5: Copy pod manifest to manifest directory
	manifestPath := haproxystep.ManifestPath()
	copyPod, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyHAProxyPodManifest",
		manifestPath,
		"haproxy_rendered_pod_manifest",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy pod step: %w", err)
	}

	// Add nodes to fragment
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareHaproxyDirs", Step: prepareDirs, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderHAProxyConfig", Step: renderConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyHAProxyConfig", Step: copyConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderHAProxyPodManifest", Step: renderPod, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyHAProxyPodManifest", Step: copyPod, Hosts: workerHosts})

	// Set up dependencies: each step depends on the previous one completing
	fragment.AddDependency("RenderHAProxyConfig", "PrepareHaproxyDirs")
	fragment.AddDependency("CopyHAProxyConfig", "RenderHAProxyConfig")
	fragment.AddDependency("RenderHAProxyPodManifest", "CopyHAProxyConfig")
	fragment.AddDependency("CopyHAProxyPodManifest", "RenderHAProxyPodManifest")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployHAProxyAsStaticPodTask)(nil)
