package runtime

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
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
	GetHost() connector.Host
	GetStepExecutionID() string
	GetExecutionStartTime() time.Time
	WithGoContext(goCtx context.Context) ExecutionContext
}
