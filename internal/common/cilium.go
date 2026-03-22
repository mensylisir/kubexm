package common

const (
	CiliumConfigFile                   = "/etc/cilium/cilium.yaml"
	DefaultTunnelingMode               = "vxlan"
	CiliumTunnelVxlanModes             = "vxlan"
	CiliumTunnelGeneveModes            = "geneve"
	CiliumTunnelDisabledModes          = "disabled"
	DefaultCiliumKPRModes              = "strict"
	CiliumKPRProbeModes                = "probe"
	CiliumKPRStrictModes               = "strict"
	CiliumKPRDisabledModes             = "disabled"
	CiliumIdentCrdModes                = "crd"
	CiliumIdentKvstoreModes            = "kvstore"
	CiliumIPAMKubernetesMode           = "kubernetes"
	CiliumIPAMClusterPoolsMode         = "cluster-pool"
	DefaultCiliumIPAMsMode             = "cluster-pool"
	DefaultCiliumBGPControlPlaneEnable = false
	DefaultEnableBPFMasqueradeEnable   = true
	DefaultCiliumHubbleConfigEnable    = true
	DefaultCiliumHubbleConfigUIEnable  = false
	DefaultIdentityAllocationMode      = "crd"
	DefaultEnableEncryption            = false
	DefaultEnableBandwidthManager      = false
)
