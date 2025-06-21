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
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	// Facade interfaces are now defined in facade.go within the same package.
)

// Context is the global container for all runtime dependencies and state.
// It aims to provide the different context facades (PipelineContext, ModuleContext, etc.)
// by implementing the interfaces defined in facade.go.
type Context struct {
	GoCtx         context.Context // Renamed from GoContext to avoid conflict with method name
	Logger        *logger.Logger
	Engine        engine.Engine
	Runner        runner.Runner
	ClusterConfig *v1alpha1.Cluster
	HostRuntimes  map[string]*HostRuntime // key: host.GetName()
	ConnectionPool *connector.ConnectionPool

	// Global configurations
	GlobalWorkDir string
	GlobalVerbose bool
	GlobalIgnoreErr bool
	GlobalConnectionTimeout time.Duration

	// Caches - Assuming specific cache types like cache.PipelineCache exist
	PipelineCache cache.PipelineCache
	ModuleCache   cache.ModuleCache
	TaskCache     cache.TaskCache
	StepCache     cache.StepCache

	// CurrentHost stores the specific host this context is currently operating on.
	// This is particularly relevant for StepContext.
	CurrentHost   connector.Host
	ControlNode   connector.Host // Represents the control node (where kubexm is running)
}

// HostRuntime encapsulates all runtime information for a single host.
type HostRuntime struct {
	Host  connector.Host    // This should be the connector.Host interface type
	Conn  connector.Connector
	Facts *runner.Facts
}

// NewContextWithGoContext is a helper to create a new context with a different Go context,
// for passing down cancellation signals from errgroup or other scoped operations.
// It performs a shallow copy of the parent context and replaces the GoCtx.
func NewContextWithGoContext(gCtx context.Context, parent *Context) *Context {
	if parent == nil {
		// Or handle this case more gracefully depending on requirements
		panic("parent context cannot be nil in NewContextWithGoContext")
	}
	newCtx := *parent // Shallow copy
	newCtx.GoCtx = gCtx
	newCtx.CurrentHost = parent.CurrentHost // Also copy CurrentHost
	return &newCtx
}

// ForHost creates a new context specifically for operations on the given host.
// It performs a shallow copy of the parent context and sets the CurrentHost.
func (c *Context) ForHost(host connector.Host) *Context {
	newCtx := *c // Shallow copy
	newCtx.CurrentHost = host
	// Potentially, the logger could also be updated here to include host-specific fields.
	// For example: newCtx.Logger = c.Logger.With("host", host.GetName())
	// However, this depends on whether the logger in StepContext should be automatically host-scoped
	// or if step implementations are responsible for using a host-specific logger.
	// For now, just copying the logger as is.
	return &newCtx
}

// --- Interface Implementations / Getters for Facades ---

// GetHost returns the current host associated with this context.
// This is required to satisfy the StepContext interface.
func (c *Context) GetHost() connector.Host {
	return c.CurrentHost
}

// GoContext returns the underlying Go context.
func (c *Context) GoContext() context.Context {
	return c.GoCtx
}

// GetLogger returns the logger instance.
func (c *Context) GetLogger() *logger.Logger {
	return c.Logger
}

// GetClusterConfig returns the cluster configuration.
func (c *Context) GetClusterConfig() *v1alpha1.Cluster {
	return c.ClusterConfig
}

// GetRunner returns the runner instance.
func (c *Context) GetRunner() runner.Runner {
	return c.Runner
}

// GetEngine returns the engine instance.
func (c *Context) GetEngine() engine.Engine {
	return c.Engine
}

