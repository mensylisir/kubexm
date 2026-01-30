# AGENTS.md - Module Composition Layer

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
The module layer composes tasks into cohesive units of work. Modules orchestrate task execution, manage task dependencies, and generate execution fragments for the pipeline engine.

## STRUCTURE
```
pkg/module/
├── addon/              # Add-on deployment modules
├── cni/                # Container network interface modules (Calico, cleanup)
├── containerd/          # Containerd installation and cleanup modules
├── docker/              # Docker installation module
├── etcd/               # etcd cluster setup and cleanup modules
├── infrastructure/       # Core infrastructure (OS, ETCD, container runtime)
├── iscsi/               # iSCSI storage module
├── kubernetes/          # Kubernetes component modules (controlplane, worker, kubelet, cleanup)
├── network/             # Network configuration module
├── preflight/           # Preflight checks and validation
├── interface.go        # Module interface definition
└── module.go           # BaseModule implementation
```

## WHERE TO LOOK
| Type/Concept | Location | Notes |
|--------------|----------|-------|
| Module interface | `interface.go` | Name(), Description(), Tasks(), Plan(), GetBase() |
| BaseModule | `module.go` | Embeds spec.ModuleMeta, Timeout, IgnoreError, ModuleTasks |
| Module planning | All submodules | Plan() creates execution fragments from tasks |
| Fragment composition | All submodules | MergeFragment, LinkFragments, UniqueNodeIDs |
| Task composition | `pkg/task/` | Modules compose tasks from pkg/task |

## CONVENTIONS
- **Every module embeds `module.BaseModule`** for common fields and methods
- **Use `plan.NewExecutionFragment()`** for module fragments (not `task.NewExecutionFragment`)
- **Context assertion**: ModuleContext → TaskContext for task operations (`ctx.(runtime.TaskContext)`)
- **Always call `task.IsRequired()`** before `task.Plan()` to skip unnecessary tasks
- **Deduplicate node IDs** using `plan.UniqueNodeIDs()` for entry/exit nodes
- **Return empty fragments** using `plan.NewEmptyFragment(name)` when no tasks are required

## ANTI-PATTERNS
- **Never use `task.NewExecutionFragment()`** - Use `plan.NewExecutionFragment()` for modules
- **Never manually merge nodes** - Use `plan.MergeFragment()` to combine task fragments
- **Never forget `plan.UniqueNodeIDs()`** - Always deduplicate entry/exit node IDs
- **Never skip `task.IsRequired()` check** - Always check before calling `task.Plan()`
- **Don't assume context type** - Always assert `ctx.(runtime.TaskContext)` before task operations
- **Don't hardcode task dependencies** - Use `plan.LinkFragments()` for dynamic linking

## UNIQUE STYLES
- **Fragment composition**: Create fragment → Assert context → Iterate tasks → Plan → Merge → Link → Set entry/exit
- **Parallel composition**: Addons install in parallel, no linking between them
- **Phased composition**: Infrastructure has sequential OS tasks, then parallel ETCD and Container Runtime phases
- **Dynamic task generation**: AddonsModule generates tasks from cluster config
- **Context hierarchy**: PipelineContext → ModuleContext → TaskContext

## NOTES
- Module execution: Plan() generates fragment → pipeline executes steps
- Fragment management: MergeFragment (merge), LinkFragments (dependencies), UniqueNodeIDs (dedup)
- Module types: Static tasks (fixed list), Dynamic tasks (from config), Cleanup modules (resource removal)
- Always wrap errors with context: `fmt.Errorf("failed to plan task %s in module %s: %w", task.Name(), m.Name(), err)`
- Use contextual logging: `logger := ctx.GetLogger().With("module", m.Name())`
