package common

type InternalLoadBalancerType string

const (
	InternalLBTypeHAProxy InternalLoadBalancerType = "haproxy"
	InternalLBTypeNginx   InternalLoadBalancerType = "nginx"
	InternalLBTypeKubeVIP InternalLoadBalancerType = "kube-vip"
)

type ExternalLoadBalancerType string

const (
	ExternalLBTypeKubeVIP  ExternalLoadBalancerType = "kube-vip"
	ExternalLBTypeKubexmKH ExternalLoadBalancerType = "kubexm-kh"
	ExternalLBTypeKubexmKN ExternalLoadBalancerType = "kubexm-kn"
	ExternalLBTypeExternal ExternalLoadBalancerType = "external"
	ExternalLBTypeNone     ExternalLoadBalancerType = ""
)
