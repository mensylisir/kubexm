# AGENTS.md - Spec Types Layer

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
The spec package defines metadata types used across all layers (step, task, module, pipeline) for consistent configuration and logging.

## STRUCTURE
```
pkg/spec/
└── spec.go            # Meta type definitions (StepMeta, TaskMeta, ModuleMeta, PipelineMeta)
```

## WHERE TO LOOK
| Type | Location | Notes |
|------|----------|-------|
| StepMeta | `spec.go` | Embedded in `step.Base` |
| TaskMeta | `spec.go` | Embedded in `task.Base` |
| ModuleMeta | `spec.go` | Embedded in `module.BaseModule` |
| PipelineMeta | `spec.go` | Embedded in `pipeline.Base` |

## CONVENTIONS
- All Meta types have identical fields: `Name`, `Description`, `Hidden`, `AllowFailure`
- Fields use JSON and YAML tags for serialization
- Use `omitempty` for optional fields
- Embed Meta in base structs: `Base { Meta spec.StepMeta }`

## ANTI-PATTERNS
- **Never add methods to spec types** - These are pure data structures
- **Don't store runtime state in Meta** - Meta is for static configuration only
- **Never modify Meta after creation** - Treat as immutable
- **Don't add validation logic to spec types** - Validation belongs in the layer that uses the type

## UNIQUE STYLES
- **Pure data structures**: No methods, just fields with serialization tags
- **Consistent field names**: All Meta types share the same 4 fields
- **Zero internal dependencies**: pkg/spec can be used anywhere in the codebase

## NOTES
- Used by all layers for consistent logging: `logger.With("step", s.Base.Meta.Name)`
- Step interface requires `Meta() *spec.StepMeta` method
- High reference centrality (629 references) - breaking changes affect the entire codebase
- No tests needed - pure data structures with no logic
