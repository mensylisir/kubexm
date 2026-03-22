# AGENTS.md - AI Coding Agent Guidelines

This file provides guidelines for AI coding agents working on kubexm codebase.

## Build, Lint and Test Commands

### Build Commands

```bash
# Build the entire project
go build ./...

# Build kubexm CLI
go build ./bin/kubexm

# Build specific packages
go build ./internal/tool/...
go build ./internal/runtime/...
go build ./internal/runner/...
go build ./internal/step/...
go build ./internal/task/...
go build ./internal/module/...
go build ./internal/util/...

# Build specific files
go build ./internal/tool/json.go
```

### Test Commands

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/tool/...
go test ./internal/runtime/...
go test ./internal/runner/...
go test ./internal/step/...
go test ./internal/task/...

# Run tests with verbose output
go test -v ./internal/tool/...

# Run tests with coverage
go test -cover ./...
go test -cover ./internal/tool/...

# Run single test function (go 1.21+)
go test -run TestFunctionName ./internal/tool/json.go

# Run specific test with coverage
go test -cover ./internal/tool/...
```

### Lint Commands

```bash
# Run go vet (built-in linter)
go vet ./...

# Run go vet on specific package
go vet ./internal/tool/...

# Run golangci-lint (if installed)
golangci-lint run ./...
```

## Code Style Guidelines

### Import Conventions

```go
// Standard library imports first (grouped, blank line after)
import (
    "errors"
    "fmt"
    "strconv"
)

// External library imports (grouped, blank line after)
import (
    "github.com/pkg/errors"
    "gopkg.in/yaml.v3"
)

// Internal kubexm imports (grouped, blank line after)
import (
    "github.com/mensylisir/kubexm/internal/common"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/spec"
)
```

### Naming Conventions

**Package Names**
- Use lowercase single words: `tool`, `util`, `runtime`, `runner`
- Avoid abbreviations unless widely known: `step`, `task`, `module`, `pipeline`

**Constants**
- `UPPER_SNAKE_CASE` for constants
- `const MaxRetries = 3`
- `const DefaultTimeout = 30 * time.Second`

**Interfaces**
- Start with capital letter: `Runner`, `Step`, `Task`, `Module`
- Use short, descriptive names: `Connector`, `ExecutionContext`, `ClusterConfig`

**Types/Structs**
- Start with capital letter: `Image`, `Cluster`, `Config`
- Use descriptive names: `LoadBalancerSpec`, `StepMeta`, `TaskContext`

**Functions**
- Exported functions: PascalCase: `NewRunner`, `GetImage`, `GenerateConfig`
- Internal functions: camelCase: `parseToken`, `isValidUUID`, `getHostConnector`
- Private methods (unexported): camelCase: `validateInput`, `cleanupResources`

**Variables**
- camelCase: `hostConnector`, `clusterConfig`, `imageList`
- Short names: `err`, `ctx`, `cfg`
- Avoid single-letter variables except in loops: `i`, `j`, `k`

**Interfaces and Structs**
- Interfaces: Start with `I` prefix for interfaces
  - `type Runner interface { ... }`
  - `type Connector interface { ... }`
  - Structs: No prefix
  - `type Runner struct { ... }`
  - `type ClusterConfig struct { ... }`

**Functions**
- Exported functions: PascalCase: `NewRunner`, `GetImage`, `GenerateConfig`
- Internal functions: camelCase: `parseToken`, `isValidUUID`, `getHostConnector`
- Private methods (unexported): camelCase: `validateInput`, `cleanupResources`

### Formatting Conventions

```go
// Function definitions
func NewRunner(ctx runtime.ExecutionContext, conn connector.Connector) *Runner {
    return &Runner{
        ctx:  ctx,
        conn: conn,
        // ...
    }
}

// Struct initialization
cluster := &Cluster{
    Name: "my-cluster",
    Spec: spec.ClusterSpec{
        // ...
    },
}

// Error wrapping
return fmt.Errorf("failed to connect to host %s: %w", host, err)