// GetHostsByRole returns hosts matching a given role.
// Note: This requires knowledge of how roles are defined on connector.Host or v1alpha1.HostSpec.
// The issue implies HostSpec has a Roles []string field.
func (c *Context) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized in Context")
	}
	for _, hr := range c.HostRuntimes {
		// Assuming hr.Host is a connector.Host which has a GetRoles() method,
		// or we access the original HostSpec if hr.Host stores it.
		// For this example, let's assume connector.Host has a Roles() []string method or similar.
		// This part is a bit underspecified in the original prompt for connector.Host.
		// Let's assume we can access HostSpec from connector.Host or HostRuntime.Host

		// A common pattern is that connector.Host is an interface, and one of its
		// implementations might hold the original HostSpec.
		// For simplicity, let's assume HostRuntime.Host has a way to get roles.
		// This might involve a type assertion if connector.Host is too generic.

		// Given the problem describes `HostSpec` having `Roles []string`,
		// and `HostRuntime.Host` being `connector.Host`, we need a way to get roles.
		// Let's assume `connector.Host` has a method `GetRoles() []string`.
		// If not, the `connector.HostFromSpec` (from builder) needs to expose this.

		hostRoles := hr.Host.GetRoles() // Hypothetical method on connector.Host
		for _, r := range hostRoles {
			if r == role {
				hosts = append(hosts, hr.Host)
				break
			}
		}
	}
	if len(hosts) == 0 {
		// It's not necessarily an error to find no hosts for a role.
		// Consider if an error should be returned or just an empty slice.
		// The interface now includes error, so we can use it if needed.
		// For now, returning no error if simply no hosts match.
		// return nil, fmt.Errorf("no hosts found with role: %s", role)
	}
	return hosts, nil
}

// GetHostFacts returns the gathered facts for a specific host.
func (c *Context) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()] // Assuming GetName() gives the map key
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s (not in HostRuntimes map)", host.GetName())
	}
	if hr.Facts == nil {
		return nil, fmt.Errorf("no facts gathered or available for host: %s", host.GetName())
	}
	return hr.Facts, nil
}

// GetConnectorForHost returns the active connector for a specific host.
func (c *Context) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s (not in HostRuntimes map)", host.GetName())
	}
	if hr.Conn == nil {
		return nil, fmt.Errorf("no active connector found or available for host: %s", host.GetName())
	}
	return hr.Conn, nil
}

// GetCurrentHostFacts returns facts for the host currently associated with this StepContext.
func (c *Context) GetCurrentHostFacts() (*runner.Facts, error) {
	if c.CurrentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostFacts")
	}
	return c.GetHostFacts(c.CurrentHost)
}

// GetCurrentHostConnector returns a connector for the host currently associated with this StepContext.
func (c *Context) GetCurrentHostConnector() (connector.Connector, error) {
	if c.CurrentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostConnector")
	}
	return c.GetConnectorForHost(c.CurrentHost)
}

// GetControlNode returns the special Host object representing the control node.
func (c *Context) GetControlNode() (connector.Host, error) {
	if c.ControlNode == nil {
		return nil, fmt.Errorf("control node has not been initialized in runtime context")
	}
	return c.ControlNode, nil
}


// --- Global Configuration Getters ---

// GetGlobalWorkDir returns the global working directory.
func (c *Context) GetGlobalWorkDir() string {
	return c.GlobalWorkDir
}

// IsVerbose returns true if verbose mode is enabled.
func (c *Context) IsVerbose() bool {
	return c.GlobalVerbose
}

// ShouldIgnoreErr returns true if errors should be ignored.
func (c *Context) ShouldIgnoreErr() bool {
	return c.GlobalIgnoreErr
}

// GetGlobalConnectionTimeout returns the global connection timeout.
func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

// PipelineCache returns the pipeline cache.
func (c *Context) PipelineCache() cache.PipelineCache {
	return c.PipelineCache // Assumes c.PipelineCache is already of type cache.PipelineCache
}

// StepCache returns the step cache.
// This is required to satisfy the StepContext interface.
func (c *Context) StepCache() cache.StepCache {
	return c.StepCache // Assumes c.StepCache is already of type cache.StepCache
}

// TaskCache returns the task cache.
// This is required to satisfy the StepContext interface (if it needs TaskCache access).
func (c *Context) TaskCache() cache.TaskCache {
	return c.TaskCache // Assumes c.TaskCache is already of type cache.TaskCache
}

// ModuleCache returns the module cache.
// This is required to satisfy the StepContext interface (if it needs ModuleCache access).
func (c *Context) ModuleCache() cache.ModuleCache {
	return c.ModuleCache // Assumes c.ModuleCache is already of type cache.ModuleCache
}

// --- Facade Provider Methods ---

// AsPipelineContext returns the context as a PipelineContext.
// Since *Context implements all methods of PipelineContext, this is a direct cast.
func (c *Context) AsPipelineContext() (PipelineContext, bool) {
	return c, true
}

