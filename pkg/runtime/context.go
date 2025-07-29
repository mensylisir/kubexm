package runtime

import (
	"context"
	"fmt"
	"net/http"
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

type Context struct {
	GoCtx    context.Context
	Logger   *logger.Logger
	Runner   runner.Runner
	Recorder record.EventRecorder
	//Engine        engine.Engine
	ClusterConfig *v1alpha1.Cluster

	GlobalWorkDir           string
	GlobalVerbose           bool
	GlobalIgnoreErr         bool
	GlobalConnectionTimeout time.Duration

	PipelineCache cache.PipelineCache
	ModuleCache   cache.ModuleCache
	TaskCache     cache.TaskCache
	StepCache     cache.StepCache

	hostInfoMap    map[string]*HostRuntimeInfo
	controlNode    connector.Host
	ConnectionPool *connector.ConnectionPool

	currentHost        connector.Host
	stepExecutionID    string
	executionStartTime time.Time

	httpClient               *http.Client
	currentStepRuntimeConfig map[string]interface{}
}

type HostRuntimeInfo struct {
	Host  connector.Host
	Conn  connector.Connector
	Facts *runner.Facts
}

var _ PipelineContext = (*Context)(nil)
var _ ModuleContext = (*Context)(nil)
var _ TaskContext = (*Context)(nil)
var _ ExecutionContext = (*Context)(nil)

func ForHost(rootCtx *Context, host connector.Host) ExecutionContext {
	newCtx := *rootCtx
	newCtx.currentHost = host
	newCtx.stepExecutionID = ""
	newCtx.executionStartTime = time.Time{}
	return &newCtx
}

func (c *Context) WithGoContext(goCtx context.Context) ExecutionContext {
	newCtx := *c
	newCtx.GoCtx = goCtx
	return &newCtx
}

func (c *Context) GetFromRuntimeConfig(key string) (interface{}, bool) {
	if c.currentStepRuntimeConfig == nil {
		return nil, false
	}
	val, ok := c.currentStepRuntimeConfig[key]
	return val, ok
}

func (c *Context) SetRuntimeConfig(config map[string]interface{}) *Context {
	newCtx := *c
	newCtx.currentStepRuntimeConfig = config
	return &newCtx
}

func (c *Context) GoContext() context.Context        { return c.GoCtx }
func (c *Context) GetLogger() *logger.Logger         { return c.Logger }
func (c *Context) GetRunner() runner.Runner          { return c.Runner }
func (c *Context) GetRecorder() record.EventRecorder { return c.Recorder }

// func (c *Context) GetEngine() engine.Engine            { return c.Engine }
func (c *Context) GetClusterConfig() *v1alpha1.Cluster { return c.ClusterConfig }

func (c *Context) GetPipelineCache() cache.PipelineCache { return c.PipelineCache }
func (c *Context) GetModuleCache() cache.ModuleCache     { return c.ModuleCache }
func (c *Context) GetTaskCache() cache.TaskCache         { return c.TaskCache }
func (c *Context) GetStepCache() cache.StepCache         { return c.StepCache }
func (c *Context) GetHttpClient() *http.Client           { return c.httpClient }

func (c *Context) GetHostsByRole(role string) []connector.Host {
	var hosts []connector.Host
	if c.hostInfoMap == nil {
		return nil
	}
	if role == "" {
		hosts := make([]connector.Host, 0, len(c.hostInfoMap))
		for _, hri := range c.hostInfoMap {
			hosts = append(hosts, hri.Host)
		}
		return hosts
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
		return nil, fmt.Errorf("host cannot be nil")
	}
	hri, ok := c.hostInfoMap[host.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime info for host: %s", host.GetName())
	}
	if hri.Facts == nil {
		return nil, fmt.Errorf("facts not available for host: %s", host.GetName())
	}
	return hri.Facts, nil
}

func (c *Context) GetControlNode() (connector.Host, error) {
	if c.controlNode == nil {
		return nil, fmt.Errorf("control node not initialized")
	}
	return c.controlNode, nil
}

func (c *Context) GetHost() connector.Host { return c.currentHost }

func (c *Context) GetCurrentHostFacts() (*runner.Facts, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context")
	}
	return c.GetHostFacts(c.currentHost)
}

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

func (c *Context) IsVerbose() bool       { return c.GlobalVerbose }
func (c *Context) ShouldIgnoreErr() bool { return c.GlobalIgnoreErr }
func (c *Context) GetGlobalConnectionTimeout() time.Duration {
	return c.GlobalConnectionTimeout
}

func (c *Context) GetGlobalWorkDir() string { return c.GlobalWorkDir }
func (c *Context) GetWorkspace() string     { return c.GetGlobalWorkDir() }

func (c *Context) GetClusterArtifactsDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		c.Logger.Error(nil, "ClusterConfig or name is not set, returning invalid artifacts dir")
		return filepath.Join(c.GlobalWorkDir, common.KUBEXM, "_INVALID_CLUSTER_")
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

func (c *Context) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	componentDir := filepath.Join(c.GetClusterArtifactsDir(), componentName, version, arch)
	if fileName != "" {
		return filepath.Join(componentDir, fileName)
	}
	return componentDir
}

func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GetClusterArtifactsDir(), hostname)
}

func (c *Context) GetCurrentHostConnector() (connector.Connector, error) {
	if c.currentHost == nil {
		return nil, fmt.Errorf("no current host set in context")
	}
	if c.hostInfoMap == nil {
		return nil, fmt.Errorf("hostInfoMap is not initialized")
	}
	hri, ok := c.hostInfoMap[c.currentHost.GetName()]
	if !ok {
		return nil, fmt.Errorf("no runtime info for current host: %s", c.currentHost.GetName())
	}
	if hri.Conn == nil {
		return nil, fmt.Errorf("connector not available for current host: %s", c.currentHost.GetName())
	}
	return hri.Conn, nil
}
