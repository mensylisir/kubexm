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

// Default SSH connection parameters.
const (
	DefaultSSHPort           = 22
	DefaultConnectionTimeout = 30 * time.Second
)

// Default and supported architectures.
const DefaultArch = "amd64"

// SupportedArches lists the CPU architectures supported by Kubexm.
var SupportedArches = []string{"amd64", "arm64"}

// Host types for connection.
const (
	HostTypeSSH   = "ssh"   // Indicates an SSH connection to a remote host.
	HostTypeLocal = "local" // Indicates operations are to be performed on the local machine.
)

// ValidTaintEffects lists the valid effects for Kubernetes node taints.
var ValidTaintEffects = []string{"NoSchedule", "PreferNoSchedule", "NoExecute"}

// DomainValidationRegexString is used for validating domain names.
const DomainValidationRegexString = `^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`

// Default values for various components and settings.
const (
	DefaultKubernetesAPIServerPort = 6443      // Default secure port for Kubernetes API server.
	DefaultK8sVersion              = "v1.28.2" // Example default Kubernetes version.
	DefaultImageRegistry           = "registry.k8s.io" // Default image registry for Kubernetes components.
	PauseImageName                 = "pause"           // Name of the pause image.
	DefaultEtcdVersion             = "3.5.10-0"        // Example default Etcd version.
	DefaultCoreDNSVersion          = "v1.10.1"         // Example default CoreDNS version.
	DefaultContainerdVersion       = "1.7.11"          // Example default Containerd version.

	DefaultKeepalivedAuthPass   = "kxm_pass"           // Default auth password for Keepalived.
	DefaultHAProxyFrontendPort  = 6443                 // Default frontend port for HAProxy for K8s API.
	DefaultHAProxyMode          = "tcp"                // Default mode for HAProxy.
	DefaultHAProxyAlgorithm     = "roundrobin"         // Default load balancing algorithm for HAProxy.
	DefaultKubeletHairpinMode   = "promiscuous-bridge" // Default hairpin mode for Kubelet.
	DefaultLocalRegistryDataDir = "/var/lib/registry"    // Default data directory for a locally deployed image registry.
	DefaultRemoteWorkDir        = "/tmp/kubexms_work"    // Default working directory on remote hosts.

	DefaultWorkDirName = ".kubexm" // Default name for the main work directory on the control machine (within the execution path, e.g., $(pwd)/.kubexm).
	DefaultTmpDirName  = ".kubexm_tmp" // Default name for a temporary directory under the system's temp path on the control machine.

	// Socket paths for container runtimes.
	ContainerdSocketPath = "unix:///run/containerd/containerd.sock" // Default socket path for Containerd.
	DockerSocketPath     = "unix:///var/run/docker.sock"             // Default socket path for Docker.
	CriDockerdSocketPath = "/var/run/cri-dockerd.sock"               // Default socket path for cri-dockerd.

	// Essential Kernel Modules for Kubernetes.
	KernelModuleBrNetfilter = "br_netfilter" // Kernel module for bridge netfilter.
	KernelModuleIpvs        = "ip_vs"        // Kernel module for IPVS.

	// Default preflight check values.
	DefaultMinCPUCores = 2            // Default minimum CPU cores required.
	DefaultMinMemoryMB = uint64(2048) // Default minimum memory in MB (2GB).

	// Cgroup driver names.
	CgroupDriverSystemd  = "systemd"
	CgroupDriverCgroupfs = "cgroupfs"

	// KubeProxy modes.
	KubeProxyModeIPTables = "iptables"
	KubeProxyModeIPVS     = "ipvs"

	// CNI plugin names (string values; typed versions are in types.go).
	CNICalicoStr    = "calico"
	CNIFlannelStr  = "flannel"
	CNICiliumStr    = "cilium"
	CNIMultusStr    = "multus"
	CNIKubeOvnStr   = "kube-ovn"
	CNIHybridnetStr = "hybridnet"

	// Container Runtime names (string values; typed versions are in types.go).
	RuntimeDockerStr     = "docker"
	RuntimeContainerdStr = "containerd"
	RuntimeCRIOStr       = "cri-o"
	RuntimeIsulaStr      = "isula"

	// Special Role Names used internally.
	AllHostsRole        = "all"                 // Represents all hosts in the inventory for targeting operations.
	ControlNodeRole     = "control-node"        // Represents the machine where kubexm CLI is running, for local operations.
	ControlNodeHostName = "kubexm-control-node" // Special hostname for the control machine.

	// Docker specific defaults for daemon configuration.
	DockerDefaultDataRoot               = "/var/lib/docker" // Default data directory for Docker.
	DockerLogOptMaxSizeDefault          = "100m"            // Default max size for Docker log files.
	DockerLogOptMaxFileDefault          = "5"               // Default max number of log files for Docker.
	DockerMaxConcurrentDownloadsDefault = 3                 // Default max concurrent image downloads for Docker.
	DockerMaxConcurrentUploadsDefault   = 5                 // Default max concurrent image uploads for Docker.
	DefaultDockerBridgeName             = "docker0"         // Default bridge name for Docker.
	DockerLogDriverJSONFile             = "json-file"
	DockerLogDriverJournald             = "journald"
	DockerLogDriverSyslog               = "syslog"
	DockerLogDriverFluentd              = "fluentd"
	DockerLogDriverNone                 = "none"

	// Keepalived specific defaults.
	KeepalivedAuthTypePASS        = "PASS"                               // Default authentication type for Keepalived.
	KeepalivedAuthTypeAH          = "AH"                                 // AH authentication type for Keepalived.
	DefaultKeepalivedPreempt      = true                                 // Default preempt mode for Keepalived.
	DefaultKeepalivedCheckScript  = "/etc/keepalived/check_apiserver.sh" // Example health check script path for Keepalived.
	DefaultKeepalivedInterval     = 5                                    // Default health check interval for Keepalived.
	DefaultKeepalivedRise         = 2                                    // Default rise count for Keepalived health check.
	DefaultKeepalivedFall         = 2                                    // Default fall count for Keepalived health check.
	DefaultKeepalivedAdvertInt    = 1                                    // Default advertisement interval for Keepalived.
	DefaultKeepalivedLVScheduler  = "rr"                                 // Default LVS scheduler for Keepalived.

	// Nginx LoadBalancer specific defaults.
	DefaultNginxListenPort     = 6443                            // Default listen port for Nginx LB.
	DefaultNginxMode           = "stream"                          // Default mode for Nginx LB (TCP).
	DefaultNginxAlgorithm      = "round_robin"                     // Default load balancing algorithm for Nginx.
	DefaultNginxConfigFilePath = "/etc/nginx/nginx.conf"         // Default config file path for Nginx.

	// KubeVIP specific defaults.
	DefaultKubeVIPImage = "ghcr.io/kube-vip/kube-vip:v0.7.0" // Example default Kube-VIP image.

	// DNS specific defaults.
	DefaultCoreDNSUpstreamGoogle     = "8.8.8.8"     // Google's public DNS.
	DefaultCoreDNSUpstreamCloudflare = "1.1.1.1"     // Cloudflare's public DNS.
	DefaultExternalZoneCacheSeconds  = 300           // Default cache time for external DNS zones.

	// Etcd specific paths and names.
	EtcdDefaultPKIDir               = "/etc/kubernetes/pki/etcd" // Standard path for etcd PKI assets when managed by kubeadm.
	EtcdDefaultDataDir              = "/var/lib/etcd"            // Default data directory for Etcd.
	CACertFileName                  = "ca.pem"                   // Common name for CA certificate file (PEM format).
	EtcdServerCertFileName          = "server.pem"               // Default Etcd server certificate filename.
	EtcdServerKeyFileName           = "server-key.pem"           // Default Etcd server key filename.
	EtcdPeerCertFileName            = "peer.pem"                 // Default Etcd peer certificate filename.
	EtcdPeerKeyFileName             = "peer-key.pem"             // Default Etcd peer key filename.
	EtcdClientCertFileName          = "client.pem"               // Default Etcd client certificate filename (generic).
	EtcdClientKeyFileName           = "client-key.pem"           // Default Etcd client key filename (generic).
	EtcdDefaultBinDir               = "/usr/local/bin"           // Default directory for Etcd binaries.
	DefaultEtcdVersionForBinInstall = "v3.5.13"                  // Default Etcd version for binary installations.

	// Containerd specific constants.
	ContainerdPluginCRI = "cri" // Name of the CRI plugin for Containerd.

	// System Configuration Defaults.
	DefaultSELinuxMode  = "permissive" // Default SELinux mode.
	DefaultIPTablesMode = "legacy"     // Default IPTable mode.
)

// Valid System Configuration Values for certain string enum-like fields.
var (
	ValidSELinuxModes  = []string{"permissive", "enforcing", "disabled", ""} // Empty allows no-op/system default
	ValidIPTablesModes = []string{"legacy", "nft", ""}                      // Empty allows no-op/system default
)

// Default File Permissions used across the application.
const (
	DefaultDirPermission        = os.FileMode(0755) // Default permission for directories created by Kubexm.
	DefaultFilePermission       = os.FileMode(0644) // Default permission for files created by Kubexm.
	DefaultKubeconfigPermission = os.FileMode(0600) // Restricted permission for Kubeconfig files.
	DefaultPrivateKeyPermission = os.FileMode(0600) // Restricted permission for private key files.
)