// AsModuleContext returns the context as a ModuleContext.
func (c *Context) AsModuleContext() (ModuleContext, bool) {
	return c, true
}

// AsTaskContext returns the context as a TaskContext.
func (c *Context) AsTaskContext() (TaskContext, bool) {
	return c, true
}

// NewStepContext creates a new StepContext from the main context.
// In the issue's design, the main context itself implements the StepContext interface.
func (c *Context) NewStepContext() StepContext {
	// This implies that *Context itself will have all methods of StepContext.
	return c
}

// Ensure *Context satisfies the facade interfaces.
var _ PipelineContext = (*Context)(nil)
var _ ModuleContext = (*Context)(nil)
var _ TaskContext = (*Context)(nil)
var _ StepContext = (*Context)(nil)
var _ engine.EngineExecuteContext = (*Context)(nil) // Ensure Context implements EngineExecuteContext

// WithGoContext returns a new Context (which is a StepContext) using the provided Go context.
// This is primarily used by the engine to propagate cancellation from errgroup contexts to steps.
func (c *Context) WithGoContext(goCtx context.Context) StepContext {
	newCtx := *c // Shallow copy
	newCtx.GoCtx = goCtx
	// CurrentHost should already be set correctly if this c is from a ForHost() call
	return &newCtx
}


// --- Artifact Path Helper Methods ---

// GetClusterArtifactsDir returns the base directory for all cluster-specific artifacts.
// Path: ${GlobalWorkDir}/${cluster_name}
func (c *Context) GetClusterArtifactsDir() string {
	return c.ClusterArtifactsDir
}

// GetCertsDir returns the base directory for all certificates.
// Path: ${GlobalWorkDir}/${cluster_name}/certs
func (c *Context) GetCertsDir() string {
	return filepath.Join(c.ClusterArtifactsDir, common.DefaultCertsDir)
}

// GetEtcdCertsDir returns the directory for ETCD certificates.
// Path: ${GlobalWorkDir}/${cluster_name}/certs/etcd
func (c *Context) GetEtcdCertsDir() string {
	return filepath.Join(c.GetCertsDir(), common.DefaultEtcdDir)
}

// GetComponentArtifactsDir returns the base directory for a specific component's artifacts.
// Path: ${GlobalWorkDir}/${cluster_name}/${componentName}
func (c *Context) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(c.ClusterArtifactsDir, componentName)
}

// GetEtcdArtifactsDir returns the base directory for ETCD artifacts (binaries, etc.).
// Path: ${GlobalWorkDir}/${cluster_name}/etcd
func (c *Context) GetEtcdArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultEtcdDir)
}

// GetContainerRuntimeArtifactsDir returns the base directory for container runtime artifacts.
// Path: ${GlobalWorkDir}/${cluster_name}/container_runtime
func (c *Context) GetContainerRuntimeArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultContainerRuntimeDir)
}

// GetKubernetesArtifactsDir returns the base directory for Kubernetes component artifacts.
// Path: ${GlobalWorkDir}/${cluster_name}/kubernetes
func (c *Context) GetKubernetesArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultKubernetesDir)
}

// GetFileDownloadPath returns the standardized local path for a downloaded file,
// typically structured as:
// ${GlobalWorkDir}/${cluster_name}/${componentName}/${version}/${arch}/${filename}
// If version or arch are empty, they are omitted from the path.
func (c *Context) GetFileDownloadPath(componentName, version, arch, filename string) string {
	baseDir := c.GetComponentArtifactsDir(componentName)
	pathParts := []string{baseDir}
	if version != "" {
		pathParts = append(pathParts, version)
	}
	if arch != "" {
		pathParts = append(pathParts, arch)
	}
	pathParts = append(pathParts, filename)
	return filepath.Join(pathParts...)
}

// GetHostDir returns the host-specific working directory on the local machine.
// Path: ${GlobalWorkDir}/${hostname}
// This is for local storage related to a specific host, not necessarily cluster artifacts.
func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GlobalWorkDir, hostname)
}
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Corrected path - already imported above
)

