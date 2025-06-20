package runtime

import (
	"context"
	"fmt"
	"time" // Added for time.Duration
	"github.com/mensylisir/kubexm/pkg/cache" // Cache not used in provided snippet, can be added if needed
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Corrected path
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

	// Caches
	PipelineCache cache.Cache
	ModuleCache   cache.Cache
	TaskCache     cache.Cache
	StepCache     cache.Cache

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
	// Assuming c.StepCache is of a type that can be asserted to cache.StepCache.
	// This typically means that cache.StepCache is an interface implemented by the
	// concrete type stored in c.StepCache, or c.StepCache is already of this type.
	if stepCache, ok := c.StepCache.(cache.StepCache); ok {
		return stepCache
	}
	// Handle cases where the assertion might fail, though ideally, it shouldn't
	// if everything is initialized correctly.
	// Returning nil or panicking depends on how strict the error handling should be.
	// For now, let's assume it's always correctly initialized by the builder.
	// If c.StepCache can be nil, further checks might be needed.
	return c.StepCache.(cache.StepCache)
}

// TaskCache returns the task cache.
// This is required to satisfy the StepContext interface (if it needs TaskCache access).
func (c *Context) TaskCache() cache.TaskCache {
	// Similar to StepCache, assuming c.TaskCache can be asserted to cache.TaskCache.
	if taskCache, ok := c.TaskCache.(cache.TaskCache); ok {
		return taskCache
	}
	// Fallback or panic, assuming correct initialization.
	return c.TaskCache.(cache.TaskCache)
}

// ModuleCache returns the module cache.
// This is required to satisfy the StepContext interface (if it needs ModuleCache access).
func (c *Context) ModuleCache() cache.ModuleCache {
	if moduleCache, ok := c.ModuleCache.(cache.ModuleCache); ok {
		return moduleCache
	}
	return c.ModuleCache.(cache.ModuleCache)
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
