# AGENTS.md - Add-on Deployment Tasks

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
This package contains tasks for deploying and managing Kubernetes add-ons using Helm, manifests, and custom operators.

## STRUCTURE
```
pkg/task/addon/
├── install_addon_task.go    # Main add-on installation task
└── AGENTS.md                # This file
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Add-on installation | `install_addon_task.go` | Uses Helm and manifest application |
| Helm integration | Multiple task packages | `helm/` package for chart operations |

## CONVENTIONS
- Follow parent AGENTS.md for general Go conventions
- Use `ExtendedTask.GetDependencies()` for add-ons with dependencies
- Create `plan.ExecutionFragment` via `Plan()` method

## ANTI-PATTERNS
- **Never use `EnhancedTask`** - Use `ExtendedTask` (correct interface name)

## UNIQUE STYLES
- Builder pattern: `NewInstallAddOnTaskBuilder()` → `.WithAddonName()` → `.Build()`
- Task execution: `Task.Plan(ctx)` creates execution fragments

## NOTES
- Add-on tasks compose steps from `pkg/step/addon/`
- Dependencies are resolved by the scheduler before execution
