package common

// This file defines constants related to various software components,
// their service names, default ports, and related tooling.

// --- Well-Known Component Names ---
// These constants represent the canonical names for various components managed by Kubexm.
const (
	KubeAPIServer         = "kube-apiserver"
	KubeControllerManager = "kube-controller-manager"
	KubeScheduler         = "kube-scheduler"
	Kubelet               = "kubelet"
	KubeProxy             = "kube-proxy"
	Etcd                  = "etcd"
	Etcdctl               = "etcdctl"      // Etcd command-line client.
	Containerd            = "containerd"
	Docker                = "docker"
	Runc                  = "runc"         // Default OCI runtime for Containerd and Docker.
	CniDockerd            = "cri-dockerd"  // CRI shim for Docker.
	Kubeadm               = "kubeadm"      // Kubernetes cluster bootstrap tool.
	Kubectl               = "kubectl"      // Kubernetes command-line client.
	Keepalived            = "keepalived"   // For IPVS-based HA solutions.
	HAProxy               = "haproxy"      // For TCP/HTTP load balancing.
	Nginx                 = "nginx"        // Often used as a load balancer or reverse proxy.
	KubeVIP               = "kube-vip"     // For VIP management in Kubernetes clusters.
	Calicoctl             = "calicoctl"    // Calico command-line client.
	Helm                  = "helm"         // Kubernetes package manager.
	Crictl                = "crictl"       // CRI-compatible command-line client for runtimes.
	NodeLocalDNS          = "node-local-dns" // NodeLocal DNSCache component name.
)

// --- Systemd Service Names ---
// These constants define the standard systemd service names for various components.
const (
	KubeletServiceName           = "kubelet.service"
	ContainerdServiceName        = "containerd.service"
	DockerServiceName            = "docker.service"
	EtcdServiceName              = "etcd.service"
	CniDockerdServiceName        = "cri-dockerd.service"
	KeepalivedServiceName        = "keepalived.service"
	HAProxyServiceName           = "haproxy.service"
	NginxServiceName             = "nginx.service" // Added for Nginx as a system service
	CrioServiceName              = "crio.service"  // Added for CRI-O
	IsuladServiceName            = "isulad.service"// Added for iSulad
	EtcdDefragTimerServiceName   = "etcd-defrag.timer" // For scheduled defrag
	EtcdDefragSystemdServiceName = "etcd-defrag.service" // For on-demand defrag
)

// --- Default Network Ports ---
// Default ports used by various components.
const (
	KubeAPIServerDefaultPort         = 6443  // Default secure port for Kubernetes API server.
	KubeSchedulerDefaultPort         = 10259 // Default secure port for Kubernetes Scheduler.
	KubeControllerManagerDefaultPort = 10257 // Default secure port for Kubernetes Controller Manager.
	KubeletDefaultPort               = 10250 // Default Kubelet API port.
	EtcdDefaultClientPort            = 2379  // Default client port for Etcd.
	EtcdDefaultPeerPort              = 2380  // Default peer port for Etcd.
	HAProxyDefaultFrontendPort       = 6443  // Default frontend port for HAProxy (often for K8s API).
	CoreDNSMetricsPort               = 9153  // Default metrics port for CoreDNS.
	NodeLocalDNSMetricsPort          = 9253  // Default metrics port for NodeLocal DNSCache.
	KubeProxyMetricsPort             = 10249 // Default metrics port for KubeProxy.
	KubeProxyHealthzPort             = 10256 // Default health check port for KubeProxy.
)

// --- Common Tools and Utility Packages ---
// Names of common system utilities or packages that might be dependencies.
const (
	Socat      = "socat"
	Conntrack  = "conntrack-tools" // Package name can vary, e.g. conntrack on some
	IPSet      = "ipset"
	Ipvsadm    = "ipvsadm"
	NfsUtils   = "nfs-utils"   // Or nfs-common on Debian/Ubuntu.
	CephCommon = "ceph-common" // For CephFS and RBD storage.
	Curl       = "curl"
	Pgrep      = "pgrep" // From procps or similar package
	Killall    = "killall" // From psmisc or similar package
)

// --- Default Image Repositories and Versions ---
// Centralized defaults for common container images. Specific versions might be overridden by user config.
const (
	DefaultK8sImageRegistry         = "registry.k8s.io"                       // Default registry for official Kubernetes images.
	DefaultCoreDNSImageRepository   = DefaultK8sImageRegistry + "/coredns"    // Default image repository for CoreDNS.
	DefaultPauseImageRepository     = DefaultK8sImageRegistry                 // Pause image is also in registry.k8s.io.
	DefaultKubeVIPImageRepository   = "ghcr.io/kube-vip"                      // Default image repository for Kube-VIP.
	DefaultHAProxyImageRepository = "docker.io/library/haproxy"             // Default image repository for HAProxy.
	DefaultNginxImageRepository   = "docker.io/library/nginx"               // Default image repository for Nginx.
	// DefaultPauseImageVersion is defined in constants.go.
	// DefaultCoreDNSVersion is defined in constants.go.
	// DefaultKubeVIPImage is defined in images.go (contains version).
)

// --- Default Socket Paths ---
// Moved from original constants.go
const (
	ContainerdSocketPath = "unix:///run/containerd/containerd.sock" // Default socket path for Containerd.
	DockerSocketPath     = "unix:///var/run/docker.sock"             // Default socket path for Docker.
	CriDockerdSocketPath = "/var/run/cri-dockerd.sock"               // Default socket path for cri-dockerd.
	// Consider adding other runtime sockets if they become relevant e.g. CRIO
)

// --- Containerd Specific ---
// Moved from original constants.go
const (
	ContainerdPluginCRI = "cri" // Name of the CRI plugin for Containerd.
)
