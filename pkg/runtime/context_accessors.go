package runtime

import (
	"context"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	// Import specific context interfaces from pipeline, module, task, step
	// To avoid import cycles if these packages also import runtime for Context,
	// we might need to define these interfaces in a neutral package or use interface{}
	// and type assertions. For now, assuming direct interface usage is manageable
	// or these interface definitions are moved to a common place e.g. pkg/contexts.
	// For this implementation, I will assume the interfaces are accessible.
	// Let's try to make this work by having the context implementations here
	// and the interfaces defined in their respective packages.
	// The actual interface satisfaction will be checked by the compiler.
)

// baseContext provides common fields and methods for all context implementations.
// It holds a pointer to the main runtime Context, allowing access to shared services and data.
type baseContext struct {
	mainCtx *Context
}

// --- PipelineContext Implementation ---

// pipelineContextImpl implements the pipeline.PipelineContext interface.
type pipelineContextImpl struct {
	baseContext
}

// NewPipelineContext creates a new context for pipeline execution.
func NewPipelineContext(mainCtx *Context) *pipelineContextImpl {
	return &pipelineContextImpl{baseContext{mainCtx: mainCtx}}
}

func (ctx *pipelineContextImpl) GoContext() context.Context                 { return ctx.mainCtx.GoCtx }
func (ctx *pipelineContextImpl) GetLogger() *logger.Logger                  { return ctx.mainCtx.Logger }
func (ctx *pipelineContextImpl) GetClusterConfig() *v1alpha1.Cluster        { return ctx.mainCtx.ClusterConfig }
func (ctx *pipelineContextImpl) GetPipelineCache() cache.PipelineCache      { return ctx.mainCtx.PipelineCache } // Note: Direct access to mainCtx.PipelineCache
func (ctx *pipelineContextImpl) GetGlobalWorkDir() string                   { return ctx.mainCtx.GlobalWorkDir }
func (ctx *pipelineContextImpl) GetEngine() engine.Engine                   { return ctx.mainCtx.Engine }

// --- ModuleContext Implementation ---

// moduleContextImpl implements the module.ModuleContext interface.
type moduleContextImpl struct {
	baseContext
}

// NewModuleContext creates a new context for module execution.
func NewModuleContext(mainCtx *Context) *moduleContextImpl {
	return &moduleContextImpl{baseContext{mainCtx: mainCtx}}
}

func (ctx *moduleContextImpl) GoContext() context.Context                 { return ctx.mainCtx.GoCtx }
func (ctx *moduleContextImpl) GetLogger() *logger.Logger                  { return ctx.mainCtx.Logger }
func (ctx *moduleContextImpl) GetClusterConfig() *v1alpha1.Cluster        { return ctx.mainCtx.ClusterConfig }
func (ctx *moduleContextImpl) GetPipelineCache() cache.PipelineCache      { return ctx.mainCtx.PipelineCache }
func (ctx *moduleContextImpl) GetModuleCache() cache.ModuleCache          { return ctx.mainCtx.ModuleCache } // Note: Direct access
func (ctx *moduleContextImpl) GetGlobalWorkDir() string                   { return ctx.mainCtx.GlobalWorkDir }
func (ctx *moduleContextImpl) GetEngine() engine.Engine { return ctx.mainCtx.Engine } // Module might need engine to plan sub-tasks if design changes

// --- TaskContext Implementation ---

// taskContextImpl implements the task.TaskContext interface.
type taskContextImpl struct {
	baseContext
}

// NewTaskContext creates a new context for task execution.
func NewTaskContext(mainCtx *Context) *taskContextImpl {
	return &taskContextImpl{baseContext{mainCtx: mainCtx}}
}

