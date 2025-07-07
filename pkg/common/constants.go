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
)
