package module

import (
	// "github.com/mensylisir/kubexm/pkg/plan" // No longer directly returns ExecutionPlan
	"context"                                                // Added
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Added
	"github.com/mensylisir/kubexm/pkg/cache"                 // For cache.ModuleCache and cache.PipelineCache
	"github.com/mensylisir/kubexm/pkg/engine"                // Added for engine.Engine (if GetEngine is kept)
	"github.com/mensylisir/kubexm/pkg/logger"                // Added
	// "github.com/mensylisir/kubexm/pkg/pipeline" // REMOVED to break module <-> pipeline cycle
	"github.com/mensylisir/kubexm/pkg/task" // Uses task.Task and task.ExecutionFragment
)

// ModuleContext defines the methods available at the module execution level.
type ModuleContext interface {
	// Methods previously from pipeline.PipelineContext that modules need:
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	GetPipelineCache() cache.PipelineCache
	GetGlobalWorkDir() string
	// GetEngine() engine.Engine // Modules should not directly access the engine; execution is handled by Pipeline.

	// Module-specific methods:
	GetModuleCache() cache.ModuleCache
	// Add other methods specific to module context if needed, e.g., GetModuleConfig() if modules have their own config sections.
}

// Module defines the methods that all concrete module types must implement.
// Modules are responsible for planning a larger ExecutionFragment by orchestrating
// and linking multiple Task ExecutionFragments.
type Module interface {
	// Name returns the designated name of the module.
	Name() string

	// GetTasks returns a list of tasks that belong to this module, potentially dynamically
	// determined based on the provided context (e.g., cluster configuration).
	// Returns an error if tasks cannot be determined.
	GetTasks(ctx ModuleContext) ([]task.Task, error)

	// Plan aggregates ExecutionFragments from its tasks into a larger ExecutionFragment.
	// It is responsible for linking the exit nodes of one task's fragment
	// to the entry nodes of the next task's fragment, creating dependencies according
	// to the module's logic (e.g., sequential, parallel).
	Plan(ctx ModuleContext) (*task.ExecutionFragment, error)
}
