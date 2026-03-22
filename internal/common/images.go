package common

const (
	DefaultPauseImage             = "registry.k8s.io/pause:3.9"
	DefaultK8sImageRegistry       = "registry.k8s.io"
	DefaultCoreDNSImageRepository = DefaultK8sImageRegistry + "/coredns"
	DefaultPauseImageRepository   = DefaultK8sImageRegistry
	DefaultKubeVIPImageRepository = "ghcr.io/kube-vip"
	DefaultHAProxyImageRepository = "docker.io/library/haproxy"
	DefaultNginxImageRepository   = "docker.io/library/nginx"
)

const (
	CnRegistry                = "registry.cn-beijing.aliyuncs.com"
	CnNamespaceOverride       = "kubexmio"
	DefaultKubeImageNamespace = "kubexm"
)
