package common

type CNIType string

const (
	CNITypeCalico    CNIType = "calico"
	CNITypeFlannel   CNIType = "flannel"
	CNITypeCilium    CNIType = "cilium"
	CNITypeKubeOvn   CNIType = "kube-ovn"
	CNITypeMultus    CNIType = "multus"
	CNITypeHybridnet CNIType = "hybridnet"
)

const (
	DefaultCNIConfDirTarget = "/etc/cni/net.d"
	DefaultCNIBinDirTarget  = "/opt/cni/bin"
	DefaultCniPath          = "/opt/cni/bin" // Default CNI plugin path
	DefaultCniConfigPath    = "/etc/cni/net.d"
	DefaultCniConfig        = "10-kubexm.conf"
)

const (
	DefaultKubePodsCIDR    = "10.244.0.0/16"
	DefaultKubeServiceCIDR = "10.96.0.0/12"
	DefaultKubeClusterCIDR = "10.244.0.0/16"
)
