package task

import (
	"github.com/mensylisir/kubexm/pkg/cache" // For cache.TaskCache, cache.ModuleCache, cache.PipelineCache
	// "github.com/mensylisir/kubexm/pkg/plan"  // Will be used for plan.NodeID, plan.ExecutionNode - Removed as ExecutionFragment is now local to task pkg
	// "github.com/mensylisir/kubexm/pkg/runtime" // No longer needed for runtime.TaskContext
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/runner"    // For runner.Facts
	// "github.com/mensylisir/kubexm/pkg/module" // REMOVED to break cycle
	"context"                                                // Added
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Added
	"github.com/mensylisir/kubexm/pkg/logger"                // Added
	// "github.com/mensylisir/kubexm/pkg/engine" // Removed, GetEngine removed from TaskContext
)

// TaskContext defines the methods available at the task execution level.
type TaskContext interface {
	// Methods from former embedded ModuleContext (which were from PipelineContext)
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	GetPipelineCache() cache.PipelineCache
	GetGlobalWorkDir() string
	// GetEngine() engine.Engine // Removed

	// Methods from former embedded ModuleContext (module-specific)
	GetModuleCache() cache.ModuleCache

	// Task-specific methods
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	GetTaskCache() cache.TaskCache
	GetControlNode() (connector.Host, error)
}

// Task defines the methods that all concrete task types must implement.
// Tasks are responsible for planning a subgraph of operations (ExecutionFragment)
// to achieve a specific part of a module's goal.
type Task interface {
	// Name returns the designated name of the task.
	Name() string

	// Description provides a brief summary of what the task does.
	// This can be removed if not strictly needed by the new model,
	// or kept for informational purposes. For now, I'll keep it.
	Description() string

	// IsRequired determines if the task needs to generate a plan.
	// This can be based on the current system state (via TaskContext)
	// or configuration.
	IsRequired(ctx TaskContext) (bool, error) // Changed to local TaskContext

	// Plan now generates an ExecutionFragment, a self-contained subgraph
	// with defined entry and exit points for linking.
	Plan(ctx TaskContext) (*ExecutionFragment, error) // Changed to local TaskContext
}
