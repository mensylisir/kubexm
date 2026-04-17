package dep

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func getValue(data []byte, key string) string {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key) {
			return strings.TrimPrefix(line, key)
		}
	}
	return ""
}

// OSType represents the operating system family.
type OSType string

const (
	OSTypeCentOS    OSType = "centos"
	OSTypeRocky     OSType = "rocky"
	OSTypeRHEL      OSType = "rhel"
	OSTypeUOS       OSType = "uos"
	OSTypeAnolis    OSType = "anolis"
	OSTypeOpenEuler OSType = "openeuler"
	OSTypeKylin     OSType = "kylin"
	OSTypeAlmaLinux OSType = "almalinux"
	OSTypeOracle    OSType = "oracle"
	OSTypeFedora    OSType = "fedora"
	OSTypeUbuntu    OSType = "ubuntu"
	OSTypeDebian    OSType = "debian"
	OSTypeUnknown   OSType = "unknown"
)

// PackageManager represents the package manager type.
type PackageManager string

const (
	PackageManagerYUM PackageManager = "yum"
	PackageManagerDNF PackageManager = "dnf"
	PackageManagerAPT PackageManager = "apt"
)

// DetectOS detects the current operating system.
func DetectOS() (OSType, string) {
	// Linux detection
	if runtime.GOOS != "linux" {
		return OSTypeUnknown, ""
	}

	// Try reading os-release
	for _, f := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		data, err := readFile(f)
		if err == nil {
			id := getValue(data, "ID=")
			versionID := getValue(data, "VERSION_ID=")
			versionID = strings.Trim(versionID, "\"")

			switch strings.ToLower(id) {
			case "centos":
				return OSTypeCentOS, versionID
			case "rocky":
				return OSTypeRocky, versionID
			case "rhel", "redhat":
				return OSTypeRHEL, versionID
			case "uos", "uniontech", "uniontecos":
				// UOS 20 Server / 统信 UOS (龙芯/飞腾版，基于 RPM)
				return OSTypeUOS, versionID
			case "anolis":
				// 龙蜥 Anolis OS (RPM)
				return OSTypeAnolis, versionID
			case "openeuler":
				// 华为 openEuler (RPM)
				return OSTypeOpenEuler, versionID
			case "kylin":
				// 麒麟 Linux (RPM)
				return OSTypeKylin, versionID
			case "almalinux":
				// AlmaLinux (RPM)
				return OSTypeAlmaLinux, versionID
			case "ol", "oraclelinux":
				// Oracle Linux (RPM)
				return OSTypeOracle, versionID
			case "fedora":
				// Fedora (RPM, always dnf)
				return OSTypeFedora, versionID
			case "ubuntu":
				return OSTypeUbuntu, versionID
			case "debian":
				return OSTypeDebian, versionID
			}
		}
	}

	return OSTypeUnknown, ""
}

// IsRPM returns true if the OS uses RPM packages.
func (o OSType) IsRPM() bool {
	return o == OSTypeCentOS || o == OSTypeRocky || o == OSTypeRHEL ||
		o == OSTypeUOS || o == OSTypeAnolis || o == OSTypeOpenEuler ||
		o == OSTypeKylin || o == OSTypeAlmaLinux || o == OSTypeOracle || o == OSTypeFedora
}

// IsDEB returns true if the OS uses DEB packages.
func (o OSType) IsDEB() bool {
	return o == OSTypeUbuntu || o == OSTypeDebian
}

// String returns the string representation.
func (o OSType) String() string {
	return string(o)
}

// String returns the string representation of PackageManager.
func (p PackageManager) String() string {
	return string(p)
}

// Package represents a single package with metadata.
type Package struct {
	Name            string   // Package name
	Version        string   // Package version
	Architecture   string   // Architecture (x86_64, aarch64, etc.)
	Repository     string   // Source repository
	DownloadURL    string   // Direct download URL (if available)
	Dependencies   []string // Direct dependencies (for display)
	IsInstalled    bool     // Whether already installed
	LocalPath      string   // Local file path after download
	SHA256         string   // SHA256 checksum
	Size           int64    // Package size in bytes
}

// PackageList is a collection of packages with O(1) deduplication.
// Internally uses a map for fast lookups and a slice for ordered iteration.
type PackageList struct {
	pkgs []*Package
	idx  map[string]*Package // keyed by "name/arch"
}

// NewPackageList creates an empty PackageList.
func NewPackageList() PackageList {
	return PackageList{
		pkgs: []*Package{},
		idx:  make(map[string]*Package),
	}
}

