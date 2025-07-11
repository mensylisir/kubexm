package common

import (
	"os"
	"time"
)

// This file centralizes miscellaneous constants used throughout the Kubexm application.
// It aims to avoid magic strings and numbers, providing a single source of truth for such values.
// More specific constants (paths, component names, roles) are in their respective files
// (paths.go, components.go, roles_labels.go, types.go).

// General cluster operation types.
// These are now preferably defined with more specific types in types.go (e.g., KubernetesDeploymentType).
// These string constants are kept for broader, general-purpose use or backward compatibility during refactoring.
const (
	// ClusterTypeKubeXM indicates a cluster where core components are deployed as binaries.
	// Prefer using KubernetesDeploymentTypeKubexm from types.go for typed fields.
	ClusterTypeKubeXM = "kubexm"
	// ClusterTypeKubeadm indicates a cluster where core components are deployed via Kubeadm.
	// Prefer using KubernetesDeploymentTypeKubeadm from types.go for typed fields.
	ClusterTypeKubeadm = "kubeadm"
)

// Default SSH connection parameters moved to ssh.go
// Default and supported architectures moved to arch.go
// HostTypeSSH and HostTypeLocal moved to types.go (as HostConnectionType) or covered by roles_labels.go
// ValidTaintEffects moved to roles_labels.go
// DomainValidationRegexString moved to dns.go

// Default values for various components and settings.
const (
	// DefaultKubernetesAPIServerPort moved to components.go
	// DefaultK8sVersion moved to images.go
	// DefaultImageRegistry moved to images.go
	// PauseImageName moved to images.go
	// DefaultEtcdVersion moved to images.go
	// DefaultCoreDNSVersion moved to images.go
	// DefaultContainerdVersion moved to images.go

	// DefaultKeepalivedAuthPass moved to keepalived_defaults.go
	// DefaultHAProxyFrontendPort moved to components.go
	// DefaultHAProxyMode moved to haproxy_defaults.go
	// DefaultHAProxyAlgorithm moved to haproxy_defaults.go
	DefaultKubeletHairpinMode   = "promiscuous-bridge" // Default hairpin mode for Kubelet. // To kubernetes_internal.go
	DefaultLocalRegistryDataDir = "/var/lib/registry"    // Default data directory for a locally deployed image registry. // To paths.go
	// DefaultRemoteWorkDir moved to workdirs.go
	// DefaultWorkDirName moved to workdirs.go
	// DefaultTmpDirName moved to workdirs.go

	// Socket paths for container runtimes.
	// ContainerdSocketPath moved to components.go
	// DockerSocketPath moved to components.go
	// CriDockerdSocketPath moved to components.go

	// Essential Kernel Modules for Kubernetes.
	KernelModuleBrNetfilter = "br_netfilter" // Kernel module for bridge netfilter. // To kubernetes_internal.go
	KernelModuleIpvs        = "ip_vs"        // Kernel module for IPVS. // To kubernetes_internal.go

	// Default preflight check values moved to preflight.go

	// Cgroup driver names.
	CgroupDriverSystemd  = "systemd"  // To kubernetes_internal.go
	CgroupDriverCgroupfs = "cgroupfs" // To kubernetes_internal.go

	// KubeProxy modes.
	KubeProxyModeIPTables = "iptables" // To kubernetes_internal.go
	KubeProxyModeIPVS     = "ipvs"     // To kubernetes_internal.go

	// CNI plugin names (string values; typed versions are in types.go).
	CNICalicoStr    = "calico"    // To cni_strings.go
	CNIFlannelStr  = "flannel"   // To cni_strings.go
	CNICiliumStr    = "cilium"    // To cni_strings.go
	CNIMultusStr    = "multus"    // To cni_strings.go
	CNIKubeOvnStr   = "kube-ovn"  // To cni_strings.go
	CNIHybridnetStr = "hybridnet" // To cni_strings.go

	// Container Runtime names (string values; typed versions are in types.go).
	RuntimeDockerStr     = "docker"     // To runtime_strings.go
	RuntimeContainerdStr = "containerd" // To runtime_strings.go
	RuntimeCRIOStr       = "cri-o"      // To runtime_strings.go
	RuntimeIsulaStr      = "isula"      // To runtime_strings.go

	// Special Role Names used internally. // Moved to roles_labels.go
	AllHostsRole        = "all"
	ControlNodeRole     = "control-node"
	ControlNodeHostName = "kubexm-control-node"

	// Docker specific defaults for daemon configuration moved to docker_defaults.go
	// Keepalived specific defaults moved to keepalived_defaults.go

	// Nginx LoadBalancer specific defaults
	// DefaultNginxListenPort moved to components.go
	// DefaultNginxMode moved to nginx_defaults.go
	// DefaultNginxAlgorithm moved to nginx_defaults.go
	DefaultNginxConfigFilePath = "/etc/nginx/nginx.conf" // Default config file path for Nginx. // To paths.go

	// KubeVIP specific defaults moved to images.go (DefaultKubeVIPImage)

	// DNS specific defaults moved to dns.go

	// Etcd specific paths and names.
	EtcdDefaultPKIDir               = "/etc/kubernetes/pki/etcd" // Standard path for etcd PKI assets when managed by kubeadm. // To paths.go
	EtcdDefaultDataDir              = "/var/lib/etcd"            // Default data directory for Etcd. // To paths.go
	CACertFileName                  = "ca.pem"                   // Common name for CA certificate file (PEM format). // To paths.go or pki.go
	EtcdServerCertFileName          = "server.pem"               // Default Etcd server certificate filename. // To paths.go or pki.go
	EtcdServerKeyFileName           = "server-key.pem"           // Default Etcd server key filename. // To paths.go or pki.go
	EtcdPeerCertFileName            = "peer.pem"                 // Default Etcd peer certificate filename. // To paths.go or pki.go
	EtcdPeerKeyFileName             = "peer-key.pem"             // Default Etcd peer key filename. // To paths.go or pki.go
	EtcdClientCertFileName          = "client.pem"               // Default Etcd client certificate filename (generic). // To paths.go or pki.go
	EtcdClientKeyFileName           = "client-key.pem"           // Default Etcd client key filename (generic). // To paths.go or pki.go
	EtcdDefaultBinDir               = "/usr/local/bin"           // Default directory for Etcd binaries. // To paths.go (but should use DefaultBinDir)
	// DefaultEtcdVersionForBinInstall moved to images.go

	// Containerd specific constants.
	ContainerdPluginCRI = "cri" // Name of the CRI plugin for Containerd. // To components.go
)

// Valid System Configuration Values for certain string enum-like fields. // Moved to system_config.go
// Default File Permissions used across the application. // Moved to permissions.go
