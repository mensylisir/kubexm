package runtime

import (
	"context"
	"fmt"
	"path/filepath" // Added for artifact path helpers
	"time"          // Added for time.Duration

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/util" // For util.BinaryInfo
	"k8s.io/client-go/tools/record"         // Added for event.Recorder
)

// Context holds all runtime information, services, and configurations.
// It is designed to be passed down through the execution layers (Pipeline, Module, Task, Step).
type Context struct {
	GoCtx         context.Context
	Logger        *logger.Logger
	Engine        engine.Engine // DAG-aware engine
	Runner        runner.Runner
	Recorder      record.EventRecorder // Added event recorder
	ClusterConfig *v1alpha1.Cluster

	// Global configurations accessible throughout the runtime.
	GlobalWorkDir           string // Base work directory on the control machine (e.g., $(pwd)/.kubexm/${cluster_name})
	GlobalVerbose           bool
	GlobalIgnoreErr         bool
	GlobalConnectionTimeout time.Duration

	// Scoped caches, one instance per type for the entire pipeline execution.
	PipelineCache cache.PipelineCache
	ModuleCache   cache.ModuleCache
	TaskCache     cache.TaskCache
	StepCache     cache.StepCache

	// Information about all hosts in the cluster, populated by RuntimeBuilder.
	// This map is primarily for the builder to pass info to the Context.
	// Accessors like GetHostsByRole, GetHostFacts, GetConnectorForHost will use this.
	// For DAG, individual step/task contexts might get specific host info directly.
	hostInfoMap map[string]*HostRuntimeInfo // Key: host.GetName()

	// currentHost and controlNode are for specific contexts (e.g. StepContext)
	// currentHost will be set by the engine when dispatching a step to a host.
	currentHost    connector.Host
	controlNode    connector.Host            // Represents the machine running Kubexm CLI
	ConnectionPool *connector.ConnectionPool // Added connection pool
}

// HostRuntimeInfo holds connection and facts for a specific host.
// This is an internal structure primarily for the RuntimeBuilder and Context accessors.
type HostRuntimeInfo struct {
	Host  connector.Host      // The abstract Host object
	Conn  connector.Connector // The active connector to this host
	Facts *runner.Facts       // Gathered facts for this host
}

// NewContextWithGoContext creates a new Context instance with a new Go context,
// typically used by the engine to propagate cancellation or deadlines.
func NewContextWithGoContext(goCtx context.Context, parent *Context) *Context {
	if parent == nil {
		// This should not happen in normal operation.
		// If it does, it's a programming error.
		panic("parent context cannot be nil in NewContextWithGoContext")
	}
	// Create a shallow copy and then replace the GoCtx.
	// Other fields like caches and hostInfoMap are shared.
	newCtx := *parent
	newCtx.GoCtx = goCtx
	// currentHost is not typically inherited this way; it's set by the dispatcher.
	// newCtx.currentHost = parent.currentHost
	return &newCtx
}

// --- Context Implementation for Different Layers ---

// GoContext returns the underlying Go context.
func (c *Context) GoContext() context.Context { return c.GoCtx }

// GetLogger returns the logger instance.
func (c *Context) GetLogger() *logger.Logger { return c.Logger }

// GetClusterConfig returns the parsed cluster configuration.
func (c *Context) GetClusterConfig() *v1alpha1.Cluster { return c.ClusterConfig }

// GetRunner returns the runner service.
func (c *Context) GetRunner() runner.Runner { return c.Runner }

// GetEngine returns the execution engine.
func (c *Context) GetEngine() engine.Engine { return c.Engine }

// GetRecorder returns the event recorder.
func (c *Context) GetRecorder() record.EventRecorder { return c.Recorder }

// GetConnectionPool returns the connection pool.
func (c *Context) GetConnectionPool() *connector.ConnectionPool { return c.ConnectionPool }

// PipelineCache returns the pipeline-scoped cache.
func (c *Context) GetPipelineCache() cache.PipelineCache { return c.PipelineCache }

// ModuleCache returns the module-scoped cache.
func (c *Context) GetModuleCache() cache.ModuleCache { return c.ModuleCache }

// TaskCache returns the task-scoped cache.
func (c *Context) GetTaskCache() cache.TaskCache { return c.TaskCache }

// StepCache returns the step-scoped cache.
func (c *Context) GetStepCache() cache.StepCache { return c.StepCache }

// --- Host Information Accessors (used by various context levels) ---

