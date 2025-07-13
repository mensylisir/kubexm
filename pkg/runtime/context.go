package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"k8s.io/client-go/tools/record"
)

// Context holds all runtime information, services, and configurations.
// It implements various context interfaces but we don't import them to avoid circular dependencies.
type Context struct {
	GoCtx         context.Context
	Logger        *logger.Logger
	Runner        runner.Runner
	Recorder      record.EventRecorder
	ClusterConfig *v1alpha1.Cluster

	// Global configurations
	GlobalWorkDir           string
	GlobalVerbose           bool
	GlobalIgnoreErr         bool
	GlobalConnectionTimeout time.Duration

	// Caches
	PipelineCache cache.PipelineCache
	ModuleCache   cache.ModuleCache
	TaskCache     cache.TaskCache
	StepCache     cache.StepCache

	// Host information
	hostInfoMap    map[string]*HostRuntimeInfo
	currentHost    connector.Host
	controlNode    connector.Host
	ConnectionPool *connector.ConnectionPool

	// Step execution tracking
	stepExecutionID    string
	executionStartTime time.Time
}

// HostRuntimeInfo holds connection and facts for a specific host.
type HostRuntimeInfo struct {
	Host  connector.Host
	Conn  connector.Connector
	Facts *runner.Facts
}

// NewContextWithGoContext creates a new Context instance with a new Go context
func NewContextWithGoContext(goCtx context.Context, parent *Context) *Context {
	if parent == nil {
		panic("parent context cannot be nil in NewContextWithGoContext")
	}
	newCtx := *parent
	newCtx.GoCtx = goCtx
	return &newCtx
}

// Basic context methods
func (c *Context) GoContext() context.Context          { return c.GoCtx }
func (c *Context) GetLogger() *logger.Logger           { return c.Logger }
func (c *Context) GetClusterConfig() *v1alpha1.Cluster { return c.ClusterConfig }
func (c *Context) GetRunner() runner.Runner            { return c.Runner }
func (c *Context) GetRecorder() record.EventRecorder   { return c.Recorder }

// Cache accessors
func (c *Context) GetPipelineCache() cache.PipelineCache { return c.PipelineCache }
func (c *Context) GetModuleCache() cache.ModuleCache     { return c.ModuleCache }
func (c *Context) GetTaskCache() cache.TaskCache         { return c.TaskCache }
func (c *Context) GetStepCache() cache.StepCache         { return c.StepCache }

// Host information accessors
func (c *Context) GetHostsByRole(role string) []connector.Host {
	var hosts []connector.Host
	if c.hostInfoMap == nil {
		return nil
	}
	for _, hri := range c.hostInfoMap {
		for _, r := range hri.Host.GetRoles() {
			if r == role {
				hosts = append(hosts, hri.Host)
				break
			}
		}
	}
	return hosts
}

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
		return nil, fmt.Errorf("facts not available for host: %s", host.GetName())
	}
	return hri.Facts, nil
}

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
		return nil, fmt.Errorf("connector not available for host: %s", host.GetName())
	}
	return hri.Conn, nil
}

func (c *Context) GetHost() connector.Host {
	return c.currentHost
}

func (c *Context) GetCurrentHostFacts() (*runner.Facts, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostFacts")
	}
	return c.GetHostFacts(c.currentHost)
}

func (c *Context) GetCurrentHostConnector() (connector.Connector, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context for GetCurrentHostConnector")
	}
	return c.GetConnectorForHost(c.currentHost)
}

func (c *Context) GetControlNode() (connector.Host, error) {
	if c.controlNode == nil {
		return nil, fmt.Errorf("control node has not been initialized in runtime context")
	}
	return c.controlNode, nil
}

// Global configuration accessors
func (c *Context) GetGlobalWorkDir() string { return c.GlobalWorkDir }
func (c *Context) GetWorkspace() string     { return c.GlobalWorkDir }
func (c *Context) IsVerbose() bool          { return c.GlobalVerbose }
func (c *Context) ShouldIgnoreErr() bool    { return c.GlobalIgnoreErr }
func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

// Artifact path helpers
func (c *Context) GetClusterArtifactsDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		c.Logger.Error(nil, "ClusterConfig or ClusterConfig.Name is not set when trying to get cluster artifacts directory")
		return filepath.Join(c.GlobalWorkDir, common.KUBEXM, "_INVALID_CLUSTER_NAME_")
	}
	return c.GlobalWorkDir
}

func (c *Context) GetCertsDir() string {
	return filepath.Join(c.GetClusterArtifactsDir(), common.DefaultCertsDir)
}

func (c *Context) GetEtcdCertsDir() string {
	return filepath.Join(c.GetCertsDir(), common.DefaultEtcdDir)
}

func (c *Context) GetComponentArtifactsDir(componentTypeDir string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), componentTypeDir)
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

func (c *Context) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	// Simplified implementation for now
	componentDir := filepath.Join(c.GetClusterArtifactsDir(), componentName, version, arch)
	if fileName != "" {
		return filepath.Join(componentDir, fileName)
	}
	return componentDir
}

func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), hostname)
}

// Step execution tracking
func (c *Context) GetStepExecutionID() string {
	if c.stepExecutionID == "" {
		c.stepExecutionID = uuid.New().String()
	}
	return c.stepExecutionID
}

func (c *Context) GetExecutionStartTime() time.Time {
	if c.executionStartTime.IsZero() {
		c.executionStartTime = time.Now()
	}
	return c.executionStartTime
}

// Context creation methods for different layers - these will satisfy the interfaces
func (c *Context) WithGoContext(goCtx context.Context) *Context {
	newRuntimeCtx := *c
	newRuntimeCtx.GoCtx = goCtx
	return &newRuntimeCtx
}

func (c *Context) ForHost(host connector.Host) *Context {
	newRuntimeCtx := *c
	newRuntimeCtx.currentHost = host
	return &newRuntimeCtx
}

func (c *Context) SetCurrentHost(host connector.Host) *Context {
	c.currentHost = host
	return c
}
