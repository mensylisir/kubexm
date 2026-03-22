package common

const (
	KubeAPIServer         = "kube-apiserver"
	KubeControllerManager = "kube-controller-manager"
	KubeScheduler         = "kube-scheduler"
	Kubelet               = "kubelet"
	KubeProxy             = "kube-proxy"
	Etcd                  = "etcd"
	Etcdctl               = "etcdctl"
	Etcdutl               = "etcdutl"
	Containerd            = "containerd"
	Docker                = "docker"
	Runc                  = "runc"
	CniDockerd            = "cri-dockerd"
	Kubeadm               = "kubeadm"
	Kubectl               = "kubectl"
	Keepalived            = "keepalived"
	HAProxy               = "haproxy"
	Nginx                 = "nginx"
	KubeVIP               = "kube-vip"
	Calicoctl             = "calicoctl"
	Helm                  = "helm"
	Crictl                = "crictl"
	NodeLocalDNS          = "node-local-dns"
)

const (
	KubeletServiceName           = "kubelet.service"
	ContainerdServiceName        = "containerd.service"
	DockerServiceName            = "docker.service"
	EtcdServiceName              = "etcd.service"
	CniDockerdServiceName        = "cri-dockerd.service"
	KeepalivedServiceName        = "keepalived.service"
	HAProxyServiceName           = "haproxy.service"
	NginxServiceName             = "nginx.service"
	CrioServiceName              = "crio.service"
	IsuladServiceName            = "isulad.service"
	EtcdDefragTimerServiceName   = "etcd-defrag.timer"
	EtcdDefragSystemdServiceName = "etcd-defrag.service"
	CertCheckServiceName         = "cert-check.service"
	CertCheckTimerServiceName    = "cert-check.timer"
	CertUpdateServiceName        = "cert-update.service"
	CertUpdateTimerServiceName   = "cert-update.timer"
)

// --- Default Network Ports ---
// Default ports used by various components.
const (
	KubeAPIServerDefaultPort         = 6443
	KubeSchedulerDefaultPort         = 10259
	KubeControllerManagerDefaultPort = 10257
	KubeletDefaultPort               = 10250
	EtcdDefaultClientPort            = 2379
	EtcdDefaultPeerPort              = 2380
	HAProxyDefaultFrontendPort       = 6443
	CoreDNSMetricsPort               = 9153
	NodeLocalDNSMetricsPort          = 9253
	KubeProxyMetricsPort             = 10249
	KubeProxyHealthzPort             = 10256
	DefaultAPIServerPort             = 6443
	DefaultEtcdClientPort            = 2379
	DefaultEtcdPeerPort              = 2380
	EtcdctlDefaultEndpoint           = "127.0.0.1:2379"
)

const (
	Socat               = "socat"
	ConntrackTools      = "conntrack-tools"
	Conntrack           = "conntrack"
	IPSet               = "ipset"
	Ipvsadm             = "ipvsadm"
	NfsUtils            = "nfs-utils"
	NfsCommon           = "nfs-common"
	CephCommon          = "ceph-common"
	Curl                = "curl"
	OpenIscsi           = "open-iscsi"
	IscsiInitiatorUtils = "iscsi-initiator-utils"
	Pgrep               = "pgrep"
	Killall             = "killall"
	KeepalivedPackage   = "keepalived"
	HaproxyPackage      = "haproxy"
	NginxPackage        = "nginx"
)
