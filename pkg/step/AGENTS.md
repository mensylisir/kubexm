# AGENTS.md - Step Implementation Layer

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
The step layer contains atomic, idempotent operations that form the building blocks of task workflows.

## STRUCTURE
```
pkg/step/
├── addon/              # Add-on deployment steps
├── cd/                 # Continuous deployment steps
├── chrony/             # Time synchronization
├── cni/                # Container network interface
├── command/             # Generic command execution
├── common/              # Shared step implementations (upload, render, packages)
├── dns/                 # DNS configuration
├── etcd/                # etcd operations
├── harbor/              # Harbor registry
├── helpers/             # Helper functions (bom/)
├── kubernetes/          # Kubernetes component steps
├── os/                  # OS preparation
├── pki/                 # PKI and certificates
├── registry/            # Registry operations
└── repository/          # Repository setup
```

## WHERE TO LOOK
| Step | Location | Notes |
|------|----------|-------|
| Step interface | `interface.go` | Core Step interface with Run, Validate, Cleanup, Rollback |
| Base step | `base.go` | Default implementations for Cleanup, Rollback, GetStatus |
| Helpers | `helpers/bom/` | Binary and BOM helpers for etcd, harbor |

## CONVENTIONS
- All steps implement the `Step` interface from `step.Step`
- Use `step.Base` embedding for common fields (Meta, Timeout, Sudo, IgnoreError)
- Run() returns `error` (not `*StepResult, error`)

## ANTI-PATTERNS
- **Never return `(*StepResult, error)` from `Run()`** - Must return only `error`
- **Don't create `StepResult` in Run()** - The engine handles result tracking
- **Never skip Cleanup() implementation** - Even if no-op, implement it (use Base default)

## UNIQUE STYLES
- **Builder pattern**: `New*StepBuilder(ctx, name)` → `With*()` chained → `Init()` → `Build()`
- **Idempotency**: All steps should be safe to run multiple times
- **Precheck**: Use `Precheck()` to skip unnecessary execution

## NOTES
- Embed `step.Base` to get default implementations of Cleanup(), Rollback(), GetStatus(), Validate()
- Override specific methods only if your step has special needs
- Most steps should return `error` from `Run()`, not create and return `*StepResult`