func (ctx *taskContextImpl) GoContext() context.Context                                          { return ctx.mainCtx.GoCtx }
func (ctx *taskContextImpl) GetLogger() *logger.Logger                                           { return ctx.mainCtx.Logger }
func (ctx *taskContextImpl) GetClusterConfig() *v1alpha1.Cluster                                 { return ctx.mainCtx.ClusterConfig }
func (ctx *taskContextImpl) GetPipelineCache() cache.PipelineCache                               { return ctx.mainCtx.PipelineCache }
func (ctx *taskContextImpl) GetModuleCache() cache.ModuleCache                                   { return ctx.mainCtx.ModuleCache }
func (ctx *taskContextImpl) GetTaskCache() cache.TaskCache                                     { return ctx.mainCtx.TaskCache } // Note: Direct access
func (ctx *taskContextImpl) GetHostsByRole(role string) ([]connector.Host, error)                { return ctx.mainCtx.GetHostsByRole(role) }
func (ctx *taskContextImpl) GetHostFacts(host connector.Host) (*runner.Facts, error)             { return ctx.mainCtx.GetHostFacts(host) }
func (ctx *taskContextImpl) GetControlNode() (connector.Host, error)                             { return ctx.mainCtx.GetControlNode() }
func (ctx *taskContextImpl) GetGlobalWorkDir() string                                            { return ctx.mainCtx.GlobalWorkDir }
func (ctx *taskContextImpl) GetConnectorForHost(host connector.Host) (connector.Connector, error) { return ctx.mainCtx.GetConnectorForHost(host) }


// --- StepContext Implementation ---

// stepContextImpl implements the step.StepContext interface.
type stepContextImpl struct {
	baseContext
	currentGoCtx context.Context // Specific Go context for this step's execution (e.g., from errgroup)
	currentHost  connector.Host  // The specific host this step is executing on
}

// NewStepContext creates a new context for step execution.
// It is typically called by the engine when dispatching a step to a specific host.
func NewStepContext(mainCtx *Context, host connector.Host, goCtx context.Context) *stepContextImpl {
	return &stepContextImpl{
		baseContext:  baseContext{mainCtx: mainCtx},
		currentGoCtx: goCtx,
		currentHost:  host,
	}
}

func (ctx *stepContextImpl) GoContext() context.Context                 { return ctx.currentGoCtx }
func (ctx *stepContextImpl) GetLogger() *logger.Logger                  { return ctx.mainCtx.Logger.With("host", ctx.currentHost.GetName()) }
func (ctx *stepContextImpl) GetHost() connector.Host                    { return ctx.currentHost }
func (ctx *stepContextImpl) GetRunner() runner.Runner                   { return ctx.mainCtx.Runner }
func (ctx *stepContextImpl) GetClusterConfig() *v1alpha1.Cluster        { return ctx.mainCtx.ClusterConfig }
func (ctx *stepContextImpl) GetStepCache() cache.StepCache                { return ctx.mainCtx.StepCache } // Note: Direct access
func (ctx *stepContextImpl) GetTaskCache() cache.TaskCache                { return ctx.mainCtx.TaskCache }
func (ctx *stepContextImpl) GetModuleCache() cache.ModuleCache            { return ctx.mainCtx.ModuleCache }


func (ctx *stepContextImpl) GetHostsByRole(role string) ([]connector.Host, error)           { return ctx.mainCtx.GetHostsByRole(role) }
func (ctx *stepContextImpl) GetHostFacts(host connector.Host) (*runner.Facts, error)          { return ctx.mainCtx.GetHostFacts(host) }
func (ctx *stepContextImpl) GetCurrentHostFacts() (*runner.Facts, error)                      { return ctx.mainCtx.GetHostFacts(ctx.currentHost) }
func (ctx *stepContextImpl) GetConnectorForHost(host connector.Host) (connector.Connector, error) { return ctx.mainCtx.GetConnectorForHost(host) }
func (ctx *stepContextImpl) GetCurrentHostConnector() (connector.Connector, error)            { return ctx.mainCtx.GetConnectorForHost(ctx.currentHost) }
func (ctx *stepContextImpl) GetControlNode() (connector.Host, error)                          { return ctx.mainCtx.GetControlNode() }

