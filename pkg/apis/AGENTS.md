# AGENTS.md - API Definitions Layer

**Generated:** 2026-01-22
**Commit:** Based on kubexm codebase
**Branch:** main

## OVERVIEW
pkg/apis contains CRD (Custom Resource Definition) types and API specifications for kubexm cluster configuration. These types define the schema for cluster manifests, including hosts, roles, networking, container runtimes, Kubernetes components, and add-ons.

## STRUCTURE
```
pkg/apis/
└── kubexms/
    └── v1alpha1/
        ├── doc.go                 # API version and group name
        ├── register.go           # Scheme registration (commented out)
        ├── cluster_types.go       # Core Cluster spec with HostSpec, RoleGroups, Global
        ├── kubernetes_types.go    # Kubernetes components (APIServer, Kubelet, etc.)
        ├── network_types.go       # Network config and CNI plugins (Calico, Cilium, etc.)
        ├── container_runtime_types.go # Container runtime abstraction
        ├── docker_types.go        # Docker configuration
        ├── containerd_types.go    # Containerd configuration
        ├── crio_types.go         # CRI-O configuration
        ├── etcd_types.go         # etcd cluster configuration
        ├── dns_types.go          # DNS configuration
        ├── addon_types.go        # Add-on deployment (Helm charts, YAML manifests)
        ├── storage_types.go      # Storage providers (Longhorn, NFS, etc.)
        ├── registry_types.go     # Registry config (Harbor, Docker Registry)
        ├── *_types.go           # Other component types (CNI, LB, HA, etc.)
        └── helpers/             # Utility functions
            ├── pointer.go        # Helper functions for primitive pointers
            ├── validation.go     # Validation helper functions (k8sName, CIDR, etc.)
            ├── strings.go        # String helpers (ExpandHostRange, ParseCPU/Memory)
            ├── helper.go        # General helpers (GenerateWorkDir, CreateDir)
            └── host.go         # Host-related helpers
```

## WHERE TO LOOK
| Type | Location | Notes |
|------|----------|-------|
| Cluster top-level | `cluster_types.go` | Cluster, ClusterSpec, HostSpec, RoleGroupsSpec, GlobalSpec |
| Kubernetes | `kubernetes_types.go` | APIServerConfig, KubeletConfig, ControllerManagerConfig |
| Network | `network_types.go` | Network, CalicoConfig, CiliumConfig, FlannelConfig |
| Container runtime | `container_runtime_types.go` | ContainerRuntime abstraction for Docker/containerd/crio |
| Addon | `addon_types.go` | Addon, AddonSource, ChartSource, YamlSource |
| Pointer helpers | `helpers/pointer.go` | BoolPtr(), IntPtr(), StrPtr() for optional fields |
| Validation helpers | `helpers/validation.go` | IsValidK8sName(), IsValidCIDR(), etc. |

## CONVENTIONS
- **Type definitions**: All types are plain structs with JSON/YAML tags
- **Optional fields**: Use `*Type` (pointer) for optional fields (e.g., `*bool`, `*int`)
- **Default values**: Use `SetDefaults_*()` functions to initialize defaults
- **Validation**: Use `Validate_*()` functions with `*validation.ValidationErrors`
- **Error reporting**: Use `verrs.Add(path, message)` to report validation errors
- **Helper pointers**: Use `helpers.BoolPtr()`, `helpers.IntPtr()`, etc. to create pointers to primitives
- **Cascading defaults**: SetDefaults_* functions call nested SetDefaults_* recursively

## ANTI-PATTERNS
- **Never use concrete bool/int for optional fields** - Must use `*bool`, `*int` for optional values
- **Don't define default inline** - Use `SetDefaults_*()` functions for all default values
- **Never skip validation** - All types should have corresponding `Validate_*()` functions
- **Don't use direct pointer creation** - Use `helpers.BoolPtr()`, `helpers.IntPtr()` for consistency
- **Never omit json/yaml tags** - All struct fields must have both `json:"" yaml:""` tags
- **Don't put validation logic in SetDefaults** - Separation of concerns: SetDefaults for initialization, Validate for validation

## UNIQUE STYLES
- **Triple pattern per type**: Every type has struct definition, `SetDefaults_*()`, and `Validate_*()`
- **Helper pointer functions**: All primitive pointer creation uses helpers package (e.g., `helpers.BoolPtr(true)`)
- **Path-based validation errors**: `verrs.Add("spec.hosts[0].address", "cannot be empty")`
- **Validation helpers**: Extensive helper functions in `helpers/validation.go` for common checks (k8sName, CIDR, URL, etc.)
- **Runtime-specific types**: Container runtime, CNI, and load balancer types use `type` field with switch statements
- **Nested validation**: Validate_* functions recursively validate nested structs

## NOTES
- Controller implementation is commented out (register.go) - focus is on functionality first
- API group is `kubexms.io`, version is `v1alpha1`
- Uses k8s.io/apimachinery for TypeMeta and ObjectMeta
- Defaults cascade: Cluster → ClusterSpec → nested structs (Kubernetes, Network, etc.)
- All Addon, ChartSource, etc. sources support both Helm charts and YAML manifests
- Container runtime abstraction supports Docker, containerd, CRI-O, and Isulad
- Network plugin abstraction supports Calico, Cilium, Flannel, Kube-OVN, Hybridnet, and Multus
