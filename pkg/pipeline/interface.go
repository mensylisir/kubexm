package pipeline

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// PipelineContext defines the methods available at the pipeline execution level.
type PipelineContext interface {
	// Core context methods
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	
	// Cache access
	GetPipelineCache() cache.PipelineCache
	
	// Engine access for execution
	GetEngine() engine.Engine
	
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

// Pipeline defines the methods that all concrete pipeline types must implement.
// Pipelines are responsible for orchestrating multiple Modules to generate
// the final ExecutionGraph and then running it using the Engine.
type Pipeline interface {
	// Name returns the designated name of the pipeline.
	Name() string

	// Description returns a brief description of the pipeline.
	Description() string

	// Modules returns a list of modules that belong to this pipeline.
	Modules() []module.Module

	// Plan generates the final ExecutionGraph by orchestrating module fragments.
	Plan(ctx PipelineContext) (*plan.ExecutionGraph, error)

	// Run executes the pipeline using the provided ExecutionGraph.
	Run(ctx PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
