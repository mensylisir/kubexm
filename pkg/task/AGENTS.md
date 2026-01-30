# AGENTS.md - Kubernetes Automation Layer

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
Kubexm is a Kubernetes deployment automation tool with a layered architecture. This document focuses on the task and module orchestration layer that composes steps into workflows.

## STRUCTURE
```
pkg/task/
├── addon/              # Add-on deployment tasks (Helm, manifests)
├── cd/                 # Continuous deployment (ArgoCD)
├── cni/                # Container network interface tasks (Calico, Flannel, Cilium, etc.)
├── containerd/          # Containerd installation and configuration
├── crio/               # CRI-O installation and configuration
├── dns/                 # DNS configuration (CoreDNS, NodeLocalDNS)
├── docker/              # Docker runtime setup
├── etcd/               # etcd cluster tasks
├── gateway/             # Ingress gateway tasks (HAProxy, Nginx)
├── kubernetes/          # Kubernetes component tasks (kubeadm, kubexm)
├── loadbalancer/         # Load balancer tasks (HAProxy, Nginx, Keepalived, kube-vip)
├── network/             # Network plugin tasks (multus, hybridnet, kubeovn)
├── os/                  # Operating system preparation (swap, SELinux, firewall, kernel)
├── packages/            # Package management tasks
├── pki/                 # PKI/certificate management (etcd, kubeadm, kubexm)
├── pre/                 # Preflight checks and preparation
├── preflight/           # Preflight validation tasks
├── registry/             # Container registry tasks (Harbor, Docker Registry)
└── storage/             # Storage provisioning (Longhorn, NFS, OpenEBS, Local PV)
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Task composition | `interface.go`, `extended.go` | Task interface and ExtendedTask for dependencies |
| Builder pattern | All task subdirs | `New*Builder` functions create `*Builder[T, *Step]` |
| Module composition | `pkg/module/` | Composes tasks into modules |

## CONVENTIONS
Follow the parent AGENTS.md for general Go conventions. This document focuses on task-specific patterns.

## ANTI-PATTERNS
- **Never use `EnhancedTask`** - Use `ExtendedTask` (correct interface name)
- **Always handle Builder.Build() return values** - Most builders return `(Builder, error)` or `(Builder, Step)`
- **Task dependencies** - Use `ExtendedTask.GetDependencies()` only on tasks that implement the interface

## UNIQUE STYLES
- **Builder pattern**: Every task has `New*Builder()` → `Builder.With*()` → `Builder.Build()`
- **Step composition**: Tasks create `plan.ExecutionFragment` via `Plan()` method
- **Runtime context**: Tasks use `runtime.TaskContext` (not `runtime.ExecutionContext`)

## NOTES
- Task execution goes through: `Task.Plan()` → creates `plan.ExecutionFragment` → engine executes steps
- Builders follow: `Init()` → `With*()` (chained) → `Build()` (creates step)
- Use `ExtendedTask` for tasks with dependencies, plain `Task` for independent tasks
