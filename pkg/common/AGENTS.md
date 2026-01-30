# AGENTS.md - Common Shared Types and Constants

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
pkg/common provides shared constants, types, and defaults used across all layers. All packages can import common (no circular dependency issues).

## STRUCTURE
```
pkg/common/
├── cache_keys.go         # Cache key format strings for runtime cache
├── components.go         # Component names, service names, ports
├── directory.go          # All directory paths (work, logs, cache, configs)
├── system.go             # System defaults, permissions, OS/arch types, validation lists
├── images.go             # Image registry and repository defaults
├── dns.go                # DNS configuration constants
├── pki.go                # Certificate file names, permissions, defaults
├── cni.go                # CNI plugin types and network CIDR defaults
├── container_runtime.go  # Container runtime types (Docker, Containerd, CRI-O, Isulad)
├── kubernetes.go         # Kubernetes deployment types, paths, service names
├── etcd.go               # Etcd deployment types, paths, cert names
├── docker.go             # Docker-specific paths, configs, versions
├── containerd.go         # Containerd-specific constants
├── cri-o.go              # CRI-O-specific constants
├── isulad.go             # Isulad-specific constants
├── haproxy.go            # HAProxy modes, algorithms, paths
├── nginx.go              # Nginx load balancer constants
├── keepalived.go         # Keepalived VRRP/LVS constants
├── kube-vip.go           # Kube-VIP constants
├── calico.go             # Calico CNI-specific defaults
├── cilium.go             # Cilium CNI-specific defaults
├── flannel.go            # Flannel CNI-specific defaults
├── kubeovn.go            # Kube-OVN CNI-specific defaults
├── hybridnet.go          # Hybridnet CNI-specific defaults
├── multus.go             # Multus CNI-specific constants
├── loadbalancer.go       # Load balancer types (internal/external)
├── storage.go            # Storage component constants
├── ssh.go                # Host connection types
├── roles_labels.go       # Node roles, labels, taints
├── task.go               # Task category, priority, status enums
├── module.go             # Module execution, phase, status constants
├── pipeline.go           # Pipeline type, mode, status constants
└── version.go            # Version-related constants
```

## WHERE TO LOOK
| What | Location | Notes |
|------|----------|-------|
| All directory paths | `directory.go` | Work dirs, log dirs, cache dirs, system dirs |
| Validation lists | `system.go` | `Valid*` and `Supported*` slices for validation |
| Cache key formats | `cache_keys.go` | Runtime cache key templates |
| Certificate constants | `pki.go` | Cert file names, permissions, defaults |
| Component names/ports | `components.go` | Service names, ports, binary names |
| Default values | Multiple files | Component-specific defaults in each file |
| Type definitions | Throughout | `type X string` enum types |

## CONVENTIONS
- **Constants first**: Define `const` groups before types
- **Type enums**: Use `type X string` with `const` variants
- **DefaultX** prefix: For default values (`DefaultTimeout`, `DefaultPort`)
- **ValidX** prefix: For validation slices (`ValidRuntimeTypes`)
- **XDirTarget** suffix: For target paths on remote hosts (`/etc/etcd`)
- **XFileName** suffix: For file names (`ca.crt`, `server.key`)
- **XServiceName** suffix: For systemd service names (`etcd.service`)

## ANTI-PATTERNS
- **Never add dependencies** - pkg/common has NO imports to internal packages
- **Never add functions with logic** - This is for constants/types only
- **Don't hardcode values** in other packages - Add to pkg/common instead
- **Never import runtime, connector, spec, etc.** - Creates circular dependencies
- **Don't add complex type definitions** - Keep it to simple string enums and constants

## UNIQUE STYLES
- **File-per-domain**: Each major component has its own file (docker.go, etcd.go, etc.)
- **Path organization**: Group paths by target (work dirs, config dirs, runtime dirs)
- **Validation slices**: Use `var ValidX = []string{...}` for validation
- **Pattern constants**: Use `FileNamePattern` for templated names (`node-%s.pem`)

## NOTES
- This package is imported by ALL other packages (runtime, runner, step, task, module)
- No internal dependencies means zero risk of circular imports
- Add new constants here rather than duplicating across packages
- Use `system.go` for cross-cutting validation lists (OS, arch, runtime types)
- Cache keys use `fmt.Sprintf` patterns with placeholders (`%s`)