// Error checking
if err != nil {
    return err
}
```

### Type Conventions

**Prefer Built-in Types**
- Use `string` instead of `[]byte` for text
- Use `[]string` for string lists
- Use `map[string]string` for key-value pairs
- Use `time.Duration` for time values

**Custom Types**
```go
// For simple cases, basic types are preferred
func ProcessFile(path string) error {
    // Use string path instead of os.File
}

// For complex cases, define types
type FileProcessor struct {
    path   string
    config *FileConfig
    // ...
}
```

### Error Handling

**Error Wrapping**
```go
import "github.com/pkg/errors"

// Always wrap errors with context
return errors.Wrapf(err, "failed to execute command on host %s", host)

// Provide context in error messages
return errors.Errorf("invalid timeout value: %s (must be > 0)", timeout)
```

**Error Creation**
```go
import (
    "errors"
    "fmt"
)

var (
    ErrInvalidInput  = errors.New("invalid input")
    ErrNotFound      = errors.New("resource not found")
    ErrTimeout       = errors.New("operation timeout")
)

// Use typed errors in returns
return ErrInvalidInput
```

**Error Handling Patterns**

```go
// Check and return early
if err != nil {
    return errors.Wrap(err, "failed to read config")
}

// Check and log warnings
if err != nil {
    logger.Warnf("failed to cleanup, ignoring: %v", err)
}

// Check and recover from panics
defer func() {
    if r := recover(); r != nil {
        logger.Errorf("panic recovered: %v", r)
    }
}()
```

### Constants Organization

**Constant Locations**
- Package-level constants: At the top of the file after imports
- Group related constants together
- Use descriptive constant names

```go
const (
    // Default values
    DefaultTimeout    = 30 * time.Second
    MaxRetries        = 3
    BufferSize       = 4096
    
    // Error messages
    ErrInvalidConfig = "invalid configuration"
    ErrNotFound      = "resource not found"
)
)
```

### Comment Guidelines

**Public API Documentation**
- Exported functions and types should have comments
- Use the format: `// FunctionName does X, Y, Z`
- Document purpose, parameters, return values

```go
// NewRunner creates a new Runner instance with given context and connector
func NewRunner(ctx runtime.ExecutionContext, conn connector.Connector) *Runner {
    // ...
}

// Image represents a container image with registry and tag information
type Image struct {
    // ...
}
```

**Complex Logic Comments**
- Add comments for non-obvious logic
- Explain WHY, not WHAT
- Keep comments concise

```go
// Validate input before proceeding (WHY: input validation is critical)
if !isValid(input) {
    return ErrInvalidInput
}

// Use exponential backoff for retries (WHAT: implementation detail)
retries := 0
for retries < MaxRetries {
    if err := attempt(); err == nil {
        break
    }
    retries++
    time.Sleep(time.Second * time.Duration(1<<uint(retries)))
}
```

**Comment Style**
- Use `//` for single-line comments
- Use `/* */` for multi-line comments (rare)
- Avoid TODO comments; use FIXME for temporary workarounds
- Use `NOTE:` for important information

### Concurrency Patterns

**Prefer Channels over Mutex**
```go
// Good: Channel-based
resultCh := make(chan Result, 1)
go func() {
    resultCh <- process(ctx)
}()

// Avoid: Shared state with mutex
var mu sync.Mutex
var sharedState State
```

**Use sync.Pool for Reusable Objects**
```go
// Good: Using pool for buffer reuse
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}

// Avoid: Allocate in hot path
func process() {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    // ...
}
```

### Package Organization

```
internal/
├── tool/          # Pure utility functions, no internal dependencies
│   ├── json.go
│   ├── yaml.go
│   ├── toml.go
│   └── path.go
├── runtime/       # Context and execution abstractions
│   ├── context.go
│   ├── state.go
│   └── errors.go
├── runner/        # Operations layer (command, file, service, container)
│   ├── interface.go
│   ├── runner.go
│   └── helpers/      # Delegates to internal/tool
├── step/          # Atomic, idempotent operations
│   ├── interface.go
│   ├── base.go
│   └── ...
├── task/          # Step composition
├── module/        # Task composition
├── pipeline/       # Module orchestration
└── util/          # General utilities and runtime-dependent operations
```

