package common

const (
	CacheKeyControlPlaneEndpoint     = "kubexm.pipeline.controlplane.endpoint"
	CacheKeyKubeadmJoinCommandMaster = "kubexm.pipeline.kubeadm.join.master"
	CacheKeyKubeadmJoinCommandWorker = "kubexm.pipeline.kubeadm.join.worker"
	CacheKeyKubeadmBootstrapToken    = "kubexm.pipeline.kubeadm.bootstraptoken"
	CacheKeyKubeadmCACertHashes      = "kubexm.pipeline.kubeadm.discovery.cacertsha256"
	CacheKeyKubeadmCertificateKey    = "kubexm.pipeline.kubeadm.certificatekey"
	CacheKeyClusterCACert            = "kubexm.pipeline.pki.ca.cert"
	CacheKeyClusterCAKey             = "kubexm.pipeline.pki.ca.key"
	CacheKeyAdminKubeconfig          = "kubexm.pipeline.kubeconfig.admin"
	CacheKeyModuleEtcdEndpoints      = "kubexm.module.etcd.endpoints"
	CacheArchivePathKey              = "shared.package.archive.path"
	CacheExtractedDirKey             = "shared.package.extracted.dir"
	CacheKeyHostFactsTemplate        = "kubexm.facts.host.%s"
)