// GetHostsByRole retrieves a list of abstract Host objects that have the specified role.
func (c *Context) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if c.hostInfoMap == nil {
		return nil, fmt.Errorf("hostInfoMap is not initialized in Context")
	}
	for _, hri := range c.hostInfoMap {
		for _, r := range hri.Host.GetRoles() {
			if r == role {
				hosts = append(hosts, hri.Host)
				break
			}
		}
	}
	if len(hosts) == 0 {
		// It's not necessarily an error if no hosts have a role, could be expected.
		// Depending on caller, this might be handled. For now, return empty list.
	}
	return hosts, nil
}

// GetHostFacts retrieves the gathered facts for a specific host.
func (c *Context) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if c.hostInfoMap == nil {
		return nil, fmt.Errorf("hostInfoMap is not initialized")
	}
	if host == nil {
		return nil, fmt.Errorf("host cannot be nil for GetHostFacts")
	}
	hri, ok := c.hostInfoMap[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s", host.GetName())
	}
	if hri.Facts == nil {
		// This could happen if facts gathering failed for this host during init.
		return nil, fmt.Errorf("facts not available for host: %s", host.GetName())
	}
	return hri.Facts, nil
}

// GetConnectorForHost retrieves the active connector for a specific host.
// This is crucial for the engine/steps to interact with hosts.
func (c *Context) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	if c.hostInfoMap == nil {
		return nil, fmt.Errorf("hostInfoMap is not initialized")
	}
	if host == nil {
		return nil, fmt.Errorf("host cannot be nil for GetConnectorForHost")
	}
	hri, ok := c.hostInfoMap[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime information found for host: %s", host.GetName())
	}
	if hri.Conn == nil {
		// This indicates connection might have failed during init or was closed.
		return nil, fmt.Errorf("connector not available for host: %s", host.GetName())
	}
	return hri.Conn, nil
}

// GetCurrentHost returns the host associated with the current step's execution context.
// This is primarily for step.StepContext.
func (c *Context) GetHost() connector.Host {
	if c.currentHost == nil {
		// This might happen if called outside a step execution context.
		// Consider if this should panic or return an error.
		// For now, returning nil and letting callers handle.
		c.Logger.Warn("GetCurrentHost called when no current host is set in context")
	}
	return c.currentHost
}

// GetCurrentHostFacts is a convenience method for step.StepContext.
func (c *Context) GetCurrentHostFacts() (*runner.Facts, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostFacts")
	}
	return c.GetHostFacts(c.currentHost)
}

// GetCurrentHostConnector is a convenience method for step.StepContext.
func (c *Context) GetCurrentHostConnector() (connector.Connector, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostConnector")
	}
	return c.GetConnectorForHost(c.currentHost)
}

// GetControlNode returns the special connector.Host representing the control machine.
func (c *Context) GetControlNode() (connector.Host, error) {
	if c.controlNode == nil {
		return nil, fmt.Errorf("control node has not been initialized in runtime context")
	}
	return c.controlNode, nil
}

// --- Global Configuration Accessors ---
func (c *Context) GetGlobalWorkDir() string { return c.GlobalWorkDir }
func (c *Context) IsVerbose() bool          { return c.GlobalVerbose }
func (c *Context) ShouldIgnoreErr() bool    { return c.GlobalIgnoreErr }
func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

// --- Artifact Path Helpers (for step.StepContext) ---

// GetClusterArtifactsDir returns the root directory for all artifacts related to this cluster
// on the control machine. e.g., /path/to/workdir/.kubexm/mycluster/
func (c *Context) GetClusterArtifactsDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		c.Logger.Error(nil, "ClusterConfig or ClusterConfig.Name is not set when trying to get cluster artifacts directory")
		// Return a non-nil but clearly invalid path to make errors more obvious downstream.
		return filepath.Join(c.GlobalWorkDir, common.KubeXMRootDir, "_INVALID_CLUSTER_NAME_")
	}
	// This path is typically GlobalWorkDir itself, as GlobalWorkDir is already cluster-specific.
	// The structure is $(pwd)/.kubexm/${cluster_name}
	return c.GlobalWorkDir
}

// GetCertsDir returns the path to the general certificates directory for the cluster.
// e.g., /path/to/workdir/.kubexm/mycluster/certs/
func (c *Context) GetCertsDir() string {
	return filepath.Join(c.GetClusterArtifactsDir(), common.DefaultCertsDir)
}

// GetEtcdCertsDir returns the path to the etcd-specific certificates directory.
// e.g., /path/to/workdir/.kubexm/mycluster/certs/etcd/
func (c *Context) GetEtcdCertsDir() string {
	return filepath.Join(c.GetCertsDir(), common.DefaultEtcdDir)
}

// GetComponentArtifactsDir returns the base directory for a specific component type's artifacts.
// e.g., for "etcd", it might return /path/to/workdir/.kubexm/mycluster/etcd/
func (c *Context) GetComponentArtifactsDir(componentTypeDir string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), componentTypeDir)
}