**Package Dependencies**

```
tool          → No internal dependencies (pure)
runtime       → Depends on: connector, common, spec
runner         → Depends on: runtime, connector
step           → Depends on: runtime, spec
task           → Depends on: step, runtime
module         → Depends on: task, runtime
pipeline       → Depends on: module, runtime
util           → Depends on: tool, runtime (for runtime operations)
```

### Testing Guidelines

**Unit Tests**
- Write tests for all exported functions
- Use table-driven tests for multiple test cases
- Test both success and failure paths
- Mock external dependencies using interfaces
- Use `t.Run()` for subtests
- Use `t.Helper()` for helper functions

```go
// Example: Table-driven test
func TestParseCPU(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected int64
        wantErr  bool
    }{
        {
            name:     "valid CPU string",
            input:    "100m",
            expected: 100000000,
            wantErr: false,
        },
        {
            name:     "empty CPU string",
            input:    "",
            expected: 0,
            wantErr: true,
        },
    },
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := ParseCPU(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}

// Testing Error Cases
func TestNewRunner(t *testing.T) {
    t.Run("invalid context", func(t *testing.T) {
        conn := &mockConnector{}
        runner := NewRunner(nil, conn)
        assert.NotNil(t, runner.ctx)
    })

    t.Run("connection error", func(t *testing.T) {
        conn := &mockConnector{err: errors.New("connection failed")}
        runner := NewRunner(&mockContext{}, conn)
        err := runner.Run(mockContext{}, "test")
        assert.Error(t, err)
    })
}
```

### Architecture Patterns

**Dependency Injection**
```go
// Good: Dependency injection
func NewStep(ctx runtime.ExecutionContext, runner Runner) *Step {
    return &Step{
        ctx:    ctx,
        runner: runner,
    }
}

// Avoid: Global state
var globalCtx runtime.ExecutionContext
func NewStepBad() *Step {
    return &Step{ctx: globalCtx}
}
```

**Interface Segregation**
```go
// Good: Small interface
type FileRunner interface {
    WriteFile(ctx context.Context, path string, content []byte) error
    ReadFile(ctx context.Context, path string) ([]byte, error)
}

// Avoid: God interface
type Runner interface {
    FileRunner
    CommandRunner
    ServiceRunner
    ContainerRunner
    // ... 100+ methods
}
```

## Project Architecture

### Layer Responsibilities

