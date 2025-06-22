package runtime

import (
	"context"
	"fmt"
	"path/filepath" // Added for artifact path helpers
	"time"          // Added for time.Duration

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Corrected path
	"github.com/mensylisir/kubexm/pkg/common"                // Added for common.Default*Dir constants
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine" // For engine.Engine, engine.EngineExecuteContext
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"     // For step.StepContext
	"github.com/mensylisir/kubexm/pkg/pipeline" // For pipeline.PipelineContext
	"github.com/mensylisir/kubexm/pkg/module"   // For module.ModuleContext
	"github.com/mensylisir/kubexm/pkg/task"     // For task.TaskContext
)

type Context struct {
	GoCtx         context.Context
	Logger        *logger.Logger
	Engine        engine.Engine
	Runner        runner.Runner
	ClusterConfig *v1alpha1.Cluster
	HostRuntimes  map[string]*HostRuntime
	ConnectionPool *connector.ConnectionPool

	GlobalWorkDir string
	GlobalVerbose bool
	GlobalIgnoreErr bool
	GlobalConnectionTimeout time.Duration

	internalPipelineCache cache.PipelineCache // Renamed field
	internalModuleCache   cache.ModuleCache   // Renamed field
	internalTaskCache     cache.TaskCache     // Renamed field
	internalStepCache     cache.StepCache     // Renamed field

	CurrentHost   connector.Host
	ControlNode   connector.Host
	ClusterArtifactsDir string // Added field
}

type HostRuntime struct {
	Host  connector.Host
	Conn  connector.Connector
	Facts *runner.Facts
}

func NewContextWithGoContext(gCtx context.Context, parent *Context) *Context {
	if parent == nil {
		panic("parent context cannot be nil in NewContextWithGoContext")
	}
	newCtx := *parent
	newCtx.GoCtx = gCtx
	newCtx.CurrentHost = parent.CurrentHost
	return &newCtx
}

func (c *Context) ForHost(host connector.Host) step.StepContext {
	newCtx := *c
	newCtx.CurrentHost = host
	return &newCtx
}

func (c *Context) GetHost() connector.Host {
	return c.CurrentHost
}

func (c *Context) GoContext() context.Context {
	return c.GoCtx
}

func (c *Context) GetLogger() *logger.Logger {
	return c.Logger
}

func (c *Context) GetClusterConfig() *v1alpha1.Cluster {
	return c.ClusterConfig
}

func (c *Context) GetRunner() runner.Runner {
	return c.Runner
}

func (c *Context) GetEngine() engine.Engine {
	return c.Engine
}

func (c *Context) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized in Context")
	}
	for _, hr := range c.HostRuntimes {
		hostRoles := hr.Host.GetRoles()
		for _, r := range hostRoles {
			if r == role {
				hosts = append(hosts, hr.Host)
				break
			}
		}
	}
	return hosts, nil
}

func (c *Context) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s", host.GetName())
	}
	if hr.Facts == nil {
		return nil, fmt.Errorf("no facts gathered or available for host: %s", host.GetName())
	}
	return hr.Facts, nil
}

func (c *Context) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s", host.GetName())
	}
	if hr.Conn == nil {
		return nil, fmt.Errorf("no active connector found or available for host: %s", host.GetName())
	}
	return hr.Conn, nil
}

func (c *Context) GetCurrentHostFacts() (*runner.Facts, error) {
	if c.CurrentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostFacts")
	}
	return c.GetHostFacts(c.CurrentHost)
}

func (c *Context) GetCurrentHostConnector() (connector.Connector, error) {
	if c.CurrentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostConnector")
	}
	return c.GetConnectorForHost(c.CurrentHost)
}

func (c *Context) GetControlNode() (connector.Host, error) {
	if c.ControlNode == nil {
		return nil, fmt.Errorf("control node has not been initialized in runtime context")
	}
	return c.ControlNode, nil
}

func (c *Context) GetGlobalWorkDir() string {
	return c.GlobalWorkDir
}

func (c *Context) IsVerbose() bool {
	return c.GlobalVerbose
}

func (c *Context) ShouldIgnoreErr() bool {
	return c.GlobalIgnoreErr
}

func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

func (c *Context) PipelineCache() cache.PipelineCache {
	return c.internalPipelineCache // Use renamed field
}

func (c *Context) StepCache() cache.StepCache {
	return c.internalStepCache // Use renamed field
}

func (c *Context) TaskCache() cache.TaskCache {
	return c.internalTaskCache // Use renamed field
}

func (c *Context) ModuleCache() cache.ModuleCache {
	return c.internalModuleCache // Use renamed field
}

func (c *Context) AsPipelineContext() (pipeline.PipelineContext, bool) {
	return c, true
}

func (c *Context) AsModuleContext() (module.ModuleContext, bool) {
	return c, true
}

func (c *Context) AsTaskContext() (task.TaskContext, bool) {
	return c, true
}

func (c *Context) NewStepContext() step.StepContext {
	return c
}