func (ctx *stepContextImpl) GetGlobalWorkDir() string                                         { return ctx.mainCtx.GlobalWorkDir }
func (ctx *stepContextImpl) IsVerbose() bool                                                  { return ctx.mainCtx.GlobalVerbose }
func (ctx *stepContextImpl) ShouldIgnoreErr() bool                                            { return ctx.mainCtx.GlobalIgnoreErr }
func (ctx *stepContextImpl) GetGlobalConnectionTimeout() time.Duration                        { return ctx.mainCtx.GlobalConnectionTimeout }

// Artifact path helpers
func (ctx *stepContextImpl) GetClusterArtifactsDir() string          { return ctx.mainCtx.GetClusterArtifactsDir() }
func (ctx *stepContextImpl) GetCertsDir() string                     { return ctx.mainCtx.GetCertsDir() }
func (ctx *stepContextImpl) GetEtcdCertsDir() string                 { return ctx.mainCtx.GetEtcdCertsDir() }
func (ctx *stepContextImpl) GetComponentArtifactsDir(componentName string) string { return ctx.mainCtx.GetComponentArtifactsDir(componentName) }
func (ctx *stepContextImpl) GetEtcdArtifactsDir() string             { return ctx.mainCtx.GetEtcdArtifactsDir() }
func (ctx *stepContextImpl) GetContainerRuntimeArtifactsDir() string { return ctx.mainCtx.GetContainerRuntimeArtifactsDir() }
func (ctx *stepContextImpl) GetKubernetesArtifactsDir() string       { return ctx.mainCtx.GetKubernetesArtifactsDir() }
func (ctx *stepContextImpl) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	return ctx.mainCtx.GetFileDownloadPath(componentName, version, arch, fileName)
}
func (ctx *stepContextImpl) GetHostDir(hostname string) string { return ctx.mainCtx.GetHostDir(hostname) }

// WithGoContext for step.StepContext to allow engine to set per-step go context.
// This method needs to return the step.StepContext interface type.
// To avoid import cycles, this might require step.StepContext to be defined in a common package,
// or use interface{} and type assertion in the engine.
// For now, returning a concrete type and assuming the engine will handle it.
// The actual interface is defined in pkg/step/interface.go
func (ctx *stepContextImpl) WithGoContext(goCtx context.Context) interface{} { // step.StepContext {
	newStepCtx := *ctx
	newStepCtx.currentGoCtx = goCtx
	return &newStepCtx
}

// Ensure context implementations satisfy their respective interfaces.
// This will be checked by the compiler once the interfaces are defined/imported.
// e.g.
// var _ pipeline.PipelineContext = (*pipelineContextImpl)(nil)
// var _ module.ModuleContext = (*moduleContextImpl)(nil)
// var _ task.TaskContext = (*taskContextImpl)(nil)
// var _ step.StepContext = (*stepContextImpl)(nil)

// The main *runtime.Context will need methods to create these specific contexts.
// For example:
func (c *Context) NewPipelineContext() *pipelineContextImpl { // Should return pipeline.PipelineContext
	return NewPipelineContext(c)
}

func (c *Context) NewModuleContext() *moduleContextImpl { // Should return module.ModuleContext
	return NewModuleContext(c)
}

func (c *Context) NewTaskContext() *taskContextImpl { // Should return task.TaskContext
	return NewTaskContext(c)
}

// NewStepContext is slightly different as it needs host and goCtx
// This would typically be called by the engine: NewStepContext(mainCtx, host, goCtx)
// So, *Context doesn't need a method that returns a step.StepContext without host/goCtx.
// The engine will construct stepContextImpl directly using NewStepContext defined above.

// Helper function to get the main context from any of the wrapped contexts if needed,
// though this is usually not exposed to the layers themselves.
func (b *baseContext) GetMainContext() *Context {
	return b.mainCtx
}
