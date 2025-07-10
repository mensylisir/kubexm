package common

import (
	"os"
	"time"
)

// This file contains a minimal set of constants to allow pkg/config and pkg/util/validation to compile.
// Other constants that were previously here and potentially causing redeclaration errors due to
// other .go files in the pkg/common package (which I cannot see or delete) are temporarily removed.
// They will be restored and de-duplicated in a dedicated step for pkg/common refactoring.

const (
	ClusterTypeKubeXM  = "kubexm"
	ClusterTypeKubeadm = "kubeadm"
)

const (
	DefaultSSHPort = 22
)

const DefaultArch = "amd64"
const HostTypeSSH = "ssh"
const DefaultConnectionTimeout = 30 * time.Second
const HostTypeLocal = "local"

var SupportedArches = []string{"amd64", "arm64"}
var ValidTaintEffects = []string{"NoSchedule", "PreferNoSchedule", "NoExecute"}

const (
	ExternalLBTypeKubexmKH = "kubexm-kh"
	ExternalLBTypeKubexmKN = "kubexm-kn"
)

// Regex constants are moved directly into pkg/util/validation/validation.go for now.
// const ValidChartVersionRegexString = ` + "`^v?([0-9]+)(\\.[0-9]+){0,2}$`" + `
const DomainValidationRegexString = `^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`

// Minimal set of other constants that might be directly or indirectly needed by validation logic
// or default setting logic in cluster_types.go.
const (
	DefaultKubernetesAPIServerPort = 6443
	DefaultK8sVersion              = "v1.28.2" // Example, as used in some SetDefaults
	DefaultImageRegistry           = "registry.k8s.io"
	PauseImageName                 = "pause"
	DefaultEtcdVersion          = "3.5.10-0"
	DefaultCoreDNSVersion       = "v1.10.1"
	DefaultContainerdVersion    = "1.7.11"
	DefaultKeepalivedAuthPass   = "kxm_pass"
	DefaultHAProxyFrontendPort  = 6443
	DefaultHAProxyMode          = "tcp"
	DefaultHAProxyAlgorithm     = "roundrobin"
	DefaultKubeletHairpinMode   = "promiscuous-bridge"
	DefaultLocalRegistryDataDir = "/var/lib/registry"
	DefaultRemoteWorkDir        = "/tmp/kubexms_work"
	DefaultWorkDirName          = ".kubexm"
	// DefaultTmpDirName is the default name for a temporary directory under the system's temp path.
	DefaultTmpDirName           = ".kubexm"
	ContainerdSocketPath        = "unix:///run/containerd/containerd.sock"
	DockerSocketPath            = "unix:///var/run/docker.sock"
	CriDockerdSocketPath        = "/var/run/cri-dockerd.sock"
	KernelModuleBrNetfilter     = "br_netfilter"
	KernelModuleIpvs            = "ip_vs"
	DefaultMinCPUCores          = 2
	DefaultMinMemoryMB          = uint64(2048)
	CgroupDriverSystemd         = "systemd"
	CgroupDriverCgroupfs        = "cgroupfs"
	KubeProxyModeIPTables       = "iptables"
	KubeProxyModeIPVS           = "ipvs"
	CNICalico                   = "calico"
	CNIFlannel                  = "flannel"
	CNICilium                   = "cilium"
	CNIMultus                   = "multus"
	CNIKubeOvn                  = "kube-ovn"
	CNIHybridnet                = "hybridnet"
	InternalLBTypeHAProxy       = "haproxy"
	InternalLBTypeNginx         = "nginx"
	InternalLBTypeKubeVIP       = "kube-vip"
	ExternalLBTypeExternal      = "external"
	RuntimeDocker               = "docker"
	RuntimeContainerd           = "containerd"
	RuntimeCRIO                 = "cri-o"
	RuntimeIsula                = "isula"

	// Special Role Names
	AllHostsRole    = "all"          // Represents all hosts in the inventory for targeting operations
	ControlNodeRole = "control-node" // Represents the machine where kubexm CLI is running

	// Docker specific defaults
	DockerLogOptMaxSizeDefault          = "100m"
	DockerLogOptMaxFileDefault          = "5"
	DockerMaxConcurrentDownloadsDefault = 3
	DockerMaxConcurrentUploadsDefault   = 5
	DefaultDockerBridgeName             = "docker0"
	DockerLogDriverJSONFile             = "json-file"
	DockerLogDriverJournald             = "journald"
	DockerLogDriverSyslog               = "syslog"
	DockerLogDriverFluentd              = "fluentd"
	DockerLogDriverNone                 = "none"

	// Keepalived specific defaults
	KeepalivedAuthTypePASS           = "PASS"
	KeepalivedAuthTypeAH             = "AH"
	DefaultKeepalivedPreempt         = true
	DefaultKeepalivedCheckScript     = "/etc/keepalived/check_apiserver.sh" // Example path
	DefaultKeepalivedInterval        = 5
	DefaultKeepalivedRise            = 2
	DefaultKeepalivedFall            = 2
	DefaultKeepalivedAdvertInt       = 1
	DefaultKeepalivedLVScheduler     = "rr"

	// NginxLB specific defaults
	DefaultNginxListenPort       = 6443
	DefaultNginxMode             = "stream" // for TCP load balancing
	DefaultNginxAlgorithm        = "round_robin"
	DefaultNginxConfigFilePath   = "/etc/nginx/nginx.conf"

	// KubeVIP specific defaults
	DefaultKubeVIPImage          = "ghcr.io/kube-vip/kube-vip:v0.7.0" // Example version

	// DNS specific defaults
	DefaultCoreDNSUpstreamGoogle     = "8.8.8.8"
	DefaultCoreDNSUpstreamCloudflare = "1.1.1.1"
	DefaultExternalZoneCacheSeconds  = 300

	// Etcd specific paths and names
	EtcdDefaultPKIDir          = "/etc/kubernetes/pki/etcd" // Standard path for etcd PKI managed by kubeadm, adapt if binary install uses different
	EtcdDefaultDataDir         = "/var/lib/etcd"
	CACertFileName             = "ca.pem" // More common than .crt for PEM
	EtcdServerCertFileName     = "server.pem"
	EtcdServerKeyFileName      = "server-key.pem"
	EtcdPeerCertFileName       = "peer.pem"
	EtcdPeerKeyFileName        = "peer-key.pem"
	EtcdClientCertFileName     = "client.pem" // Generic client, apiserver might use apiserver-etcd-client.pem
	EtcdClientKeyFileName      = "client-key.pem"
	EtcdDefaultConfFile        = "/etc/etcd/etcd.conf.yaml"
	EtcdDefaultSystemdFile     = "/etc/systemd/system/etcd.service"
	EtcdDefaultBinDir          = "/usr/local/bin"
	DefaultEtcdVersionForBinInstall = "v3.5.13" // As used in the task
)

// File Permissions
const (
	DefaultDirPermission        = os.FileMode(0755)
	DefaultFilePermission       = os.FileMode(0644)
	DefaultKubeconfigPermission = os.FileMode(0600)
	DefaultPrivateKeyPermission = os.FileMode(0600)
)