// GetEtcdArtifactsDir returns /path/to/workdir/.kubexm/mycluster/etcd/
func (c *Context) GetEtcdArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultEtcdDir)
}

// GetContainerRuntimeArtifactsDir returns /path/to/workdir/.kubexm/mycluster/container_runtime/
func (c *Context) GetContainerRuntimeArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultContainerRuntimeDir)
}

// GetKubernetesArtifactsDir returns /path/to/workdir/.kubexm/mycluster/kubernetes/
func (c *Context) GetKubernetesArtifactsDir() string {
	return c.GetComponentArtifactsDir(common.DefaultKubernetesDir)
}

// GetFileDownloadPath generates the full local path where a binary/artifact is expected to be stored.
// It uses util.GetBinaryInfo internally to determine the correct subdirectories.
// Example: /path/to/workdir/.kubexm/mycluster/etcd/v3.5.9/amd64/etcd-v3.5.9-linux-amd64.tar.gz
func (c *Context) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	// This method's logic is now largely encapsulated within util.GetBinaryInfo,
	// which constructs the FilePath based on workDir, clusterName, type, component, version, arch.
	// The workDir passed to GetBinaryInfo should be the *base* work directory (e.g., $(pwd)),
	// not the cluster-specific GlobalWorkDir from the context, as GetBinaryInfo itself appends /.kubexm/${clusterName}.

	// Determine the root work directory (e.g. current working directory of the CLI)
	// GlobalWorkDir is $(pwd)/.kubexm/${cluster_name}. We need the part before /.kubexm.
	pwdSuperDir := filepath.Dir(filepath.Dir(c.GlobalWorkDir)) // $(pwd)

	// Get the BinaryInfo which contains the pre-calculated FilePath
	// Zone can be obtained from util.GetZone() or passed if available in context.
	// For now, using util.GetZone() as it's a global setting.
	binInfo, err := util.GetBinaryInfo(componentName, version, arch, util.GetZone(), pwdSuperDir, c.ClusterConfig.Name)
	if err != nil {
		c.Logger.Errorf(err, "Failed to get binary info for path construction",
			"component", componentName, "version", version, "arch", arch, "fileName", fileName)
		// Return a best-effort path or an empty string to indicate failure
		// Constructing a path that's clearly an error might be better than empty.
		return filepath.Join(c.GetClusterArtifactsDir(), "ERROR_GETTING_PATH", componentName, version, arch, fileName)
	}
	// If a specific fileName is provided, ensure it matches what GetBinaryInfo determined.
	// This function is more about getting the *directory* for a component-version-arch,
	// or the full path if fileName is also given.
	if fileName != "" && binInfo.FileName != fileName {
		c.Logger.Warnf("Provided fileName '%s' does not match expected fileName '%s' from GetBinaryInfo for component '%s'. Using expected.", fileName, binInfo.FileName, componentName)
	}
	return binInfo.FilePath
}

// GetHostDir returns the local working directory specific to a host on the control machine.
// e.g., /path/to/workdir/.kubexm/mycluster/${hostname}/ (This was from old spec)
// New spec: $(pwd)/.kubexm/${hostname} - this seems to be a global host dir, not cluster specific.
// The plan for RuntimeBuilder indicates: GlobalWorkDir/${hostname} which is $(pwd)/.kubexm/${cluster_name}/${hostname}
// This seems more consistent. Let's use that.
func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), hostname)
}

// --- Context Interface Implementations ---
// These ensure *Context can be directly used where a more specific context interface is required.

var _ pipeline.PipelineContext = (*Context)(nil)
var _ module.ModuleContext = (*Context)(nil)
var _ task.TaskContext = (*Context)(nil)
var _ step.StepContext = (*Context)(nil)

// Note: engine.EngineExecuteContext might be the same as *Context or a subset.
// If it's just *Context, no explicit check needed beyond what Engine.Execute expects.

// WithGoContext for step.StepContext to allow engine to set per-step go context.
func (c *Context) WithGoContext(goCtx context.Context) step.StepContext {
	newCtx := *c // Create a copy
	newCtx.GoCtx = goCtx
	// currentHost for this new step context should be set by the dispatcher
	// It should not be inherited from the parent *Context directly here.
	// The engine, when creating a context for a step on a specific host, will set currentHost.
	return &newCtx
}

// SetCurrentHost is an internal method for the engine/dispatcher to set the
// context for a specific host when a step is being executed.
func (c *Context) SetCurrentHost(host connector.Host) *Context {
	c.currentHost = host
	return c // Return self for chaining if needed, though not typical for setters
}