```
┌─────────────────────────────────────────────────────┐
│                    internal/                              │
├─────────────────────────────────────────────────────┤
│  tool/    │  Pure parsing utilities (JSON, YAML, TOML, path)  │
├─────────────────────────────────────────────────────┤
│  runtime/  │  Execution context, state, errors         │
├─────────────────────────────────────────────────────┤
│  runner/   │  Operations: command, file, service, container   │
│             │  │  Delegates to: runtime, connector, tool     │
├─────────────────────────────────────────────────────┤
│  step/     │  Atomic, idempotent operations                │
│             │  │ Implements: Step interface                 │
│             │  │ Delegates to: runner, runtime           │
├─────────────────────────────────────────────────────┤
│  task/     │  │ Step composition and lifecycle           │
│  module/   │  │ Task composition and orchestration       │
│  pipeline/  │  Module orchestration                    │
│  util/     │ │ Delegates to: tool, runtime              │
└─────────────────────────────────────────────────────┘
                    internal/apis/                               │
├─────────────────────────────────────────────────────┤
│  kubexms/  │  CRD types, configuration specs              │
│             │  │                                            │
│  connector/  │  SSH and local connection interfaces         │
│             │  │  - SSHConnector                             │
│             │  │  - LocalConnector                          │
│             │  │  - Connector interface (union)             │
└─────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Single Responsibility**: Each layer has a single, well-defined purpose
2. **Dependency Direction**: Dependencies flow downward (connector → runtime → runner → step → task → module → pipeline)
3. **Interface Segregation**: Use small, focused interfaces
4. **Idempotency**: All operations should be idempotent
5. **Testability**: Components should be testable in isolation
6. **Circular Dependency Prevention**: `tool` package has NO internal dependencies, breaking cycles

### Working with Codebase

**Before Making Changes**
1. Build the target package: `go build ./internal/tool/...`
2. Run tests for the package: `go test ./internal/tool/...`
3. Check existing similar code patterns in codebase
4. Update imports if moving code between packages
5. Follow import ordering conventions
6. Add error handling with context
7. Write tests before implementation
8. Follow existing naming conventions

**When Adding New Features**
1. Define interface first
2. Implement in appropriate layer (tool, runtime, runner, step, task, module)
3. Write tests before implementation
4. Follow import ordering conventions
5. Use dependency injection
6. Add error handling with context
7. Write tests for both success and failure paths
8. Follow existing naming conventions

**When Refactoring**
1. Start by identifying the problem
2. Create a plan to fix it
3. Make small, incremental changes
4. Build and test after each change
5. Update related code that depends on changed code
6. Delete dead code after verification
7. Update related documentation

**Common Pitfalls**

1. **Circular Imports**: Moving code from `util` to `runtime` can create cycles
   - Check import dependencies before moving code
   - Use interfaces to break circular dependencies
   - `tool` package has NO internal dependencies

2. **God Interfaces**: The `Runner` interface has 200+ methods
   - When working with `internal/runner`, use only what you need
   - Don't create new "Extended" interfaces
   - Small, focused interfaces are preferred

3. **Missing Cleanup**: Operations that create resources should clean them up
   - Implement `Cleanup()` method for all Steps
   - Use `defer` for temporary resources
   - Log cleanup errors but don't fail the operation

4. **Inconsistent Error Handling**: Mix of different error patterns
   - Always use `errors.Wrap()` for context
   - Always return errors from error paths
   - Define typed errors at package level
   - Mix of return patterns (error, tuple, error)

5. **Package Organization Confusion**: Code scattered across `util`, `helpers`, `runner/helpers`
   - Use `internal/tool` for pure utilities (no dependencies)
   - Use `internal/util` for runtime-dependent utilities
   - Use `internal/runner/helpers` to delegate to `internal/tool`

### Tool Package (internal/tool)

**Purpose**: Pure utility functions with NO internal dependencies

**What Goes Here**:
- JSON parsing (from `tidwall/gjson`, `sjson`)
- YAML parsing (from `gopkg.in/yaml.v3`)
- TOML parsing (from `github.com/pelletier/go-toml/v2`)
- Path parsing and manipulation

**What Does NOT Go Here**:
- Functions that depend on internal packages (runtime, connector, spec, common)
- Functions that need k8s.io or other external libraries (besides parsing ones)

**Usage**:
```go
import "github.com/mensylisir/kubexm/internal/tool"

// All functions are directly exported, no interfaces needed
data, err := tool.JsonToMap(jsonBytes)
config, err := tool.GetTomlValue(tomlData, "key")
content, err := tool.SetYamlValue(yamlData, "key.path", value)
```

### Utility Package (internal/util)

**Purpose**: General utilities, some may depend on `runtime`

**What Goes Here**:
- File operations that need runtime context
- Image operations that need cluster config
- Helper functions that need execution context

**What Does NOT Go Here**:
- Pure parsing utilities (put these in `internal/tool`)
- Pure string/array operations (put these in `internal/tool`)

**Usage**:
```go
import "github.com/mensylisir/kubexm/internal/util"
import "github.com/mensylisir/kubexm/internal/runtime"

// Delegates to internal/tool for pure parsing
import tool "github.com/mensylisir/kubexm/internal/tool"

data, err := tool.JsonToMap(jsonBytes)

