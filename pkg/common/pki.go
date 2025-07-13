package common

const (
	CACertFileName                     = "ca.crt"
	CAKeyFileName                      = "ca.key"
	APIServerCertFileName              = "apiserver.crt"
	APIServerKeyFileName               = "apiserver.key"
	APIServerKubeletClientCertFileName = "apiserver-kubelet-client.crt"
	APIServerKubeletClientKeyFileName  = "apiserver-kubelet-client.key"
	APIServerEtcdClientCertFileName    = "apiserver-etcd-client.crt"
	APIServerEtcdClientKeyFileName     = "apiserver-etcd-client.key"
	FrontProxyCACertFileName           = "front-proxy-ca.crt"
	FrontProxyCAKeyFileName            = "front-proxy-ca.key"
	FrontProxyClientCertFileName       = "front-proxy-client.crt"
	FrontProxyClientKeyFileName        = "front-proxy-client.key"
	ServiceAccountPublicKeyFileName    = "sa.pub"
	ServiceAccountPrivateKeyFileName   = "sa.key"

	KubeletClientCertFileName   = "kubelet-client.crt"
	KubeletClientKeyFileName    = "kubelet-client.key"
	KubeletServerCertFileName   = "kubelet-server.crt"
	KubeletServerKeyFileName    = "kubelet-server.key"
	KubeProxyClientCertFileName = "kube-proxy.crt"
	KubeProxyClientKeyFileName  = "kube-proxy.key"

	DefaultCertificateKeyAlgorithm   = "RSA"
	DefaultCertificateKeySize        = 2048
	DefaultCertificateValidityDays   = 365
	DefaultCACertificateValidityDays = 3650

	DefaultCertificateKeyUsage    = "Digital Signature, Key Encipherment"
	DefaultCACertificateKeyUsage  = "Digital Signature, Key Encipherment, Certificate Signing"
	DefaultCertificateExtKeyUsage = "Server Authentication, Client Authentication"
)

const (
	DefaultKubexmCertificateOrganization = "kubexm"
	DefaultCertificateCountry            = "CN"
	DefaultCertificateProvince           = "Beijing"
	DefaultCertificateCity               = "Beijing"
	DefaultCertificateOrganizationUnit   = "kubexm-cluster"

	KubernetesAPICertCNPattern = "kube-apiserver"
	KubernetesAPICertCN        = "kubernetes"
	EtcdServerCertCNPattern    = "etcd-server"
	EtcdPeerCertCNPattern      = "etcd-peer"
	EtcdClientCertCNPattern    = "etcd-client"
	KubeletServerCertCNPattern = "kubelet-server"
	KubeletClientCertCNPattern = "kubelet-client"

	DefaultKubernetesServiceName = "kubernetes"
	DefaultKubernetesServiceFQDN = "kubernetes.default.svc.cluster.local"
	DefaultAPIServerServiceName  = "kube-apiserver"
	DefaultEtcdServiceName       = "etcd"
	DefaultEtcdServiceFQDN       = "etcd.kube-system.svc.cluster.local"
)

const (
	DefaultCertExpirationWarningDays  = 30
	DefaultCertExpirationCriticalDays = 7

	DefaultCertRenewalThresholdDays = 30
	DefaultCertRenewalCheckInterval = 24

	DefaultCertBackupRetentionDays = 90
	DefaultCertBackupEnabled       = true

	TenYears     = 10 * 365
	HundredYears = 100 * 365
)

const (
	DefaultCertificateFilePermission = 0644
	DefaultPrivateKeyFilePermission  = 0600
	DefaultCertificateDirPermission  = 0755
	DefaultPrivateKeyDirPermission   = 0700

	DefaultCertificateFileOwner = "root"
	DefaultCertificateFileGroup = "root"
	DefaultPrivateKeyFileOwner  = "root"
	DefaultPrivateKeyFileGroup  = "root"

	DefaultEtcdCertFilePermission = 0644
	DefaultEtcdKeyFilePermission  = 0600
	DefaultEtcdCertDirPermission  = 0755
	DefaultEtcdKeyDirPermission   = 0700
)
