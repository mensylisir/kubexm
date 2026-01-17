package runtime

import (
	"context"
	"net/http"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/types"
)

type ExecutionContext interface {
	FileSystemContext
	GlobalSettingsContext
	CacheProviderContext
	CoreServiceContext
	ClusterQueryContext
	GetCurrentHostConnector() (connector.Connector, error)
	GetHost() connector.Host
	GetStepExecutionID() string
	GetExecutionStartTime() time.Time
	WithGoContext(goCtx context.Context) ExecutionContext
	GetHttpClient() *http.Client
	GetFromRuntimeConfig(key string) (value interface{}, ok bool)
	GetGlobalState() StateBag
	GetPipelineState() StateBag
	GetModuleState() StateBag
	GetTaskState() StateBag
	SetStepResult(result *types.StepResult)
	GetStepResult() *types.StepResult
	Export(scope string, key string, value interface{}) error
	Import(scope string, key string) (interface{}, bool)
}
