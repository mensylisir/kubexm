# AGENTS.md - PKI Management Tasks

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
This package contains PKI/certificate management tasks for etcd, kubeadm, and kubexm clusters.

## STRUCTURE
```
pkg/task/pki/
├── etcd/                  # etcd CA and certificate operations
├── kubeadm/              # kubeadm PKI management
├── kubexm/               # kubexm PKI management
└── AGENTS.md               # This file
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Certificate generation | Multiple subdirs | etcd, kubeadm, kubexm have similar patterns |
| CA operations | `check_certificates_status.go` | Expiration checks and renewal |
| Rollout operations | `rollout_ca.go`, `generate_certs.go` | CA distribution and activation |

## CONVENTIONS
- Follow parent AGENTS.md for general Go conventions
- Use `ExtendedTask.GetDependencies()` for PKI rollout tasks
- Create `plan.ExecutionFragment` via `Plan()` method

## ANTI-PATTERNS
- **Never use `EnhancedTask`** - Use `ExtendedTask` (correct interface name)

## UNIQUE STYLES
- Builder pattern across all subdirectories
- Task execution creates fragments for step composition
- Certificate validation before rollout

## NOTES
- PKI tasks have complex dependencies between etcd, kubeadm, and kubexm
- Follow the same certificate lifecycle: Generate → Distribute → Activate → Cleanup
