package common

// --- Component Names ---
const (
	KubeAPIServer        = "kube-apiserver"
	KubeControllerManager = "kube-controller-manager"
	KubeScheduler        = "kube-scheduler"
	Kubelet              = "kubelet"
	KubeProxy            = "kube-proxy"
	Etcd                 = "etcd"
	Etcdctl              = "etcdctl" // Added from etcd_types.go for etcdctl binary
	Containerd           = "containerd"
	Docker               = "docker"
	Runc                 = "runc"        // Added from containerd_types.go
	CniDockerd           = "cri-dockerd" // Added from docker_types.go
	Kubeadm              = "kubeadm"
	Kubectl              = "kubectl"
	Keepalived           = "keepalived"  // Added for HA
	HAProxy              = "haproxy"     // Added for HA
	KubeVIP              = "kube-vip"    // Added for HA
)

// --- Service Names (systemd) ---
const (
	KubeletServiceName      = "kubelet.service"
	ContainerdServiceName   = "containerd.service"
	DockerServiceName       = "docker.service"
	EtcdServiceName         = "etcd.service"         // Added
	CniDockerdServiceName   = "cri-dockerd.service"  // Added
	KeepalivedServiceName   = "keepalived.service" // Added
	HAProxyServiceName      = "haproxy.service"    // Added
)

// --- Default Ports ---
const (
	KubeAPIServerDefaultPort         = 6443
	KubeSchedulerDefaultPort         = 10259 // Secure port for scheduler
	KubeControllerManagerDefaultPort = 10257 // Secure port for controller-manager
	// KubeSchedulerDefaultInsecurePort = 10251 (Older insecure, now typically secure or via kubeconfig)
	// KubeControllerManagerDefaultInsecurePort = 10252 (Older insecure)
	KubeletDefaultPort               = 10250
	EtcdDefaultClientPort            = 2379
	EtcdDefaultPeerPort              = 2380
	HAProxyDefaultFrontendPort       = 6443 // Often same as APIServer for external LB
)

// --- Kernel Modules ---
const (
	KernelModuleBrNetfilter = "br_netfilter"
	KernelModuleIpvs        = "ip_vs"
)

// --- Preflight Check related defaults ---
const (
	DefaultMinCPUCores   = 2
	DefaultMinMemoryMB   = 2048 // 2GB
)
