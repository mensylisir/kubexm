package common

const (
	DefaultK8sVersion       = "v1.28.2"
	DefaultPauseImageName   = "pause"
	KubernetesDefaultDomain = "local.kubexm.io"
)

type KubernetesDeploymentType string

const (
	KubernetesDeploymentTypeKubeadm KubernetesDeploymentType = "kubeadm"
	KubernetesDeploymentTypeKubexm  KubernetesDeploymentType = "kubexm"
)

const (
	ClusterTypeKubeXM  = string(KubernetesDeploymentTypeKubexm)
	ClusterTypeKubeadm = string(KubernetesDeploymentTypeKubeadm)
)

const (
	KubeSystemNamespace   = "kube-system"
	KubePublicNamespace   = "kube-public"
	DefaultAddonNamespace = KubeSystemNamespace
)

const (
	ClusterInfoConfigMapName   = "cluster-info"
	KubeadmConfigConfigMapName = "kubeadm-config"
	KubeletConfigConfigMapName = "kubelet-config"
	KubeProxyConfigMapName     = "kube-proxy"
	KubeProxyDaemonSetName     = "kube-proxy"
	BootstrapTokenSecretPrefix = "bootstrap-token-"
	KubeadmCertsSecretName     = "kubeadm-certs"
)

const (
	NodeBootstrapperClusterRoleName        = "system:node-bootstrapper"
	KubeadmNodeAdminClusterRoleBindingName = "kubeadm:node-admins"
	SystemNodeClusterRoleName              = "system:node"
	SystemKubeProxyClusterRoleBindingName  = "system:kube-proxy"
	DefaultCertificateOrganization         = "system:masters"
	KubeletCertificateOrganization         = "system:nodes"
	KubeletCertificateCNPrefix             = "system:node:" // Followed by the node name.
	KubeAPIServerCN                        = "kube-apiserver"
	KubeControllerManagerUser              = "system:kube-controller-manager"
	KubeSchedulerUser                      = "system:kube-scheduler"
	KubeProxyUser                          = "system:kube-proxy"
	EtcdAdminUser                          = "root"
)

const (
	AnnotationNodeKubeadmAlphaExcludeFromExternalLB = "node.kubernetes.io/exclude-from-external-load-balancers"
	LabelNodeRoleExcludeBalancer                    = "node-role.kubernetes.io/exclude-balancer"
	LabelKubeAPIServerHA                            = "kubexm.io/ha-apiserver"
)

const (
	DefaultKubeConfigPath                  = "/etc/kubernetes"
	DefaultKubeletPath                     = "/var/lib/kubelet"
	KubeletHomeDir                         = "/var/lib/kubelet"
	KubernetesConfigDir                    = "/etc/kubernetes"
	KubernetesManifestsDir                 = "/etc/kubernetes/manifests"
	DefaultManifestPath                    = "/etc/kubernetes/manifests"
	KubernetesPKIDir                       = "/etc/kubernetes/pki"
	DefaultPKIPath                         = "/etc/kubernetes/pki"
	KubernetesPKISSLDir                    = "/etc/kubernetes/ssl"
	DefaultKubeconfigPath                  = "/root/.kube/config"
	KubeletSystemdDropinDirTarget          = "/etc/systemd/system/kubelet.service.d"
	KubeletCSICertsVolumeName              = "kubelet-csi-certs"
	KubeletCSICertsMountPath               = "/var/lib/kubelet/plugins_registry"
	KubeletKubeconfigPathTarget            = "/etc/kubernetes/kubelet.conf"
	KubeletBootstrapKubeconfigPathTarget   = "/etc/kubernetes/bootstrap-kubelet.conf"
	KubeletConfigYAMLPathTarget            = "/var/lib/kubelet/config.yaml"
	KubeletFlagsEnvPathTarget              = "/var/lib/kubelet/kubeadm-flags.env"
	KubeletPKIDirTarget                    = "/var/lib/kubelet/pki"
	DefaultKubernetesPKIDir                = "/etc/kubernetes/pki"
	DefaultKubernetesPKIFrontProxyDir      = "/etc/kubernetes/pki/front-proxy"
	DefaultKubeletPKIDir                   = "/var/lib/kubelet/pki"
	DefaultKubeletCertsDir                 = "/etc/kubernetes/kubelet"
	DefaultLocalPKIWorkDir                 = "pki"
	DefaultLocalEtcdPKIWorkDir             = "pki/etcd"
	DefaultLocalKubernetesPKIWorkDir       = "pki/kubernetes"
	KubeletKubeconfigFileName              = "kubelet.conf"
	KubeletSystemdEnvFileName              = "10-kubeadm.conf"
	ControllerManagerKubeconfigFileName    = "controller-manager.conf"
	SchedulerKubeconfigFileName            = "scheduler.conf"
	AdminKubeconfigFileName                = "admin.conf"
	KubeProxyKubeconfigFileName            = "kube-proxy.conf"
	KubeAPIServerStaticPodFileName         = "kube-apiserver.yaml"
	KubeControllerManagerStaticPodFileName = "kube-controller-manager.yaml"
	KubeSchedulerStaticPodFileName         = "kube-scheduler.yaml"
	EtcdStaticPodFileName                  = "etcd.yaml"
	DefaultKubeConfigFile                  = "admin.conf"
	DefaultKubeletConfig                   = "kubelet.conf"
)

const (
	KubeadmConfigFileName           = "kubeadm-config.yaml"
	KubeadmInitConfigFileName       = "kubeadm-init-config.yaml"
	KubeadmJoinMasterConfigFileName = "kubeadm-join-master-config.yaml"
	KubeadmJoinWorkerConfigFileName = "kubeadm-join-worker-config.yaml"

	KubeadmTokenDefaultTTL                = "24h0m0s"
	KubeadmDiscoveryTokenCACertHashPrefix = "sha256:"

	KubeadmUploadCertsPhase = "upload-certs"
	KubeadmTokenCommand     = "token"
	KubeadmInitCommand      = "init"
	KubeadmResetCommand     = "reset"
	KubeadmJoinCommand      = "join"
	KubeadmUpgradeCommand   = "upgrade"
)

const (
	DefaultKubeletHairpinMode = "promiscuous-bridge"
	KubeProxyModeIPTables     = "iptables"
	KubeProxyModeIPVS         = "ipvs"
	CgroupDriverSystemd       = "systemd"
	CgroupDriverCgroupfs      = "cgroupfs"
)
