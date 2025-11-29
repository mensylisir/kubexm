package runtime

import (
	"context"
	"net/http"
	"time"

	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/types"
)

type ExecutionContext interface {
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
	GetGlobalState() StateBag
	GetPipelineState() StateBag
	GetModuleState() StateBag
	GetTaskState() StateBag
	SetStepResult(result *types.StepResult)
	GetStepResult() *types.StepResult
}
