package common

// System Configuration Defaults.
const (
	DefaultSELinuxMode  = "permissive" // Default SELinux mode.
	DefaultIPTablesMode = "legacy"     // Default IPTable mode.
)

// Valid System Configuration Values for certain string enum-like fields.
var (
	ValidSELinuxModes  = []string{"permissive", "enforcing", "disabled", ""} // Empty allows no-op/system default
	ValidIPTablesModes = []string{"legacy", "nft", ""}                      // Empty allows no-op/system default
)

// Essential Kernel Modules for Kubernetes.
const (
	KernelModuleBrNetfilter = "br_netfilter" // Kernel module for bridge netfilter.
	KernelModuleIpvs        = "ip_vs"        // Kernel module for IPVS.
)
