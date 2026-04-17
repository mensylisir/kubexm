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

// DeployNginxAsStaticPodTask deploys NGINX as a static pod on worker nodes.
// This task composes atomic steps:
// 1. prepare_dirs - create necessary directories
// 2. render_config - render NGINX configuration
// 3. copy_config - copy config to remote hosts
// 4. render_pod - render static pod manifest
// 5. copy_pod - copy pod manifest to manifest directory
type DeployNginxAsStaticPodTask struct {
	task.Base
}

func NewDeployNginxAsStaticPodTask() task.Task {
	return &DeployNginxAsStaticPodTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployNginxAsStaticPod",
				Description: "Deploy NGINX as a static pod on worker nodes",
			},
		},
	}
}

func (t *DeployNginxAsStaticPodTask) Name() string        { return t.Meta.Name }
func (t *DeployNginxAsStaticPodTask) Description() string { return t.Meta.Description }

func (t *DeployNginxAsStaticPodTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type != string(pkgcommon.InternalLBTypeNginx) {
		return false, nil
	}
	// StaticPod mode only applies to kubeadm type (not kubexm which uses Daemon mode)
	kubernetesType := cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Type == string(pkgcommon.KubernetesDeploymentTypeKubexm)
	return !kubernetesType && len(ctx.GetHostsByRole(pkgcommon.RoleMaster)) > 1, nil
}

func (t *DeployNginxAsStaticPodTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(pkgcommon.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Prepare directories
	dirs := []string{pkgcommon.DefaultNginxConfigDir}
	prepareDirs, err := lbcommon.NewPrepareLBDirsStepBuilder(runtimeCtx, "PrepareNginxStaticPodDirs", dirs).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare dirs step: %w", err)
	}

	// Step 2: Render NGINX config
	renderConfig, err := nginxstep.NewRenderNginxConfigStepBuilder(runtimeCtx, "RenderNginxStaticPodConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render config step: %w", err)
	}

	// Step 3: Copy config to remote
	copyConfig, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyNginxStaticPodConfig",
		pkgcommon.DefaultNginxConfigFilePath,
		"nginx_rendered_config",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy config step: %w", err)
	}

	// Step 4: Render pod manifest
	renderPod, err := nginxstep.NewRenderNginxPodManifestStepBuilder(runtimeCtx, "RenderNginxStaticPodManifest").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create render pod step: %w", err)
	}

	// Step 5: Copy pod manifest to manifest directory
	copyPod, err := lbcommon.NewCopyFileStepBuilder(
		runtimeCtx,
		"CopyNginxStaticPodManifest",
		nginxstep.NginxManifestPath(),
		"nginx_rendered_pod_manifest",
		"0644",
	).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create copy pod step: %w", err)
	}

	// Add nodes to fragment
	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareNginxStaticPodDirs", Step: prepareDirs, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderNginxStaticPodConfig", Step: renderConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyNginxStaticPodConfig", Step: copyConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RenderNginxStaticPodManifest", Step: renderPod, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CopyNginxStaticPodManifest", Step: copyPod, Hosts: workerHosts})

	// Set up dependencies
	fragment.AddDependency("RenderNginxStaticPodConfig", "PrepareNginxStaticPodDirs")
	fragment.AddDependency("CopyNginxStaticPodConfig", "RenderNginxStaticPodConfig")
	fragment.AddDependency("RenderNginxStaticPodManifest", "CopyNginxStaticPodConfig")
	fragment.AddDependency("CopyNginxStaticPodManifest", "RenderNginxStaticPodManifest")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployNginxAsStaticPodTask)(nil)
