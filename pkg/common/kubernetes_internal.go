package common

// --- Kubernetes Internal Resource Names ---

// --- CoreDNS ---
const (
	CoreDNSConfigMapName  = "coredns"
	CoreDNSDeploymentName = "coredns"
	CoreDNSServiceName    = "kube-dns" // The service name for CoreDNS is often kube-dns for historical reasons
)

// --- Kube-proxy ---
const (
	KubeProxyConfigMapName = "kube-proxy"
	KubeProxyDaemonSetName = "kube-proxy"
)

// --- Cluster Info ---
const (
	ClusterInfoConfigMapName   = "cluster-info" // Typically in kube-public namespace
	KubeadmConfigConfigMapName = "kubeadm-config" // In kube-system namespace
)

// --- Secrets ---
const (
	BootstrapTokenSecretPrefix = "bootstrap-token-"
)

// --- RBAC ---
const (
	NodeBootstrapperClusterRoleName        = "system:node-bootstrapper"
	KubeadmNodeAdminClusterRoleBindingName = "kubeadm:node-admins" // RoleBinding for admin access via kubelet certs
	// More RBAC constants can be added as needed, e.g., for specific CNI permissions if managed by kubexm.
)

// --- Kubelet settings ---
const (
	KubeletCSICertsVolumeName = "kubelet-csi-certs"
	KubeletCSICertsMountPath  = "/var/lib/kubelet/plugins_registry"
)