// Directly uses runtime for execution
func UploadFile(ctx runtime.ExecutionContext, ...) error {
    // ...
}
```

### Runner Helpers Package (internal/runner/helpers)

**Purpose**: Compatibility layer to delegate to `internal/tool`

**What Goes Here**:
- Wrapper functions that delegate to `internal/tool` functions
- Maintains backward compatibility

**What Does NOT Go Here**:
- Core operation logic (put in `internal/runner/*.go`)
- Functions that need runtime context directly (use `internal/util` instead)

**Usage**:
```go
import "github.com/mensylisir/kubexm/internal/runner/helpers"

// Delegates to internal/tool
func JsonToMap(jsonData []byte) (map[string]interface{}, error) {
    return tool.JsonToMap(jsonData)
}
```

---

## Directory Scoring Matrix

When adding new AGENTS.md files to subdirectories, use this scoring system:

### Scoring Formula
```
Score = (File Count × 3) + (Subdir Count × 2) + (Domain Distinctiveness × 5) + (Reference Centrality × 2)
```

### Decision Rules
- **Score >100**: Must create AGENTS.md (critical infrastructure)
- **Score 50-100**: Should create AGENTS.md (high priority)
- **Score 15-50**: Consider creating AGENTS.md (medium priority)
- **Score <15**: Skip (covered by parent or not needed)

### Reference Centrality Analysis

Based on codebase analysis, these packages have highest reference centrality:

| Package | References | Priority |
|---------|-----------|----------|
| internal/runtime | 670 | CRITICAL |
| internal/spec | 629 | CRITICAL |
| internal/step | 525 | HIGH |
| internal/common | 400 | HIGH |
| internal/plan | 154 | MEDIUM |
| internal/task | 147 | MEDIUM |
| internal/connector | 126 | MEDIUM |
| internal/apis | 73 | MEDIUM |
| internal/templates | 63 | LOW |

### Current AGENTS.md Coverage

| Directory | Files | Score | Status |
|-----------|-------|-------|--------|
| internal/step | 527 | 1649 | ✅ Existing |
| internal/task | 130 | 438 | ✅ Existing |
| internal/runtime | 13 | ~750 | 🔲 Needed (highest reference centrality) |
| internal/spec | 1 | ~650 | 🔲 Needed (high reference centrality) |
| internal/common | 37 | 116 | 🔲 Needed |
| internal/apis | 35 | 112 | 🔲 Needed |
| internal/module | 22 | 91 | 🔲 Needed |
| internal/runner | 26 | 87 | 🔲 Needed |
| internal/util | 45 | 148 | 🔲 Needed |
| internal/connector | 15 | 48 | 🔲 Consider |
| internal/pipeline | 5 | 20 | 🔲 Consider |
| internal/plan | 5 | 17 | 🔲 Consider |
| internal/cache | 8 | 26 | 🔲 Consider |
| internal/tool | 4 | 13 | ✅ Covered (root) |

This file provides comprehensive guidelines for AI coding agents. Follow these conventions to maintain code quality and consistency across the kubexm codebase.

# cc-connect Integration

This project is managed via cc-connect, a bridge to messaging platforms.

## Scheduled tasks (cron)

When the user asks you to do something on a schedule (e.g. "every day at 6am", "every Monday morning"), use the Bash/shell tool to run:

```
cc-connect cron add --cron " " --prompt "" --desc ""
```

Environment variables `CC_PROJECT` and `CC_SESSION_KEY` are already set — do NOT specify `--project` or `--session-key`.

Examples:

```
cc-connect cron add --cron "0 6 * * *" --prompt "Collect GitHub trending repos and send a summary" --desc "Daily GitHub Trending"
cc-connect cron add --cron "0 9 * * 1" --prompt "Generate a weekly project status report" --desc "Weekly Report"
```

To list or delete cron jobs:

```
cc-connect cron list
cc-connect cron del
```

## Send message to current chat

To proactively send a message back to the user's chat session (use --stdin heredoc for long/multi-line messages):

```
cc-connect send --stdin <<'CCEOF'
your message here (any special characters are safe)
CCEOF
```

For short single-line messages:

```
cc-connect send -m "short message"
```
