package common

const (
	DefaultAddonNamespace = "kube-system"

	// Addon related constants
	ValidChartVersionRegexString = `^v?([0-9]+)(\.[0-9]+){0,2}$` // Allows "latest", "stable", or versions like "1.2.3", "v1.2.3", "1.2", "v1.0", "1", "v2".

	// Endpoint related constants
	DomainValidationRegexString = `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`

	// Kubernetes related constants
	CgroupDriverSystemd  = "systemd"
	CgroupDriverCgroupfs = "cgroupfs"
	KubeProxyModeIPTables = "iptables"
	KubeProxyModeIPVS    = "ipvs"
)
