# AGENTS.md - Kubernetes Component Tasks

**Generated:** 2026-01-19
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
This package contains tasks for deploying and configuring Kubernetes components (kubeadm, kubelet, kube-proxy, control plane).

## STRUCTURE
```
pkg/task/kubernetes/
├── kubeadm/              # kubeadm installation and initialization
├── kubelet/              # kubelet deployment
├── kube-proxy/            # kube-proxy deployment
├── kubexm/               # kubexm control plane configuration
└── AGENTS.md               # This file
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| kubeadm bootstrap | `bootstrap_first_master.go` | Initial cluster bootstrapping |
| Control plane config | Multiple subdirs | API server, controller, scheduler |
| Node preparation | `deploy_nodes.go` | Distributing configs and binaries |
| Kubeconfig generation | `generate_node_kubeconfigs.go` | Creating kubeconfigs |

## CONVENTIONS
- Follow parent AGENTS.md for general Go conventions
- Use `ExtendedTask.GetDependencies()` for Kubernetes component tasks
- Create `plan.ExecutionFragment` via `Plan()` method

## ANTI-PATTERNS
- **Never use `EnhancedTask`** - Use `ExtendedTask` (correct interface name)

## UNIQUE STYLES
- Multi-step bootstrap: Generate init config → kubeadm init → distribute configs
- Service lifecycle tasks for control plane components
- Node preparation with credential distribution

## NOTES
- Kubernetes tasks compose steps from `pkg/step/kubernetes/`, `pkg/step/cd/`, `pkg/step/pki/`
- Follow the Kubernetes component ordering: etcd → kubeadm → kubelet → kube-proxy → control plane
