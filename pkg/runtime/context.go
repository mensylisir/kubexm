package runtime

import (
	"context"
	"fmt"
	"context"
	"fmt"
	"time" // Added for time.Duration

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Corrected path
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// PipelineContext defines the methods available at the pipeline execution level.
type PipelineContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	PipelineCache() cache.PipelineCache // Assuming cache.PipelineCache exists
	GetGlobalWorkDir() string
}

// ModuleContext defines the methods available at the module execution level.
type ModuleContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	ModuleCache() cache.ModuleCache // Assuming cache.ModuleCache exists
	GetGlobalWorkDir() string
}

// TaskContext defines the methods available at the task execution level.
type TaskContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	TaskCache() cache.TaskCache // Assuming cache.TaskCache exists
	ModuleCache() cache.ModuleCache // Tasks can access parent module's cache
	GetGlobalWorkDir() string
}

// StepContext defines the methods available at the step execution level.
type StepContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetRunner() runner.Runner
	GetHost() connector.Host // Current host for the step
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	GetConnectorForHost(host connector.Host) (connector.Connector, error)
	StepCache() cache.StepCache     // Assuming cache.StepCache exists
	TaskCache() cache.TaskCache     // Steps can access parent task's cache
	ModuleCache() cache.ModuleCache   // Steps can access parent module's cache
	GetGlobalWorkDir() string
}

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
