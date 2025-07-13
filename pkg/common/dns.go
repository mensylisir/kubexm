package common

const DomainValidationRegexString = `^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`

const (
	DefaultCoreDNSUpstreamGoogle     = "8.8.8.8"
	DefaultCoreDNSUpstreamCloudflare = "1.1.1.1"
	DefaultExternalZoneCacheSeconds  = 300
	DefaultDNSServiceIP              = "10.96.0.10"
	DefaultDNSClusterDomain          = "cluster.local"
	DefaultDNSUpstream               = "8.8.8.8"
	DefaultDNSSecondary              = "8.8.4.4"
	DefaultCoreDNSVersion            = "v1.10.1"
)

const (
	CoreDNSConfigMapName           = "coredns"
	CoreDNSDeploymentName          = "coredns"
	CoreDNSServiceName             = "kube-dns"
	CoreDNSAutoscalerConfigMapName = "coredns-autoscaler"
)
