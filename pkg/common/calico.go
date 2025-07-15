package common

const (
	DefaultCalicoIPIPMode                            = "Always"
	CalicoIPIPModeAlways                             = "Always"
	CalicoIPIPModeCrossSubnet                        = "CrossSubnet"
	CalicoIPIPModeNever                              = "Never"
	DefaultCalicoVXLANMode                           = "Never"
	CalicoVXLANModeAlways                            = "Always"
	CalicoVXLANModeCrossSubnet                       = "CrossSubnet"
	CalicoVXLANModeNever                             = "Never"
	CalicoBGPConfigurationEnable                     = false
	CalicoTyphaDeploymentEnable                      = true
	CalicoTyphaDeploymentReplicas                    = 2
	CalicoTyphaDeploymentNodeSelector                = "node-role.kubernetes.io/master="
	CalicoIPAMAutoCreatePools                        = true
	CalicoIPPoolCIDR                                 = DefaultKubeClusterCIDR
	CalicoIPPoolEncapsulationIPIP                    = "IPIP"
	CalicoIPPoolEncapsulationIPIPCrossSubnet         = "IPIPCrossSubnet"
	CalicoIPPoolEncapsulationVXLAN                   = "VXLAN"
	CalicoIPPoolEncapsulationVXLANCrossSubnet        = "VXLANCrossSubnet"
	CalicoIPPoolEncapsulationNone                    = "None"
	CalicoIPPoolNatOutgoing                          = true
	DefaultCalicoIPPoolBlockSize                     = 26
	DefaultCalicoIPPoolDisabled                      = false
	CalicoFelixConfigurationLogSeverityScreenDebug   = "Debug"
	CalicoFelixConfigurationLogSeverityScreenInfo    = "Info"
	CalicoFelixConfigurationLogSeverityScreenWarning = "Warning"
	CalicoFelixConfigurationLogSeverityScreenError   = "Error"
	CalicoFelixConfigurationLogSeverityScreenFatal   = "Fatal"
	DefaultCalicoFelixConfigurationLogSeverityScreen = "Info"
)
