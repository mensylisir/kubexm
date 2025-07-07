package common

import (
	"os"
	"time"
)

const (
	DefaultAddonNamespace = "kube-system"

	// Addon related constants
	ValidChartVersionRegexString = `^v?([0-9]+)(\.[0-9]+){0,2}$` // Allows "latest", "stable", or versions like "1.2.3", "v1.2.3", "1.2", "v1.0", "1", "v2".

	// Endpoint related constants
	DomainValidationRegexString = `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`

	// Kubernetes related constants
	CgroupDriverSystemd  = "systemd"
	CgroupDriverCgroupfs = "cgroupfs"
	KubeProxyModeIPTables = "iptables"
	KubeProxyModeIPVS    = "ipvs"

	// Cluster Types
	ClusterTypeKubeXM  = "kubexm"
	ClusterTypeKubeadm = "kubeadm"

	// --- Status Constants ---
	StatusPending    = "Pending"
	StatusProcessing = "Processing"
	StatusSuccess    = "Success"
	StatusFailed     = "Failed"

	// --- Node Conditions ---
	// NodeConditionReady is a string type from corev1.NodeConditionType, defined here for convenience if not importing corev1.
	NodeConditionReady = "Ready"

	// --- CNI Plugin Names ---
	CNICalico   = "calico"
	CNIFlannel  = "flannel"
	CNICilium   = "cilium"
	CNIMultus   = "multus"
	// Add other CNI plugin names as needed, e.g. KubeOvn, Hybridnet

	// --- Cilium Mode Constants ---
	CiliumTunnelModeVXLAN     = "vxlan"
	CiliumTunnelModeGeneve    = "geneve"
	CiliumTunnelModeDisabled  = "disabled"

	CiliumKPRModeProbe    = "probe"
	CiliumKPRModeStrict   = "strict"
	CiliumKPRModeDisabled = "disabled"

	CiliumIdentityModeCRD     = "crd"
	CiliumIdentityModeKVStore = "kvstore"

	// --- Kernel Modules (consider moving to a system_constants.go if it grows) ---
	KernelModuleBrNetfilter = "br_netfilter"
	KernelModuleIpvs        = "ip_vs"

	// --- Preflight Defaults (consider moving to a preflight_constants.go or config_defaults.go) ---
	DefaultMinCPUCores   = 2
	DefaultMinMemoryMB   = 2048 // 2GB

	// --- Cache Key Constants ---
	// CacheKeyHostFactsPrefix is the prefix for caching host facts.
	CacheKeyHostFactsPrefix = "facts.host."
	// CacheKeyClusterCACert is the key for the cluster CA certificate.
	CacheKeyClusterCACert = "pki.ca.cert"
	// CacheKeyClusterCAKey is the key for the cluster CA key.
	CacheKeyClusterCAKey = "pki.ca.key"

	// --- Timeouts and Retries ---
	DefaultKubeAPIServerReadyTimeout  = 5 * time.Minute
	DefaultKubeletReadyTimeout      = 3 * time.Minute
	DefaultEtcdReadyTimeout         = 5 * time.Minute
	DefaultPodReadyTimeout          = 5 * time.Minute
	DefaultResourceOperationTimeout = 2 * time.Minute
	DefaultTaskRetryAttempts        = 3
	DefaultTaskRetryDelaySeconds    = 10

	// --- File Permissions ---
	DefaultDirPermission        os.FileMode = 0755
	DefaultFilePermission       os.FileMode = 0644
	DefaultKubeconfigPermission os.FileMode = 0600
	DefaultPrivateKeyPermission os.FileMode = 0600

	// --- IP Protocol Types ---
	IPProtocolIPv4      = "IPv4"
	IPProtocolIPv6      = "IPv6"
	IPProtocolDualStack = "DualStack"

	// --- Default Value Placeholders ---
	ValueAuto    = "auto"
	ValueDefault = "default"

	// --- LoadBalancer Types ---
	// Internal LoadBalancer Types
	InternalLBTypeHAProxy = "haproxy"
	InternalLBTypeNginx   = "nginx"
	InternalLBTypeKubeVIP = "kube-vip"

	// External LoadBalancer Types
	ExternalLBTypeKubexmKH   = "kubexm-kh" // KubeXMS managed Keepalived + HAProxy
	ExternalLBTypeKubexmKN   = "kubexm-kn" // KubeXMS managed Keepalived + Nginx
	ExternalLBTypeExternal   = "external"  // User-provided external LB

	// --- Host Defaults ---
	DefaultSSHPort = 22
	DefaultWorkDir = "/tmp/kubexms_work" // Default working directory on remote hosts / or local if applicable
	HostTypeSSH    = "ssh"
	HostTypeLocal  = "local"
	DefaultArch    = "amd64"

	// --- Container Runtimes ---
	RuntimeDocker     = "docker"
	RuntimeContainerd = "containerd"
	// Add other runtime names like RuntimeCRIO = "cri-o" if needed

	// --- Containerd ---
	// ContainerdDefaultConfigFile is defined in paths.go
	ContainerdPluginCRI         = "io.containerd.grpc.v1.cri"

	// --- DNS Defaults ---
	DefaultCoreDNSUpstreamGoogle      = "8.8.8.8"
	DefaultCoreDNSUpstreamCloudflare  = "1.1.1.1"
	DefaultExternalZoneCacheSeconds = 300

	// --- Docker Defaults ---
	DockerLogOptMaxSizeDefault          = "100m"
	DockerLogOptMaxFileDefault          = "3"
	DockerMaxConcurrentDownloadsDefault = 3
	DockerMaxConcurrentUploadsDefault   = 5
	DefaultDockerBridgeName             = "docker0"
	DockerLogDriverJSONFile             = "json-file"
	DockerLogDriverJournald             = "journald"
	DockerLogDriverSyslog               = "syslog"
	DockerLogDriverFluentd              = "fluentd"
	DockerLogDriverNone                 = "none"

	// --- Keepalived Defaults ---
	DefaultKeepalivedVRID            = 51
	DefaultKeepalivedPriorityMaster  = 101
	DefaultKeepalivedPriorityBackup  = 100
	KeepalivedAuthTypePASS           = "PASS" // Already in keepalived_types.go, ensure consistency or remove from one place
	KeepalivedAuthTypeAH             = "AH"   // Already in keepalived_types.go, ensure consistency or remove from one place
	DefaultKeepalivedAuthPass        = "kxm_pass" // Changed to be 8 chars or less
	DefaultKeepalivedPreempt         = true
	DefaultKeepalivedCheckScript     = "/etc/keepalived/check_apiserver.sh" // Example, might need adjustment
	DefaultKeepalivedInterval        = 5    // Health check interval in seconds
	DefaultKeepalivedRise            = 2    // Number of successful checks to transition to MASTER
	DefaultKeepalivedFall            = 2    // Number of failed checks to transition to BACKUP/FAULT
	DefaultKeepalivedAdvertInt       = 1    // VRRP advertisement interval in seconds
	DefaultKeepalivedLVScheduler     = "rr" // Default LVS scheduler (round-robin)

	// --- HAProxy Defaults ---
	DefaultHAProxyMode      = "tcp"
	DefaultHAProxyAlgorithm = "roundrobin"

	// --- NginxLB Defaults ---
	DefaultNginxListenPort   = 6443 // Or a common LB port like 8443
	DefaultNginxConfigFilePath = "/etc/nginx/nginx.conf"
	DefaultNginxMode      = "tcp"
	DefaultNginxAlgorithm = "round_robin" // Nginx uses 'round_robin' not 'roundrobin'

	// --- KubeVIP Defaults ---
	DefaultKubeVIPMode  = "ARP" // or "BGP", ensure case matches KubeVIPModeARP in kubevip_types.go
	DefaultKubeVIPImage = "ghcr.io/kube-vip/kube-vip:v0.8.0" // Example version

	// --- Kubelet Defaults ---
	DefaultKubeletHairpinMode = "promiscuous-bridge"
)
