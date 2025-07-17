package runtime

import (
	"github.com/mensylisir/kubexm/pkg/engine"
)

type OrchestrationContext interface {
	CoreServiceContext
	ClusterQueryContext
	FileSystemContext
	CacheProviderContext
	GlobalSettingsContext
}

type PipelineContext interface {
	OrchestrationContext
	GetEngine() engine.Engine
}

type ModuleContext interface {
	OrchestrationContext
}

type TaskContext interface {
	OrchestrationContext
}
