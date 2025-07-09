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
// const DomainValidationRegexString = ` + "`^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`" + `

// Minimal set of other constants that might be directly or indirectly needed by validation logic
// or default setting logic in cluster_types.go.
const (
	DefaultK8sVersion           = "v1.28.2" // Example, as used in some SetDefaults
	DefaultImageRegistry        = "registry.k8s.io"
	PauseImageName              = "pause"
	DefaultEtcdVersion          = "3.5.10-0"
	DefaultCoreDNSVersion       = "v1.10.1"
	DefaultContainerdVersion    = "1.7.11"
	DefaultKeepalivedAuthPass   = "kxm_pass"
	DefaultHAProxyMode          = "tcp"
	DefaultHAProxyAlgorithm     = "roundrobin"
	DefaultKubeletHairpinMode   = "promiscuous-bridge"
	DefaultLocalRegistryDataDir = "/var/lib/registry"
	DefaultRemoteWorkDir        = "/tmp/kubexms_work"
	DefaultWorkDirName          = ".kubexm"
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
)

// File Permissions
const (
	DefaultDirPermission        = os.FileMode(0755)
	DefaultFilePermission       = os.FileMode(0644)
	DefaultKubeconfigPermission = os.FileMode(0600)
	DefaultPrivateKeyPermission = os.FileMode(0600)
)