// Context is the global container for all runtime dependencies and state.
// It aims to provide the different context facades (PipelineContext, ModuleContext, etc.).
type Context struct {
	GoCtx         context.Context // Renamed from GoContext to avoid conflict with method name
	Logger        *logger.Logger
	Engine        engine.Engine
	Runner        runner.Runner
	ClusterConfig *v1alpha1.Cluster
	HostRuntimes  map[string]*HostRuntime // key: host.GetName()
	ConnectionPool *connector.ConnectionPool

	// Global configurations
	GlobalWorkDir string
	GlobalVerbose bool
	GlobalIgnoreErr bool
	GlobalConnectionTimeout time.Duration

	// Caches - Assuming specific cache types like cache.PipelineCache exist
	PipelineCache cache.PipelineCache
	ModuleCache   cache.ModuleCache
	TaskCache     cache.TaskCache
	StepCache     cache.StepCache

	// CurrentHost stores the specific host this context is currently operating on.
	// This is particularly relevant for StepContext.
	CurrentHost   connector.Host
	ControlNode   connector.Host // Represents the control node (where kubexm is running)

	// ClusterArtifactsDir stores the base path for all artifacts related to the current cluster
	// e.g., $(pwd)/.kubexm/${cluster_name}
	ClusterArtifactsDir string
}

// HostRuntime encapsulates all runtime information for a single host.
type HostRuntime struct {
	Host  connector.Host    // This should be the connector.Host interface type
	Conn  connector.Connector
	Facts *runner.Facts
}

// NewContextWithGoContext is a helper to create a new context with a different Go context,
// for passing down cancellation signals from errgroup or other scoped operations.
// It performs a shallow copy of the parent context and replaces the GoCtx.
func NewContextWithGoContext(gCtx context.Context, parent *Context) *Context {
	if parent == nil {
		// Or handle this case more gracefully depending on requirements
		panic("parent context cannot be nil in NewContextWithGoContext")
	}
	newCtx := *parent // Shallow copy
	newCtx.GoCtx = gCtx
	newCtx.CurrentHost = parent.CurrentHost // Also copy CurrentHost
	return &newCtx
}

// ForHost creates a new context specifically for operations on the given host.
// It performs a shallow copy of the parent context and sets the CurrentHost.
func (c *Context) ForHost(host connector.Host) *Context {
	newCtx := *c // Shallow copy
	newCtx.CurrentHost = host
	// Potentially, the logger could also be updated here to include host-specific fields.
	// For example: newCtx.Logger = c.Logger.With("host", host.GetName())
	// However, this depends on whether the logger in StepContext should be automatically host-scoped
	// or if step implementations are responsible for using a host-specific logger.
	// For now, just copying the logger as is.
	return &newCtx
}

// --- Interface Implementations / Getters for Facades ---

// GetHost returns the current host associated with this context.
// This is required to satisfy the StepContext interface.
func (c *Context) GetHost() connector.Host {
	return c.CurrentHost
}

// GoContext returns the underlying Go context.
func (c *Context) GoContext() context.Context {
	return c.GoCtx
}

// GetLogger returns the logger instance.
func (c *Context) GetLogger() *logger.Logger {
	return c.Logger
}

// GetClusterConfig returns the cluster configuration.
func (c *Context) GetClusterConfig() *v1alpha1.Cluster {
	return c.ClusterConfig
}

// GetRunner returns the runner instance.
func (c *Context) GetRunner() runner.Runner {
	return c.Runner
}

// GetHostsByRole returns hosts matching a given role.
// Note: This requires knowledge of how roles are defined on connector.Host or v1alpha1.HostSpec.
// The issue implies HostSpec has a Roles []string field.
func (c *Context) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized in Context")
	}
	for _, hr := range c.HostRuntimes {
		// Assuming hr.Host is a connector.Host which has a GetRoles() method,
		// or we access the original HostSpec if hr.Host stores it.
		// For this example, let's assume connector.Host has a Roles() []string method or similar.
		// This part is a bit underspecified in the original prompt for connector.Host.
		// Let's assume we can access HostSpec from connector.Host or HostRuntime.Host

		// A common pattern is that connector.Host is an interface, and one of its
		// implementations might hold the original HostSpec.
		// For simplicity, let's assume HostRuntime.Host has a way to get roles.
		// This might involve a type assertion if connector.Host is too generic.

		// Given the problem describes `HostSpec` having `Roles []string`,
		// and `HostRuntime.Host` being `connector.Host`, we need a way to get roles.
		// Let's assume `connector.Host` has a method `GetRoles() []string`.
		// If not, the `connector.HostFromSpec` (from builder) needs to expose this.

		hostRoles := hr.Host.GetRoles() // Hypothetical method on connector.Host
		for _, r := range hostRoles {
			if r == role {
				hosts = append(hosts, hr.Host)
				break
			}
		}
	}
	if len(hosts) == 0 {
		// It's not necessarily an error to find no hosts for a role.
		// Consider if an error should be returned or just an empty slice.
		// The interface now includes error, so we can use it if needed.
		// For now, returning no error if simply no hosts match.
		// return nil, fmt.Errorf("no hosts found with role: %s", role)
	}
	return hosts, nil
}

