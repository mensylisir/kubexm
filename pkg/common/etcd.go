package common

type EtcdDeploymentType string

const (
	EtcdDeploymentTypeKubeadm          EtcdDeploymentType = "kubeadm"
	EtcdDeploymentTypeKubexm           EtcdDeploymentType = "kubexm"
	EtcdDeploymentTypeExternal         EtcdDeploymentType = "external"
	DefaultEtcdClusterToken                               = "kubexm-etcd-default-token"
	DefaultEtcdKeepBackups                                = 7
	DefaultEtcdHeartbeatInterval                          = 250
	DefaultEtcdElectionTimeout                            = 5000
	DefaultEtcdSnapshotCount                              = 10000
	DefaultEtcdAutoCompactionRetention                    = 8
	DefaultEtcdQuotaBackendBytes                          = 2147483648
	DefaultEtcdMaxRequestBytes                            = 1572864
	DefaultEtcdMetrics                                    = "basic"
	DefaultEtcdLogLevel                                   = "info"
	DefaultEtcdMaxSnapshots                               = 5
	DefaultEtcdMaxWALs                                    = 5
)

const (
	EtcdDefaultDataDirTarget    = "/var/lib/etcd"
	EtcdDefaultWalDir           = "/var/lib/etcd/wal"
	EtcdDefaultConfDirTarget    = "/etc/etcd"
	EtcdDefaultPKIDirTarget     = "/etc/etcd/pki"
	EtcdEnvFileTarget           = "/etc/etcd.env"
	EtcdSystemdFile             = "/etc/systemd/system/etcd.service"
	EtcdDropInFile              = "/etc/systemd/system/etcd.service.d/kubexm.conf"
	DefaultKubernetesPKIEtcdDir = "/etc/kubernetes/pki/etcd"
	DefaultEtcdPKIDir           = "/etc/etcd/pki"
	DefaultEtcdPKISSLDir        = "/etc/ssl/etcd"
	DefaultEtcdPKILocalDir      = "/etc/etcd/ssl"
	DefaultEtcdPKISSLPath       = "/etc/ssl/etcd/ssl"
	DefaultEtcdPath             = "/var/lib/etcd"
	DefaultEtcdConfig           = "/etc/etcd.conf"
	DefaultEtcdBackupDir        = "/var/backups/etcd"
	DefaultEtcdScriptPath       = "/usr/local/kubexm/bin/etcd.sh"
)

const (
	EtcdCaPemFileName             = "ca.pem"
	EtcdCaKeyPemFileName          = "ca-key.pem"
	EtcdNodeCertFileNamePattern   = "node-%s.pem"       // etcd node certificate pattern (node-aa1.pem)
	EtcdNodeKeyFileNamePattern    = "node-%s-key.pem"   // etcd node key pattern (node-aa1-key.pem)
	EtcdMemberCertFileNamePattern = "member-%s.pem"     // etcd member certificate pattern (member-aa1.pem)
	EtcdMemberKeyFileNamePattern  = "member-%s-key.pem" // etcd member key pattern (member-aa1-key.pem)
	EtcdAdminCertFileNamePattern  = "admin-%s.pem"      // etcd admin certificate pattern (admin-aa1.pem)
	EtcdAdminKeyFileNamePattern   = "admin-%s-key.pem"  // etcd admin key pattern (admin-aa1-key.pem)

	EtcdCaCertFileName           = "ca.crt"
	EtcdCaKeyFileName            = "ca.key"
	EtcdServerCertFileName       = "server.crt"
	EtcdServerKeyFileName        = "server.key"
	EtcdPeerCertFileName         = "peer.crt"
	EtcdPeerKeyFileName          = "peer.key"
	EtcdAdminClientCertFileName  = "admin.crt"
	EtcdAdminClientKeyFileName   = "admin.key"
	EtcdHealthClientCertFileName = "healthcheck.crt"
	EtcdHealthClientKeyFileName  = "healthcheck.key"
)
