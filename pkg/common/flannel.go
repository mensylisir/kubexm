package common

const (
	DefaultFlannelBackendConfigType        = "vxlan"
	FlannelBackendConfigTypeVxlan          = "vxlan"
	FlannelBackendConfigTypeHostGw         = "host-gw"
	FlannelBackendConfigTypeUdp            = "udp"
	FlannelBackendConfigTypeIpsec          = "ipsec"
	DefaultFlannelVXLANConfigVNI           = 1
	DefaultFlannelVXLANConfigPort          = 8472
	DefaultFlannelVXLANConfigDirectRouting = false
)
