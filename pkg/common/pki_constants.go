package common

// PKI and Certificate constants following Kubernetes and etcd best practices
const (
	// Kubernetes PKI certificate file names
	CACertFileName                     = "ca.crt"                       // Cluster CA certificate file
	CAKeyFileName                      = "ca.key"                       // Cluster CA private key file
	APIServerCertFileName              = "apiserver.crt"                // API server certificate file
	APIServerKeyFileName               = "apiserver.key"                // API server private key file
	APIServerKubeletClientCertFileName = "apiserver-kubelet-client.crt" // API server to kubelet client certificate
	APIServerKubeletClientKeyFileName  = "apiserver-kubelet-client.key" // API server to kubelet client private key
	APIServerEtcdClientCertFileName    = "apiserver-etcd-client.crt"    // API server to etcd client certificate
	APIServerEtcdClientKeyFileName     = "apiserver-etcd-client.key"    // API server to etcd client private key
	FrontProxyCACertFileName           = "front-proxy-ca.crt"           // Front proxy CA certificate
	FrontProxyCAKeyFileName            = "front-proxy-ca.key"           // Front proxy CA private key
	FrontProxyClientCertFileName       = "front-proxy-client.crt"       // Front proxy client certificate
	FrontProxyClientKeyFileName        = "front-proxy-client.key"       // Front proxy client private key
	ServiceAccountPublicKeyFileName    = "sa.pub"                       // Service account public key
	ServiceAccountPrivateKeyFileName   = "sa.key"                       // Service account private key
	
	// etcd PKI certificate file names
	EtcdCaCertFileName              = "ca.pem"              // etcd CA certificate file
	EtcdCaKeyFileName               = "ca-key.pem"          // etcd CA private key file
	EtcdServerCertFileName          = "server.crt"          // etcd server certificate file
	EtcdServerKeyFileName           = "server.key"          // etcd server private key file
	EtcdPeerCertFileName            = "peer.crt"            // etcd peer certificate file
	EtcdPeerKeyFileName             = "peer.key"            // etcd peer private key file
	EtcdAdminClientCertFileName     = "admin.crt"           // etcd admin client certificate file
	EtcdAdminClientKeyFileName      = "admin.key"           // etcd admin client private key file
	EtcdHealthClientCertFileName    = "healthcheck.crt"     // etcd health check client certificate file
	EtcdHealthClientKeyFileName     = "healthcheck.key"     // etcd health check client private key file
	
	// etcd certificate file names with node-specific naming pattern
	EtcdNodeCertFileNamePattern     = "node-%s.pem"        // etcd node certificate pattern (node-aa1.pem)
	EtcdNodeKeyFileNamePattern      = "node-%s-key.pem"    // etcd node key pattern (node-aa1-key.pem)
	EtcdMemberCertFileNamePattern   = "member-%s.pem"      // etcd member certificate pattern (member-aa1.pem)
	EtcdMemberKeyFileNamePattern    = "member-%s-key.pem"  // etcd member key pattern (member-aa1-key.pem)
	EtcdAdminCertFileNamePattern    = "admin-%s.pem"       // etcd admin certificate pattern (admin-aa1.pem)
	EtcdAdminKeyFileNamePattern     = "admin-%s-key.pem"   // etcd admin key pattern (admin-aa1-key.pem)
	
	// Additional certificate file names
	KubeletClientCertFileName       = "kubelet-client.crt"  // Kubelet client certificate
	KubeletClientKeyFileName        = "kubelet-client.key"  // Kubelet client private key
	KubeletServerCertFileName       = "kubelet-server.crt"  // Kubelet server certificate
	KubeletServerKeyFileName        = "kubelet-server.key"  // Kubelet server private key
	KubeProxyClientCertFileName     = "kube-proxy.crt"      // Kube-proxy client certificate
	KubeProxyClientKeyFileName      = "kube-proxy.key"      // Kube-proxy client private key
	
	// Certificate key algorithms and sizes
	DefaultCertificateKeyAlgorithm  = "RSA"                 // Default certificate key algorithm
	DefaultCertificateKeySize       = 2048                  // Default certificate key size
	DefaultCertificateValidityDays  = 365                   // Default certificate validity in days
	DefaultCACertificateValidityDays = 3650                 // Default CA certificate validity in days
	
	// Certificate extensions and usage
	DefaultCertificateKeyUsage      = "Digital Signature, Key Encipherment" // Default certificate key usage
	DefaultCACertificateKeyUsage    = "Digital Signature, Key Encipherment, Certificate Signing" // Default CA certificate key usage
	DefaultCertificateExtKeyUsage   = "Server Authentication, Client Authentication" // Default certificate extended key usage
)

