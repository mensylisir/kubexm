# AGENTS.md - Runner Operations Layer

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
The runner layer provides the operations interface for executing commands, file transfers, service management, and container orchestration. All operations delegate to connector for actual execution.

## STRUCTURE
```
pkg/runner/
├── interface.go        # Runner interface (200+ methods - god interface)
├── runner.go          # defaultRunner implementation, fact gathering
├── command.go         # Command execution (Run, MustRun, Check, RunRetry)
├── file.go            # File operations (Upload, Fetch, ReadFile, WriteFile)
├── service.go         # Service management (Start, Stop, Enable, Disable)
├── docker.go          # Docker container operations
├── containerd.go      # Containerd operations (ctr commands)
├── kubectl.go         # Kubernetes operations via kubectl
├── helm.go            # Helm package manager operations
├── qemu.go            # QEMU/libvirt VM operations
├── network.go         # Network configuration
├── system.go          # System-level operations
├── user.go            # User and group management
├── template.go        # Template rendering
├── package.go         # Package management
├── archive.go         # Archive operations
├── result.go          # Result types (RunnerResult, CommandResult, FileResult)
├── extended.go        # Extended operations
├── helpers/           # Delegates to pkg/tool (ParseCPU, ParseMemory, etc.)
└── loadbalancer/     # Load balancer operations
```

## WHERE TO LOOK
| Component | Location | Notes |
|-----------|----------|-------|
| Runner interface | `interface.go` | 200+ methods - god interface anti-pattern |
| Implementation | `runner.go` | defaultRunner, GatherFacts, fact detection |
| Command operations | `command.go` | Run, MustRun, Check, RunRetry, RunInBackground |
| File operations | `file.go` | Upload, Fetch, Exists, ReadFile, WriteFile, Mkdirp |
| Container operations | `docker.go` | PullImage, CreateContainer, StartContainer |
| Containerd operations | `containerd.go` | CtrListImages, CtrRunContainer |
| Kubernetes operations | `kubectl.go` | KubectlApply, KubectlGet, KubectlExec |
| Helm operations | `helm.go` | HelmInstall, HelmUninstall, HelmList |
| Result types | `result.go` | RunnerResult, CommandResult, FileResult, ServiceResult |
| Helpers | `helpers/` | ParseCPU, ParseMemory, ParseStorage - delegates to pkg/tool |

## CONVENTIONS
- **Method signature**: All runner methods receive `(ctx context.Context, conn connector.Connector, ...)`
- **Connector delegation**: Use `conn.Exec()`, `conn.Stat()`, `conn.Upload()`, `conn.Fetch()`, etc.
- **Error wrapping**: Use `errors.Wrapf(err, "failed to X: %w", ...)`
- **Input validation**: Always check `conn == nil` first, return descriptive errors
- **Options pattern**: Use `*connector.ExecOptions` for passing timeout, sudo, etc.
- **Sudo handling**: Methods accept `sudo bool` parameter to indicate privilege escalation

## ANTI-PATTERNS
- **Don't create new Runner interfaces** - The existing Runner interface is already 200+ methods (god interface)
- **Don't implement command execution directly** - Always delegate to `conn.Exec()`
- **Don't skip input validation** - Always validate `conn` and required parameters first
- **Don't ignore errors silently** - Wrap errors with context before returning

## UNIQUE STYLES
- **Fact gathering**: Uses `errgroup` for parallel fact collection in `GatherFacts()`
- **Sudo caching**: `DetermineSudo()` caches directory writeability results in `sudoCache`
- **Operation categories**: Organized by domain (command, file, service, container, k8s, vm)
- **Result types**: Lightweight result types (`RunnerResult`, `CommandResult`, `FileResult`) separate from Step layer
- **OS-specific logic**: Fact gathering switches commands based on OS ID (linux, darwin)

## NOTES
- The `Runner` interface is intentionally broad (god interface) - it's the entry point for all operations
- `helpers/` package delegates to `pkg/tool` for pure utility functions
- Most operations don't use `runtime.ExecutionContext` - they receive `connector.Connector` directly
- Fact gathering detects package managers (apt, yum, dnf) and init systems (systemd, sysvinit) automatically
- Docker operations use specific timeout constants defined at file level (DefaultDockerPullTimeout, etc.)
