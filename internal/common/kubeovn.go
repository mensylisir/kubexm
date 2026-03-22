package common

const (
	DefaultKubeOvnNetworkingTunnelType          = "geneve"
	KubeOvnNetworkingTunnelTypeGeneve           = "geneve"
	KubeOvnNetworkingTunnelVXLAN                = "vxlan"
	DefaultKubeOvnNetworkingMTU                 = 1500
	KubeOvnControllerConfigJoinCIDR             = "100.64.0.1/16"
	KubeOvnControllerConfigNodeSwitchCIDR       = "100.64.0.0/16"
	KubeOvnControllerConfigPodDefaultSubnetCIDR = "10.16.0.0/16"
	KubeOvnAdvancedFeaturesEnableSSL            = true
	KubeOvnAdvancedFeaturesEnableVPCNATGateway  = false
	KubeOvnAdvancedFeaturesEnableSubnetQoS      = false
)