// PKI directory paths
const (
	// Kubernetes PKI directories
	DefaultKubernetesPKIDir         = "/etc/kubernetes/pki"             // Default Kubernetes PKI directory
	DefaultKubernetesPKIEtcdDir     = "/etc/kubernetes/pki/etcd"        // Default Kubernetes etcd PKI directory
	DefaultKubernetesPKIFrontProxyDir = "/etc/kubernetes/pki/front-proxy" // Default Kubernetes front-proxy PKI directory
	
	// etcd PKI directories
	DefaultEtcdPKIDir               = "/etc/etcd/pki"                   // Default etcd PKI directory
	DefaultEtcdPKISSLDir            = "/etc/ssl/etcd"                   // Alternative etcd PKI directory
	DefaultEtcdPKILocalDir          = "/etc/etcd/ssl"                   // Local etcd PKI directory
	
	// Kubelet PKI directories
	DefaultKubeletPKIDir            = "/var/lib/kubelet/pki"            // Default kubelet PKI directory
	DefaultKubeletCertsDir          = "/etc/kubernetes/kubelet"         // Default kubelet certificates directory
	
	// Local PKI working directories
	DefaultLocalPKIWorkDir          = "pki"                             // Default local PKI working directory
	DefaultLocalEtcdPKIWorkDir      = "pki/etcd"                        // Default local etcd PKI working directory
	DefaultLocalKubernetesPKIWorkDir = "pki/kubernetes"                 // Default local Kubernetes PKI working directory
)

// Certificate generation constants
const (
	// Certificate subject organization
	DefaultKubexmCertificateOrganization = "kubexm"                         // Default certificate organization for kubexm
	DefaultCertificateCountry       = "CN"                             // Default certificate country
	DefaultCertificateProvince      = "Beijing"                        // Default certificate province
	DefaultCertificateCity          = "Beijing"                        // Default certificate city
	DefaultCertificateOrganizationUnit = "kubexm-cluster"              // Default certificate organization unit
	
	// Certificate subject common name patterns
	KubernetesAPICertCNPattern      = "kube-apiserver"                  // Kubernetes API server certificate CN pattern
	KubernetesAPICertCN             = "kubernetes"                      // Kubernetes API server certificate CN
	EtcdServerCertCNPattern         = "etcd-server"                     // etcd server certificate CN pattern
	EtcdPeerCertCNPattern           = "etcd-peer"                       // etcd peer certificate CN pattern
	EtcdClientCertCNPattern         = "etcd-client"                     // etcd client certificate CN pattern
	KubeletServerCertCNPattern      = "kubelet-server"                  // Kubelet server certificate CN pattern
	KubeletClientCertCNPattern      = "kubelet-client"                  // Kubelet client certificate CN pattern
	
	// Certificate DNS names and IP addresses
	DefaultKubernetesServiceName    = "kubernetes"                      // Default Kubernetes service name
	DefaultKubernetesServiceFQDN    = "kubernetes.default.svc.cluster.local" // Default Kubernetes service FQDN
	DefaultAPIServerServiceName     = "kube-apiserver"                  // Default API server service name
	DefaultEtcdServiceName          = "etcd"                            // Default etcd service name
	DefaultEtcdServiceFQDN          = "etcd.kube-system.svc.cluster.local" // Default etcd service FQDN
)

// Certificate validation and renewal constants
const (
	// Certificate expiration warnings
	DefaultCertExpirationWarningDays = 30                              // Default certificate expiration warning days
	DefaultCertExpirationCriticalDays = 7                              // Default certificate expiration critical days
	
	// Certificate renewal thresholds
	DefaultCertRenewalThresholdDays = 30                               // Default certificate renewal threshold days
	DefaultCertRenewalCheckInterval = 24                               // Default certificate renewal check interval in hours
	
	// Certificate backup settings
	DefaultCertBackupRetentionDays  = 90                               // Default certificate backup retention days
	DefaultCertBackupEnabled        = true                             // Default certificate backup enabled
)

// Certificate file permissions and ownership
const (
	// File permissions for certificates and keys
	DefaultCertificateFilePermission = 0644                            // Default certificate file permission
	DefaultPrivateKeyFilePermission  = 0600                            // Default private key file permission
	DefaultCertificateDirPermission  = 0755                            // Default certificate directory permission
	DefaultPrivateKeyDirPermission   = 0700                            // Default private key directory permission
	
	// Certificate and key file ownership
	DefaultCertificateFileOwner     = "root"                           // Default certificate file owner
	DefaultCertificateFileGroup     = "root"                           // Default certificate file group
	DefaultPrivateKeyFileOwner      = "root"                           // Default private key file owner
	DefaultPrivateKeyFileGroup      = "root"                           // Default private key file group
	
	// Etcd specific permissions
	DefaultEtcdCertFilePermission   = 0644                             // Default etcd certificate file permission
	DefaultEtcdKeyFilePermission    = 0600                             // Default etcd private key file permission
	DefaultEtcdCertDirPermission    = 0755                             // Default etcd certificate directory permission
	DefaultEtcdKeyDirPermission     = 0700                             // Default etcd private key directory permission
)