func pkgKey(p *Package) string {
	return p.Name + "/" + p.Architecture
}

// Add adds a package if not already present (O(1)).
func (pl *PackageList) Add(pkg *Package) {
	key := pkgKey(pkg)
	if _, exists := pl.idx[key]; !exists {
		pl.idx[key] = pkg
		pl.pkgs = append(pl.pkgs, pkg)
	}
}

// Has returns true if the package (name/arch) is in the list (O(1)).
func (pl PackageList) Has(name, arch string) bool {
	_, exists := pl.idx[name+"/"+arch]
	return exists
}

// Slice returns the underlying slice of packages.
func (pl PackageList) Slice() []*Package {
	return pl.pkgs
}

// Len returns the number of packages.
func (pl PackageList) Len() int {
	return len(pl.pkgs)
}

// Names returns all package names.
func (pl PackageList) Names() []string {
	names := make([]string, len(pl.pkgs))
	for i, p := range pl.pkgs {
		names[i] = p.Name
	}
	return names
}

// DepGraph represents the dependency resolution graph.
type DepGraph struct {
	Packages   PackageList      // All resolved packages
	Conflicts  []string         // Package conflicts detected
	Missing    []string         // Packages that couldn't be resolved
	TotalSize  int64            // Total download size
	ByRepo     map[string]int   // Package count per repository
}

// PackageSet represents a set of packages to be collected.
type PackageSet struct {
	// Kubernetes prerequisites (all clusters need these)
	K8sPrereqs []string

	// Container runtime OS dependencies
	RuntimeDeps map[string][]string // runtime type -> packages

	// CNI plugin dependencies
	CNIDeps map[string][]string // plugin name -> packages

	// Load balancer dependencies
	LoadBalancerDeps map[string][]string // lb type -> packages

	// Storage dependencies
	StorageDeps map[string][]string // storage type -> packages

	// Network plugin dependencies
	NetworkDeps map[string][]string // plugin name -> packages

	// System utilities (always needed)
	SystemTools []string

	// Extra packages from config
	ExtraRPMS []string
	ExtraDEBS  []string
}

// NewPackageSet creates a default package set with known dependencies.
func NewPackageSet() *PackageSet {
	return &PackageSet{
		// Kubernetes always needs these on the host
		K8sPrereqs: []string{
			"conntrack",      // kube-proxy, kubelet
			"socat",          // kubelet port forwarding
			"ipset",          // iptables/ipset rules
			"iptables",       // firewall rules
			"ebtables",       // bridge networking
			"curl",           // health checks, downloads
			"wget",           // downloading configs
			"jq",             // JSON processing
			"bash",           // shell scripts
			"tar",            // extracting archives
			"gzip",           // decompression
			"xz-utils",       // xz decompression
			"rsync",          // efficient file transfer
			"iputils-ping",   // ping utility
			"net-tools",      // network diagnostics (netstat, etc.)
			"iproute",        // ip, ss commands
			"util-linux",     // various system utilities
			"hostname",       // hostname utility
		},

		RuntimeDeps: map[string][]string{
			"containerd": {
				"libseccomp",  // seccomp support
				"libgpgme",    // image verification
				"device-mapper",
				"libbtrfs",
			},
			"docker": {
				"libseccomp",
				"libgpgme",
				"device-mapper",
				"libbtrfs",
			},
			"cri-o": {
				"libseccomp",
				"gpgme-devel",
				"device-mapper",
			},
		},

		CNIDeps: map[string][]string{
			"calico": {
				// Calico is mostly container-based, minimal host deps
				"wget", // for calicoctl download
			},
			"cilium": {
				// Cilium needs BPF toolchain
				"clang",      // BPF compilation
				"llvm",       // BPF backend
				"linux-headers",
				"iproute",    // tc command
			},
			"flannel": {
				// Flannel is container-based, minimal deps
				"wget",
			},
			"kubeovn": {
				// OVN needs these
				"openvswitch", // OVN/OVS
				"poptr",       // OVN dependency
			},
			"hybridnet": {
				// Hybridnet needs OVS
				"openvswitch",
			},
		},

		LoadBalancerDeps: map[string][]string{
			"kubexm_kh": { // keepalived + haproxy
				"keepalived",
				"haproxy",
			},
			"kubexm_kn": { // keepalived + nginx
				"keepalived",
				"nginx",
			},
			"haproxy": {
				"haproxy",
			},
			"nginx": {
				"nginx",
			},
		},

		StorageDeps: map[string][]string{
			"nfs": {
				"nfs-utils",      // NFS client
				"nfs4-acl-tools",  // NFS ACL support
			},
			"longhorn": {
				"nfs-utils",       // Longhorn can use NFS backend
				"iscsi-initiator-utils", // iSCSI support
			},
			"openebs": {
				"nfs-utils",
				"iscsi-initiator-utils",
			},
		},

		NetworkDeps: map[string][]string{
			// Additional network-related packages
			"multus": {}, // No extra deps
		},

		SystemTools: []string{
			"bc",            // Calculator
			"nc",            // Netcat
			"telnet",        // Connectivity testing
			"strace",        // Debugging
			"lsof",          // File descriptor monitoring
			"psmisc",        // process management (killall, etc.)
			"sysstat",       // System statistics
			"ipvsadm",       // IPVS load balancer admin
		},
	}
}

