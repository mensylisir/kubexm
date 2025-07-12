package task

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// TaskContext defines the methods available at the task execution level.
type TaskContext interface {
	// Core context methods
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	
	// Cache access
	GetPipelineCache() cache.PipelineCache
	GetModuleCache() cache.ModuleCache
	GetTaskCache() cache.TaskCache
	
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

// Task defines the methods that all concrete task types must implement.
type Task interface {
	// Name returns the designated name of the task.
	Name() string

	// Description provides a brief summary of what the task does.
	Description() string

	// IsRequired determines if the task needs to generate a plan.
	IsRequired(ctx TaskContext) (bool, error)

	// Plan generates an ExecutionFragment for this task.
	Plan(ctx TaskContext) (*ExecutionFragment, error)
}
