package runtime

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"net/http"
	"time"
)

type ExecutionContext interface {
	CoreServiceContext
	ClusterQueryContext
	FileSystemContext
	GlobalSettingsContext
	GetCurrentHostConnector() (connector.Connector, error)
	GetStepCache() cache.StepCache
	GetTaskCache() cache.TaskCache
	GetModuleCache() cache.ModuleCache
	GetPipelineCache() cache.PipelineCache
	GetHost() connector.Host
	GetStepExecutionID() string
	GetExecutionStartTime() time.Time
	WithGoContext(goCtx context.Context) ExecutionContext
	GetHttpClient() *http.Client
	GetFromRuntimeConfig(key string) (value interface{}, ok bool)
}
