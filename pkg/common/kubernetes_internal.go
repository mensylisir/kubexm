package common

// This file defines constants related to internal Kubernetes resource names,
// naming conventions, and other specific details of the Kubernetes ecosystem
// that Kubexm might need to interact with or configure.

// --- Kubernetes Internal Resource Names ---

// --- CoreDNS Related Names ---
const (
	// CoreDNSConfigMapName is the default name for the CoreDNS ConfigMap in kube-system.
	CoreDNSConfigMapName = "coredns"
	// CoreDNSDeploymentName is the default name for the CoreDNS Deployment in kube-system.
	CoreDNSDeploymentName = "coredns"
	// CoreDNSServiceName is the default service name for CoreDNS (often "kube-dns" for legacy compatibility).
	CoreDNSServiceName = "kube-dns"
	// CoreDNSAutoscalerConfigMapName is the name of the ConfigMap for coredns-autoscaler (if used).
	CoreDNSAutoscalerConfigMapName = "coredns-autoscaler"
)

// --- Kube-proxy Related Names ---
const (
	// KubeProxyConfigMapName is the default name for the Kube-proxy ConfigMap in kube-system.
	KubeProxyConfigMapName = "kube-proxy"
	// KubeProxyDaemonSetName is the default name for the Kube-proxy DaemonSet in kube-system.
	KubeProxyDaemonSetName = "kube-proxy"
)

// --- Cluster Information ConfigMap Names ---
const (
	// ClusterInfoConfigMapName is the name of the ConfigMap in kube-public holding some cluster info.
	ClusterInfoConfigMapName = "cluster-info"
	// KubeadmConfigConfigMapName is the name of the ConfigMap in kube-system storing kubeadm's ClusterConfiguration.
	KubeadmConfigConfigMapName = "kubeadm-config"
	// KubeletConfigConfigMapName is the name of the ConfigMap in kube-system storing the cluster-wide KubeletConfiguration.
	KubeletConfigConfigMapName = "kubelet-config" // Kubeadm stores KubeletConfiguration here.
)

// --- Secrets Related Names ---
const (
	// BootstrapTokenSecretPrefix is the prefix for bootstrap token secrets in kube-system.
	BootstrapTokenSecretPrefix = "bootstrap-token-"
	// KubeadmCertsSecretName is the name of the Secret in kube-system storing shared certificates for HA.
	KubeadmCertsSecretName = "kubeadm-certs"
)

// --- RBAC (Role-Based Access Control) Related Names ---
const (
	// NodeBootstrapperClusterRoleName is the ClusterRole for node bootstrapping.
	NodeBootstrapperClusterRoleName = "system:node-bootstrapper"
	// KubeadmNodeAdminClusterRoleBindingName is a ClusterRoleBinding granting admin access via Kubelet certs in some setups.
	KubeadmNodeAdminClusterRoleBindingName = "kubeadm:node-admins"
	// SystemNodeClusterRoleName is the ClusterRole for system:node identities.
	SystemNodeClusterRoleName = "system:node"
	// SystemKubeProxyClusterRoleBindingName is the ClusterRoleBinding for kube-proxy.
	SystemKubeProxyClusterRoleBindingName = "system:kube-proxy"
)

// --- Kubelet settings ---
const (
	// KubeletCSICertsVolumeName is a common volume name for CSI certs in Kubelet.
	KubeletCSICertsVolumeName = "kubelet-csi-certs"
	// KubeletCSICertsMountPath is a common mount path for CSI certs in Kubelet.
	KubeletCSICertsMountPath = "/var/lib/kubelet/plugins_registry"
)

// --- Kubeadm Specific Constants ---
const (
	// KubeadmInitConfigFileName is a common local name for the kubeadm init configuration file.
	KubeadmInitConfigFileName = "kubeadm-init-config.yaml"
	// KubeadmJoinMasterConfigFileName is a common local name for joining control-plane nodes.
	KubeadmJoinMasterConfigFileName = "kubeadm-join-master-config.yaml"
	// KubeadmJoinWorkerConfigFileName is a common local name for joining worker nodes.
	KubeadmJoinWorkerConfigFileName = "kubeadm-join-worker-config.yaml"
	// KubeadmResetCommand is the command used to reset a kubeadm node.
	KubeadmResetCommand = "reset"
	// KubeadmTokenDefaultTTL is the default time-to-live for a bootstrap token.
	KubeadmTokenDefaultTTL = "24h0m0s"
	// KubeadmDiscoveryTokenCACertHashPrefix is the prefix for CA cert hashes in discovery tokens.
	KubeadmDiscoveryTokenCACertHashPrefix = "sha256:"
	// KubeadmUploadCertsPhase is the kubeadm phase for uploading control-plane certificates.
	KubeadmUploadCertsPhase = "upload-certs"
	// KubeadmTokenCommand is the base command for token management.
	KubeadmTokenCommand = "token"
	// KubeadmInitCommand is the base command for cluster initialization.
	KubeadmInitCommand = "init"
	// KubeadmJoinCommand is the base command for joining nodes.
	KubeadmJoinCommand = "join"
	// KubeadmUpgradeCommand is the base command for cluster upgrades.
	KubeadmUpgradeCommand = "upgrade"
)

// --- Certificate Common Names (CN) and Organizations (O) ---
// Standard identities used in Kubernetes PKI.
const (
	// DefaultCertificateOrganization is often used for admin-level certificates.
	DefaultCertificateOrganization = "system:masters"
	// KubeletCertificateOrganization is the organization for Kubelet client certificates.
	KubeletCertificateOrganization = "system:nodes"
	// KubeletCertificateCNPrefix is the prefix for Kubelet client certificate Common Names.
	KubeletCertificateCNPrefix = "system:node:" // Followed by the node name.
	// KubeAPIServerCN is the Common Name for the Kube API Server certificate.
	KubeAPIServerCN = "kube-apiserver"
	// KubeControllerManagerUser is the user/CN for Kube Controller Manager.
	KubeControllerManagerUser = "system:kube-controller-manager"
	// KubeSchedulerUser is the user/CN for Kube Scheduler.
	KubeSchedulerUser = "system:kube-scheduler"
	// KubeProxyUser is the user/CN for Kube Proxy.
	KubeProxyUser = "system:kube-proxy"
	// EtcdAdminUser is a common CN for an etcd admin client certificate.
	EtcdAdminUser = "root" // Often used with O="system:masters" for etcdctl admin access.
)

// --- Common Annotation and Label Keys ---
// Standard and commonly used annotation/label keys in Kubernetes.
const (
	// AnnotationNodeKubeadmAlphaExcludeFromExternalLB is used by some external load balancers to exclude nodes.
	AnnotationNodeKubeadmAlphaExcludeFromExternalLB = "node.kubernetes.io/exclude-from-external-load-balancers"
	// LabelNodeRoleExcludeBalancer is a common label to exclude nodes from LB backends (e.g. for MetalLB).
	LabelNodeRoleExcludeBalancer = "node-role.kubernetes.io/exclude-balancer"
	// LabelKubeAPIServerHA is a label that could be used to identify HA API server pods.
	LabelKubeAPIServerHA = "kubexm.io/ha-apiserver"
)

// --- Namespaces ---
const (
	KubeSystemNamespace = "kube-system"
	KubePublicNamespace = "kube-public"
	// DefaultAddonNamespace is the default namespace for installing addons if not specified.
	DefaultAddonNamespace = KubeSystemNamespace
)