// ResolveForCluster returns the packages needed for a specific cluster configuration.
func (ps *PackageSet) ResolveForCluster(cfg *ClusterDepConfig) (*ResolvedDeps, error) {
	resolved := &ResolvedDeps{
		Packages: NewPackageList(),
		ByRepo:  make(map[string][]string),
	}

	// Always add K8s prerequisites
	for _, name := range ps.K8sPrereqs {
		resolved.addPackage(name, cfg.Arch, "base")
	}

	// Add container runtime dependencies
	if cfg.Runtime != "" {
		if deps, ok := ps.RuntimeDeps[cfg.Runtime]; ok {
			for _, name := range deps {
				resolved.addPackage(name, cfg.Arch, "base")
			}
		}
	}

	// Add CNI plugin dependencies
	if cfg.CNIPlugin != "" {
		if deps, ok := ps.CNIDeps[cfg.CNIPlugin]; ok {
			for _, name := range deps {
				resolved.addPackage(name, cfg.Arch, "base")
			}
		}
	}

	// Add load balancer dependencies
	if cfg.LoadBalancer != "" {
		if deps, ok := ps.LoadBalancerDeps[cfg.LoadBalancer]; ok {
			for _, name := range deps {
				resolved.addPackage(name, cfg.Arch, "base")
			}
		}
	}

	// Add storage dependencies
	if cfg.StorageType != "" {
		if deps, ok := ps.StorageDeps[cfg.StorageType]; ok {
			for _, name := range deps {
				resolved.addPackage(name, cfg.Arch, "base")
			}
		}
	}

	// Add network dependencies
	if cfg.NetworkPlugin != "" {
		if deps, ok := ps.NetworkDeps[cfg.NetworkPlugin]; ok {
			for _, name := range deps {
				resolved.addPackage(name, cfg.Arch, "base")
			}
		}
	}

	// Add system tools
	for _, name := range ps.SystemTools {
		resolved.addPackage(name, cfg.Arch, "base")
	}

	// Add extra packages from config
	for _, name := range cfg.ExtraPackages {
		resolved.addPackage(name, cfg.Arch, "extra")
	}

	return resolved, nil
}

// ResolvedDeps holds resolved dependencies.
type ResolvedDeps struct {
	Packages   PackageList
	ByRepo     map[string][]string
	TotalSize  int64
	Unresolved []string
}

func (r *ResolvedDeps) addPackage(name, arch, repo string) {
	if r.Packages.Has(name, arch) {
		return
	}
	r.Packages.Add(&Package{
		Name:          name,
		Architecture:  arch,
		Repository:    repo,
	})
	if r.ByRepo[repo] == nil {
		r.ByRepo[repo] = []string{}
	}
	r.ByRepo[repo] = append(r.ByRepo[repo], name)
}

// ClusterDepConfig holds cluster configuration for dependency resolution.
type ClusterDepConfig struct {
	OS           OSType
	OSVersion    string
	Arch         string
	Runtime      string   // containerd, docker, cri-o
	CNIPlugin    string   // calico, cilium, flannel, kubeovn, hybridnet
	LoadBalancer string   // kubexm_kh, kubexm_kn, haproxy, nginx
	StorageType  string   // nfs, longhorn, openebs
	NetworkPlugin string  // multus, etc.
	ExtraPackages []string // user-specified extra packages
	K8sVersion    string   // kubernetes version (for info only)
}

// String returns a human-readable description.
func (c *ClusterDepConfig) String() string {
	return fmt.Sprintf("OS=%s-%s arch=%s runtime=%s cni=%s lb=%s storage=%s",
		c.OS, c.OSVersion, c.Arch, c.Runtime, c.CNIPlugin, c.LoadBalancer, c.StorageType)
}
