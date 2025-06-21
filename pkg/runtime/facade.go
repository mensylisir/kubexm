package runtime

import (
	"context"

	"github.com/mensylisir/kubexm/pkg/cache"     // Added for StepCache
	"github.com/mensylisir/kubexm/pkg/connector" // Updated import path
	"github.com/mensylisir/kubexm/pkg/logger"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runner"    // Updated import path
	// "github.com/mensylisir/kubexm/api/v1alpha1" // Assuming this path from issue, might need adjustment
	// For now, let's use a more generic config placeholder if api/v1alpha1 is not yet defined or used by this facade directly.
	// Instead of a direct v1alpha1.Cluster, let's use an interface or a more abstract config type if possible,
	// or defer its full usage until ClusterConfig type is confirmed.
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// PipelineContext defines the methods available at the pipeline execution level.
type PipelineContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	PipelineCache() cache.PipelineCache // From 13-runtime设计.md (via Context struct)
	GetGlobalWorkDir() string           // From 13-runtime设计.md (via Context struct)
}

// ModuleContext defines the methods available at the module execution level.
type ModuleContext interface {
	PipelineContext // Embed PipelineContext
	ModuleCache() cache.ModuleCache   // From 13-runtime设计.md (via Context struct)
}

// TaskContext defines the methods available at the task execution level.
type TaskContext interface {
	ModuleContext // Embed ModuleContext
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	TaskCache() cache.TaskCache     // From 13-runtime设计.md (via Context struct)
	// GetGlobalWorkDir(), GetClusterConfig(), GetLogger(), GoContext(), ModuleCache() are inherited.
}

// StepContext defines the methods available at the step execution level.
type StepContext interface {
	// Methods from 13-runtime设计.md's StepContext facade:
	GoContext() context.Context
	GetLogger() *logger.Logger
	// GetRecorder() event.Recorder // Recorder not yet implemented in main Context
	GetRunner() runner.Runner
	GetConnectorForHost(host connector.Host) (connector.Connector, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error) // Note: design doc had GetHostFacts(host connector.Host)

	// Additional useful methods often needed by steps, derived from full Context:
	GetHost() connector.Host    // Returns the specific host this step is operating on
	StepCache() cache.StepCache
	TaskCache() cache.TaskCache     // Access to parent task's cache
	ModuleCache() cache.ModuleCache   // Access to parent module's cache
	GetGlobalWorkDir() string         // Access to global work directory
}
