package module

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/task"
)

// ModuleContext defines the methods available at the module execution level.
type ModuleContext interface {
	// Core context methods
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	
	// Cache access
	GetPipelineCache() cache.PipelineCache
	GetModuleCache() cache.ModuleCache
	
	// Host and runner access (only through runner layer)
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	GetControlNode() (connector.Host, error)
	GetRunner() runner.Runner
	
	// Working directory access
	GetGlobalWorkDir() string
	GetCertsDir() string
	GetEtcdCertsDir() string
	GetEtcdArtifactsDir() string
	GetKubernetesArtifactsDir() string
	GetContainerRuntimeArtifactsDir() string
	GetHostDir(hostName string) string
}

// Module defines the methods that all concrete module types must implement.
// Modules are responsible for planning a larger ExecutionFragment by orchestrating
// and linking multiple Task ExecutionFragments.
type Module interface {
	// Name returns the designated name of the module.
	Name() string

	// Description returns a brief description of the module.
	Description() string

	// GetTasks returns a list of tasks that belong to this module.
	GetTasks(ctx ModuleContext) ([]task.Task, error)

	// Plan aggregates ExecutionFragments from its tasks into a larger ExecutionFragment.
	Plan(ctx ModuleContext) (*task.ExecutionFragment, error)
}
