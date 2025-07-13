package common

type InternalLoadBalancerType string

const (
	InternalLBTypeKubeVIP InternalLoadBalancerType = "kube-vip"
	InternalLBTypeHAProxy InternalLoadBalancerType = "haproxy"
	InternalLBTypeNginx   InternalLoadBalancerType = "nginx"
)

type ExternalLoadBalancerType string

const (
	ExternalLBTypeKubexmKH ExternalLoadBalancerType = "kubexm-kh"
	ExternalLBTypeKubexmKN ExternalLoadBalancerType = "kubexm-kn"
	ExternalLBTypeExternal ExternalLoadBalancerType = "external"
	ExternalLBTypeNone     ExternalLoadBalancerType = ""
)
