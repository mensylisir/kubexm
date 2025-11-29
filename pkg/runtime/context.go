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
	"github.com/mensylisir/kubexm/pkg/types"
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
	GlobalOfflineMode       bool

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

	httpClient *http.Client

	// StateBags for different scopes
	GlobalState   StateBag
	PipelineState StateBag
	ModuleState   StateBag
	TaskState     StateBag

	currentPipelineName string
	currentModuleName   string
	currentTaskName     string
	stepResult          *types.StepResult

	RunID string
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

func (c *Context) ForPipeline(pipelineName string) *Context {
	newCtx := *c
	newCtx.currentPipelineName = pipelineName
	if newCtx.PipelineState == nil {
		newCtx.PipelineState = NewStateBag()
	}
	return &newCtx
}

func (c *Context) ForModule(moduleName string) *Context {
	newCtx := *c
	newCtx.currentModuleName = moduleName
	if newCtx.ModuleState == nil {
		newCtx.ModuleState = NewStateBag()
	}
	return &newCtx
}

func (c *Context) ForTask(taskName string) *Context {
	newCtx := *c
	newCtx.currentTaskName = taskName
	if newCtx.TaskState == nil {
		newCtx.TaskState = NewStateBag()
	}
	return &newCtx
}

func (c *Context) WithGoContext(goCtx context.Context) ExecutionContext {
	newCtx := *c
	newCtx.GoCtx = goCtx
	return &newCtx
}

func (c *Context) GetFromRuntimeConfig(key string) (interface{}, bool) {
	if c.TaskState == nil {
		return nil, false
	}
	return c.TaskState.Get(key)
}

func (c *Context) SetRuntimeConfig(config map[string]interface{}) *Context {
	newCtx := *c
	if c.TaskState == nil {
		newCtx.TaskState = NewStateBag()
	}
	for k, v := range config {
		newCtx.TaskState.Set(k, v)
	}
	return &newCtx
}

// Data Bus Implementation

// Export exports a key-value pair to the specified scope.
// scope: "global", "pipeline", "module", "task"
func (c *Context) Export(scope string, key string, value interface{}) error {
	switch scope {
	case "global":
		if c.GlobalState == nil {
			return fmt.Errorf("global state not initialized")
		}
		c.GlobalState.Set(key, value)
	case "pipeline":
		if c.PipelineState == nil {
			return fmt.Errorf("pipeline state not initialized")
		}
		c.PipelineState.Set(key, value)
	case "module":
		if c.ModuleState == nil {
			return fmt.Errorf("module state not initialized")
		}
		c.ModuleState.Set(key, value)
	case "task":
		if c.TaskState == nil {
			return fmt.Errorf("task state not initialized")
		}
		c.TaskState.Set(key, value)
	default:
		return fmt.Errorf("unknown scope: %s", scope)
	}
	return nil
}

// Import imports a value from the specified scope.
// If scope is empty, it searches from Task -> Module -> Pipeline -> Global.
func (c *Context) Import(scope string, key string) (interface{}, bool) {
	if scope != "" {
		switch scope {
		case "global":
			if c.GlobalState != nil {
				return c.GlobalState.Get(key)
			}
		case "pipeline":
			if c.PipelineState != nil {
				return c.PipelineState.Get(key)
			}
		case "module":
			if c.ModuleState != nil {
				return c.ModuleState.Get(key)
			}
		case "task":
			if c.TaskState != nil {
				return c.TaskState.Get(key)
			}
		}
		return nil, false
	}

	// Search hierarchy
	if c.TaskState != nil {
		if v, ok := c.TaskState.Get(key); ok {
			return v, true
		}
	}
	if c.ModuleState != nil {
		if v, ok := c.ModuleState.Get(key); ok {
			return v, true
		}
	}
	if c.PipelineState != nil {
		if v, ok := c.PipelineState.Get(key); ok {
			return v, true
		}
	}
	if c.GlobalState != nil {
		if v, ok := c.GlobalState.Get(key); ok {
			return v, true
		}
	}
	return nil, false
}

func (c *Context) GetGlobalState() StateBag   { return c.GlobalState }
func (c *Context) GetPipelineState() StateBag { return c.PipelineState }
func (c *Context) GetModuleState() StateBag   { return c.ModuleState }
func (c *Context) GetTaskState() StateBag     { return c.TaskState }

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

func (c *Context) GetPipelineName() string {
	return c.currentPipelineName
}

func (c *Context) GetModuleName() string {
	return c.currentModuleName
}

func (c *Context) GetTaskName() string {
	return c.currentTaskName
}
func (c *Context) IsOfflineMode() bool {
	return c.GlobalOfflineMode
}

func (c *Context) GetRunID() string {
	return c.RunID
}

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

func (c *Context) GetClusterWorkDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		c.Logger.Error(nil, "ClusterConfig or name is not set, returning invalid artifacts dir")
		return filepath.Join(c.GlobalWorkDir, "_INVALID_CLUSTER_")
	}
	return filepath.Join(c.GlobalWorkDir, c.ClusterConfig.Name)
}

func (c *Context) GetHostWorkDir() string {
	if c.ClusterConfig == nil || c.ClusterConfig.Name == "" {
		c.Logger.Error(nil, "ClusterConfig or name is not set, returning invalid artifacts dir")
		return filepath.Join(c.GlobalWorkDir, "_INVALID_CLUSTER_")
	}
	return filepath.Join(c.GlobalWorkDir, c.ClusterConfig.Name, c.currentHost.GetName())
}

func (c *Context) GetExtractDir() string {
	return c.GetExtractDir()
}

func (c *Context) GetUploadDir() string {
	return c.GetUploadDir()
}

func (c *Context) GetKubernetesCertsDir() string {
	return filepath.Join(c.GetClusterWorkDir(), common.DefaultKubernetesDir, common.DefaultCertsDir)
}

func (c *Context) GetEtcdCertsDir() string {
	return filepath.Join(c.GetClusterWorkDir(), common.DefaultEtcdDir, common.DefaultCertsDir)
}

func (c *Context) GetHarborCertsDir() string {
	return filepath.Join(c.GetClusterWorkDir(), common.RegistryTypeHarbor, common.DefaultCertsDir)
}

func (c *Context) GetRegistryCertsDir() string {
	return filepath.Join(c.GetClusterWorkDir(), common.RegistryTypeRegistry, common.DefaultCertsDir)
}

func (c *Context) GetComponentArtifactsDir(componentTypeDir string) string {
	return filepath.Join(c.GetGlobalWorkDir(), componentTypeDir)
}

func (c *Context) GetRepositoryDir() string {
	return filepath.Join(c.GetGlobalWorkDir(), "repository")
}

func (c *Context) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	componentDir := filepath.Join(c.GetGlobalWorkDir(), componentName, version, arch)
	if fileName != "" {
		return filepath.Join(componentDir, fileName)
	}
	return componentDir
}

func (c *Context) GetHostDir(hostname string) string {
	return filepath.Join(c.GetClusterWorkDir(), hostname)
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

func (c *Context) SetStepResult(result *types.StepResult) {
	c.stepResult = result
}

func (c *Context) GetStepResult() *types.StepResult {
	return c.stepResult
}