// GetHostFacts returns the gathered facts for a specific host.
func (c *Context) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()] // Assuming GetName() gives the map key
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s (not in HostRuntimes map)", host.GetName())
	}
	if hr.Facts == nil {
		return nil, fmt.Errorf("no facts gathered or available for host: %s", host.GetName())
	}
	return hr.Facts, nil
}

// GetConnectorForHost returns the active connector for a specific host.
func (c *Context) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	if c.HostRuntimes == nil {
		return nil, fmt.Errorf("HostRuntimes map is not initialized")
	}
	hr, ok := c.HostRuntimes[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s (not in HostRuntimes map)", host.GetName())
	}
	if hr.Conn == nil {
		return nil, fmt.Errorf("no active connector found or available for host: %s", host.GetName())
	}
	return hr.Conn, nil
}

// --- Global Configuration Getters ---

// GetGlobalWorkDir returns the global working directory.
func (c *Context) GetGlobalWorkDir() string {
	return c.GlobalWorkDir
}

// IsVerbose returns true if verbose mode is enabled.
func (c *Context) IsVerbose() bool {
	return c.GlobalVerbose
}

// ShouldIgnoreErr returns true if errors should be ignored.
func (c *Context) ShouldIgnoreErr() bool {
	return c.GlobalIgnoreErr
}

// GetGlobalConnectionTimeout returns the global connection timeout.
func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

// StepCache returns the step cache.
// This is required to satisfy the StepContext interface.
func (c *Context) StepCache() cache.StepCache {
	return c.StepCache // Assumes c.StepCache is already of type cache.StepCache
}

// TaskCache returns the task cache.
// This is required to satisfy the StepContext interface (if it needs TaskCache access).
func (c *Context) TaskCache() cache.TaskCache {
	return c.TaskCache // Assumes c.TaskCache is already of type cache.TaskCache
}

// ModuleCache returns the module cache.
// This is required to satisfy the StepContext interface (if it needs ModuleCache access).
func (c *Context) ModuleCache() cache.ModuleCache {
	return c.ModuleCache // Assumes c.ModuleCache is already of type cache.ModuleCache
}

// PipelineCache returns the pipeline cache.
func (c *Context) PipelineCache() cache.PipelineCache {
	return c.PipelineCache // Assumes c.PipelineCache is already of type cache.PipelineCache
}

// --- Facade Provider Methods ---

// AsPipelineContext returns the context as a PipelineContext.
// Since *Context implements all methods of PipelineContext, this is a direct cast.
func (c *Context) AsPipelineContext() (PipelineContext, bool) {
	return c, true
}

// AsModuleContext returns the context as a ModuleContext.
func (c *Context) AsModuleContext() (ModuleContext, bool) {
	return c, true
}

// AsTaskContext returns the context as a TaskContext.
func (c *Context) AsTaskContext() (TaskContext, bool) {
	return c, true
}

// NewStepContext creates a new StepContext from the main context.
// In the issue's design, the main context itself implements the StepContext interface.
func (c *Context) NewStepContext() StepContext {
	// This implies that *Context itself will have all methods of StepContext.
	return c
}

// Ensure *Context satisfies the facade interfaces.
// These lines were already present but ensure they match the interface names defined above.
var _ PipelineContext = (*Context)(nil)
var _ ModuleContext = (*Context)(nil)
var _ TaskContext = (*Context)(nil)
var _ StepContext = (*Context)(nil)
