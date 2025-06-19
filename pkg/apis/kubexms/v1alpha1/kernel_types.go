package v1alpha1
import "strings" // Added for strings.TrimSpace

// KernelConfig holds configuration for kernel module loading and sysctl parameters.
type KernelConfig struct {
	// Modules is a list of kernel modules to be loaded on hosts.
	Modules []string `json:"modules,omitempty"`
	// SysctlParams is a map of sysctl parameters to set.
	// Example: {"net.bridge.bridge-nf-call-iptables": "1"}
	SysctlParams map[string]string `json:"sysctlParams,omitempty"`
}

// SetDefaults_KernelConfig sets default values for KernelConfig.
func SetDefaults_KernelConfig(cfg *KernelConfig) {
	if cfg == nil {
		return
	}
	if cfg.Modules == nil {
		cfg.Modules = []string{}
	}
	if cfg.SysctlParams == nil {
		cfg.SysctlParams = make(map[string]string)
	}
	// Common sysctl params can be defaulted here, e.g. for bridge networking
	// if _, exists := cfg.SysctlParams["net.bridge.bridge-nf-call-iptables"]; !exists {
	//    cfg.SysctlParams["net.bridge.bridge-nf-call-iptables"] = "1"
	// }
}

// Validate_KernelConfig validates KernelConfig.
func Validate_KernelConfig(cfg *KernelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, module := range cfg.Modules {
	   if strings.TrimSpace(module) == "" {
		   verrs.Add("%s.modules[%d]: module name cannot be empty", pathPrefix, i)
	   }
	}
	for key, val := range cfg.SysctlParams {
	   if strings.TrimSpace(key) == "" {
		   verrs.Add("%s.sysctlParams: sysctl key cannot be empty (value: '%s')", pathPrefix, val)
	   }
	   // Could also validate that val is not empty if that's a requirement
	}
}
