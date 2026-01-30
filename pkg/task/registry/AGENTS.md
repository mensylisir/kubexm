# AGENTS.md - Registry Tasks

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
This package contains tasks for setting up container registry services (Harbor, Docker Registry).

## STRUCTURE
```
pkg/task/registry/
├── registry/              # Generic Docker registry tasks
├── harbor/               # Harbor-specific tasks
└── AGENTS.md               # This file
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Harbor operations | Multiple task files | Download, extract, configure, install Harbor |
| Docker registry | Clean, install, configure | Docker Registry setup |
| Service management | Start, stop, enable, disable | Service lifecycle management |

## CONVENTIONS
- Follow parent AGENTS.md for general Go conventions
- Use `ExtendedTask.GetDependencies()` for registry setup tasks
- Create `plan.ExecutionFragment` via `Plan()` method

## ANTI-PATTERNS
- **Never use `EnhancedTask`** - Use `ExtendedTask` (correct interface name)

## UNIQUE STYLES
- Builder pattern for registry service tasks
- Multi-stage installation: Download → Extract → Configure → Install → Start
- Service lifecycle tasks for cleanup

## NOTES
- Harbor tasks use `pkg/step/harbor/` steps
- Registry tasks compose steps from `pkg/step/registry/`
