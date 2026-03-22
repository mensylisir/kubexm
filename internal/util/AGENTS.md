# AGENTS.md - Utility Package

**Generated:** 2026-01-22
**Commit:** kubexm codebase

## OVERVIEW
Runtime-dependent utilities that delegate to `internal/tool` for pure parsing operations. Provides file operations, image/binary management, container runtime utilities, and validation helpers requiring execution context.

## STRUCTURE
```
internal/util/
├── archive.go              # Archive extraction/creation (tar, zip)
├── binaries/               # Binary BOM management & versioning
│   ├── bom.go              # Binary Bill of Materials
│   ├── provider.go         # BinaryProvider: version resolution, checksums
│   └── types.go            # Binary, BinaryDetailSpec types
├── certs.go               # X.509 certificate generation & CA management
├── containerd.go          # ContainerdClient for image operations
├── docker.go              # Docker image handling utilities
├── file.go                # File upload/download with remote execution
├── host.go                # Host range expansion (host[01:10])
├── image_resolver.go      # Image name parsing (registry/repo:tag)
├── images/                # Container image BOM & provider
│   ├── provider.go        # ImageProvider: managed images by config
│   └── types.go           # Image type with registry rewriting
├── json.go                # JSON delegation to internal/tool
├── helm/                  # Helm chart BOM & provider
├── os/                    # OS detection & utilities
├── validation.go          # K8s name, hostname, domain validation
├── yaml.go                # YAML delegation to internal/tool
└── *.go                   # Misc: sha.go, hash.go, template.go, etc.
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| **File upload** | `file.go` | `UploadFile()`, `WriteContentToRemote()` |
| **Image management** | `images/provider.go` | `ImageProvider{GetImage,GetImages}` |
| **Binary management** | `binaries/provider.go` | `BinaryProvider{GetBinary,GetBinaries}` |
| **Certificate handling** | `certs.go` | `NewCertificateAuthority()`, `NewCertFromCA()` |
| **Container runtime** | `containerd.go`, `docker.go` | `ContainerdClient`, Docker utilities |
| **Validation** | `validation.go` | `IsValidK8sName()`, `ValidateHostPortStrict()` |
| **Parsing delegation** | `json.go`, `yaml.go`, `toml.go` | Delegates to `internal/tool` |

## CONVENTIONS
- **Pure parsing → internal/tool**: JSON/YAML/TOML parsing lives in `internal/tool`
- **Runtime context**: Functions needing `ExecutionContext` accept it as first parameter
- **Provider pattern**: Domain utilities use `New*Provider(ctx)` returning `*Provider`
- **BOM for managed resources**: Images and binaries use BOM (Bill of Materials) pattern
- **Error wrapping**: Use `fmt.Errorf("...: %w", err)` for context
- **Thread safety**: Client types (`ContainerdClient`) use mutex for shared state

## ANTI-PATTERNS
- **Don't put pure parsing logic here** → Put in `internal/tool` instead
- **Don't create runtime-dependent helpers in internal/tool** → `tool` must remain pure
- **Don't use global state** → Pass `ExecutionContext` explicitly
- **Don't skip cleanup** → Use `defer` for temporary files in upload operations
- **Don't ignore errors in validation** → Return typed errors, not bools for complex cases

## UNIQUE STYLES
- **Provider encapsulation**: `ImageProvider`, `BinaryProvider` hide BOM complexity
- **Zone-aware downloads**: `GetZone()` checks `KXZONE=cn` for regional binaries
- **Managed image enablement**: `isImageEnabled()` switch on cluster config
- **Checksum verification**: Binary BOM includes arch-specific checksums
- **Registry rewriting**: Image provider applies `Registry.MirroringAndRewriting` config
- **Host range expansion**: `ExpandHostRange()` parses `host[01:10]` patterns

## NOTES
- Delegates to `internal/tool`: `JsonToMap()`, `GetYamlValue()`, `GetTomlValue()`
- `runtime.ExecutionContext` provides: `GetRunner()`, `GetClusterConfig()`, `GetUploadDir()`
- File operations use temporary upload dir + move pattern for atomicity
- Validation regexes are compiled at package level (see `validation.go`)
- Subpackages (`binaries/`, `images/`, `helm/`, `os/`) follow same provider pattern
