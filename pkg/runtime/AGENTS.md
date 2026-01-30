# AGENTS.md - Runtime Context and State Management

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
The runtime package provides the foundational execution context, state management, and data flow mechanisms for the kubexm orchestration engine. All other packages (runner, step, task, module, pipeline) depend on this package.

## STRUCTURE
```
pkg/runtime/
├── context.go          # Main Context struct with execution context and state management
├── state.go            # StateBag interface and thread-safe implementation
├── errors.go           # InitializationError for error collection
├── execution.go        # ExecutionContext interface (lowest-level context)
├── orchestration.go    # Orchestration context interfaces (Pipeline, Module, Task)
├── base.go             # Core service, cluster query, filesystem, cache, settings interfaces
├── builder.go          # Builder pattern for constructing Context instances
├── data_bus.go         # SimpleDataBus for data sharing between steps
├── data_flow.go        # DataManager with typed publishers/subscribers
├── typed_state.go      # TypedStateBag for type-safe state storage
└── data_bus_v2.go      # Enhanced DataBus with channel-based subscriptions
```

## WHERE TO LOOK
| Type/Concept | Location | Notes |
|--------------|----------|-------|
| Context construction | `builder.go` | Builder pattern: NewBuilder() → With*() → Build() |
| Main Context struct | `context.go` | Implements all context interfaces |
| State storage | `state.go` | StateBag interface with Set/Get/Delete |
| Context interfaces | `execution.go`, `orchestration.go`, `base.go` | ExecutionContext (lowest), TaskContext, ModuleContext, PipelineContext |
| Context forking | `context.go` | ForPipeline(), ForModule(), ForTask(), ForHost() |
| Data flow | `data_flow.go`, `data_bus_v2.go` | DataManager, DataBus, TypedDataBus |
| Type-safe state | `typed_state.go` | TypedStateBag[T], StepOutput types |
| Error handling | `errors.go` | InitializationError for multi-error collection |

## CONVENTIONS
Follow the parent AGENTS.md for general Go conventions. This document focuses on runtime-specific patterns.

### Context Hierarchy
```
ExecutionContext (lowest level, step execution)
    ↓
TaskContext (task-level operations)
    ↓
ModuleContext (module-level operations)
    ↓
PipelineContext (pipeline-level operations)
```

### State Scoping
State is scoped hierarchically: GlobalState → PipelineState → ModuleState → TaskState
- Use `Export()` to write to specific scope
- Use `Import()` to read from specific scope or cascade (default: Task → Module → Pipeline → Global)

### Key Methods
| Method | Purpose |
|--------|---------|
| `ForPipeline(name)` | Create pipeline-scoped context |
| `ForModule(name)` | Create module-scoped context |
| `ForTask(name)` | Create task-scoped context |
| `ForHost(host)` | Create host-specific context |
| `Export(scope, key, value)` | Publish data to scope |
| `Import(scope, key)` | Subscribe to data from scope |

## ANTI-PATTERNS
- **Never create circular imports**: pkg/runtime imports pkg/runner, pkg/runner imports pkg/runtime - DO NOT add pkg/runner imports to runtime
- **Never use StateBag for type-safe operations**: Use `TypedStateBag[T]` or DataManager/DataBus instead
- **Never share StateBag across goroutines without synchronization**: StateBag is thread-safe, but ensure proper usage
- **Never use `context.Context` directly**: Use `runtime.Context` or appropriate interface (TaskContext, etc.)
- **Never cache connectors in StateBag**: Connectors are managed by ConnectionPool
- **Never modify cached StateBag references**: StateBag methods copy data on Get; use GetAll() for snapshot

## UNIQUE STYLES

### Builder Pattern
```go
ctx, cleanup, err := runtime.NewBuilder(configFile).
    WithRunID(runID).
    WithLogger(logger).
    WithHttpClient(client).
    Build(context.Background())
defer cleanup()
```

### Context Forking (Immutable Context Pattern)
All `For*()` methods create shallow copies of context with isolated state:
```go
pipelineCtx := rootCtx.ForPipeline("install")
moduleCtx := pipelineCtx.ForModule("kubernetes")
taskCtx := moduleCtx.ForTask("kubeadm-init")
```

### Data Flow Patterns
**Simple Export/Import:**
```go
// Export
ctx.Export("task", "kubeadm.token", token)

// Import (cascades: Task → Module → Pipeline → Global)
token, found := ctx.Import("", "kubeadm.token")
```

**Type-safe DataBus:**
```go
// Publish
db := runtime.NewDataBus(ctx)
db.Publish("kubeadm.init.token", token)

// Subscribe
token, found := db.GetString("kubeadm.init.token")
```

**Typed DataBus:**
```go
// Create typed bus
tokenBus := runtime.NewTypedDataBus[string](ctx, "kubeadm.token")
tokenBus.Publish(token)

// Subscribe with type safety
token, found := tokenBus.Get()
```

**DataManager with Publishers/Subscribers:**
```go
dm := runtime.NewDataManager(ctx)
pub := runtime.NewKubeadmPublisher(dm)
pub.PublishInitData(runtime.KubeadmInitData{Token: token, ...})

// Subscribe
sub := runtime.NewKubeadmSubscriber(dm)
data, ok := sub.GetInitData()
```

## NOTES

### Initialization Flow
1. Builder reads/parses cluster config (`v1alpha1.Cluster`)
2. Initializes connection pool and connector factory
3. Initializes all hosts in parallel (using errgroup)
4. Each host: creates connector, connects, gathers facts
5. Builds caches (pipeline → module → task → step)
6. Creates StateBags for each scope (Global, Pipeline, Module, Task)

### Circular Dependency Warning
⚠️ **CRITICAL**: `pkg/runtime` and `pkg/runner` have a circular dependency:
- `pkg/runtime` imports `pkg/runner` (for `runner.Runner`, `runner.Facts`)
- `pkg/runner` imports `pkg/runtime` (from `file.go` - likely a mistake)

When adding code to `pkg/runtime`, **DO NOT** import `pkg/runner` - this will exacerbate the cycle.

### Context Scoping Best Practices
- Use `ExecutionContext` for step-level operations (lowest level)
- Use `TaskContext` for task-level orchestration
- Use `ModuleContext` for module-level orchestration
- Use `PipelineContext` for pipeline-level orchestration
- Never pass broader context than needed

### State Management
- StateBag is thread-safe (uses sync.RWMutex)
- State cascades upward: Task can read Module, Pipeline, Global state
- State does NOT cascade downward: Pipeline cannot read Task state
- Each scope is isolated by default

### Data Flow Recommendations
1. For simple key-value pairs: Use `Export()`/`Import()`
2. For type safety: Use `TypedDataBus[T]`
3. For complex data structures: Use `DataManager` with typed publishers/subscribers
4. For step outputs: Use `TypedStateBag[T]` with `StepOutput` marker interface

### Key Constants (data_bus_v2.go)
- `KeyKubeadmInitData`, `KeyKubeadmJoinData`
- `KeyEtcdClusterData`, `KeyEtcdEndpoints`
- `KeyLoadBalancerVIP`, `KeyLoadBalancerConfig`
- `KeyPKICACert`, `KeyPKIClientCert`
- `KeyHostFacts`, `KeyAllHostFacts`

### Builder Cleanup
Always call the cleanup function returned by `Builder.Build()`:
```go
ctx, cleanup, err := builder.Build(context.Background())
defer cleanup() // Shuts down connection pool
```