var _ pipeline.PipelineContext = (*Context)(nil)
var _ module.ModuleContext = (*Context)(nil)
var _ task.TaskContext = (*Context)(nil)
var _ step.StepContext = (*Context)(nil)
var _ engine.EngineExecuteContext = (*Context)(nil)

func (c *Context) WithGoContext(goCtx context.Context) step.StepContext {
	newCtx := *c
	newCtx.GoCtx = goCtx
	return &newCtx
}

func (c *Context) GetClusterArtifactsDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		// Fallback or error if cluster name isn't set, as it's part of the path.
		// For now, returning a path relative to GlobalWorkDir if ClusterConfig is missing.
		// This might indicate an issue if called before ClusterConfig is fully populated.
		// Consider logging a warning here.
		// A more robust solution might be to ensure ClusterArtifactsDir is set during Context creation
		// once ClusterConfig.Name is known.
		// For now, if ClusterArtifactsDir was not explicitly set (e.g. by builder based on cluster name),
		// this might return GlobalWorkDir + "/<unknown_cluster_name>/..."
		// The field c.ClusterArtifactsDir should be the source of truth if populated.
		if c.ClusterArtifactsDir == "" && c.GlobalWorkDir != "" && c.ClusterConfig != nil && c.ClusterConfig.Name != "" {
		    // If not pre-set, construct it. This logic was in builder before.
		    // This should ideally be set once in the builder.
		    // For safety, if called and it's empty, try to construct it if possible.
		    return filepath.Join(c.GlobalWorkDir, ".kubexm", c.ClusterConfig.Name)
		}
		return c.ClusterArtifactsDir // Return pre-set value or empty if not set
	}
	// If c.ClusterArtifactsDir is already populated by the builder, use it.
	// Otherwise, construct it.
	if c.ClusterArtifactsDir == "" {
		 return filepath.Join(c.GlobalWorkDir, ".kubexm", c.ClusterConfig.Name)
	}
	return c.ClusterArtifactsDir
}

func (c *Context) GetCertsDir() string {
	return filepath.Join(c.GetClusterArtifactsDir(), common.DefaultCertsDir)
}

func (c *Context) GetEtcdCertsDir() string {
	return filepath.Join(c.GetCertsDir(), common.DefaultEtcdDir)
}

func (c *Context) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), componentName)
}

func (c *Context) GetEtcdArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultEtcdDir)
}

func (c *Context) GetContainerRuntimeArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultContainerRuntimeDir)
}

func (c *Context) GetKubernetesArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultKubernetesDir)
}

func (c *Context) GetFileDownloadPath(componentName, version, arch, filename string) string {
	var typeDir string
	// Normalize componentName for matching if needed, though current common constants are lowercase.
	// cnLower := strings.ToLower(componentName)

	// Determine the top-level type directory based on the component name.
	// This mapping should align with how directories are created in RuntimeBuilder
	// and how components are categorized.
	switch componentName {
	case common.DefaultEtcdDir: // "etcd"
		typeDir = common.DefaultEtcdDir
	case "containerd", "docker", "runc", "cri-dockerd", "cni-plugins": // Group common container runtime related tools
		typeDir = common.DefaultContainerRuntimeDir // "container_runtime"
	case "kubeadm", "kubelet", "kubectl", "kube-proxy", "kube-scheduler", "kube-controller-manager", "kube-apiserver", "helm", "crictl":
		typeDir = common.DefaultKubernetesDir // "kubernetes"
	// Add cases for other specific components if they have dedicated type directories
	// e.g., registry, harbor might go into a "registry_tools" or similar typeDir
	default:
		// Fallback for components not explicitly categorized.
		// This could mean they are placed directly under ClusterArtifactsDir/componentName
		// or a generic "binaries" directory.
		// For now, consistent with the explicit paths in 21-其他说明.md,
		// we expect components to map to one of the main type dirs.
		// If a component doesn't fit, this indicates a potential need to update this mapping
		// or the directory structure itself.
		c.Logger.Warnf("Component '%s' does not have a predefined type directory mapping for downloads. Placing under its own name.", componentName)
		typeDir = componentName // Or perhaps a common.DefaultBinariesDir
	}

	// Construct the path: GlobalWorkDir/.kubexm/cluster_name/TYPE_DIR/COMPONENT_NAME/VERSION/ARCH/FILENAME
	// This structure ensures that even if multiple components fall under "kubernetes" type_dir,
	// they each get their own subfolder (e.g., kubernetes/kubelet/v1.23.5/amd64/kubelet)

	pathSegments := []string{c.GetClusterArtifactsDir()} // GlobalWorkDir/.kubexm/cluster_name
	pathSegments = append(pathSegments, typeDir)         // TYPE_DIR
	pathSegments = append(pathSegments, componentName)   // ACTUAL_COMPONENT_NAME
	if version != "" {
		pathSegments = append(pathSegments, version)
	}
	if arch != "" {
		pathSegments = append(pathSegments, arch)
	}
	if filename != "" { // filename can be empty if we just want the directory up to arch
		pathSegments = append(pathSegments, filename)
	}
	return filepath.Join(pathSegments...)
}

func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GlobalWorkDir, hostname)
}
