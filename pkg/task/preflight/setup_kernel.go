package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	stepPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
)

// NewSetupKernelTask creates a task specification to configure kernel parameters and load modules.
func NewSetupKernelTask(cfg *config.Cluster) *spec.TaskSpec {
	// Default values
	kernelModules := []string{"br_netfilter", "overlay", "ip_vs"}
	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		// These IPVS params are often recommended but can be conditional
		// "net.ipv4.vs.conn_reuse_mode":         "0",
        // "net.ipv4.vs.expire_nodest_conn":    "1",
        // "net.ipv4.vs.expire_quiescent_template": "1",
	}
	// Default path for the sysctl config file written by the step.
	// The step itself has a default, but task factory can also suggest one.
	sysctlConfigPath := "/etc/sysctl.d/90-kubexms-kernel.conf"
	reloadSysctl := true // Default to reloading sysctl settings after writing config

	// Override with values from config if provided
	if cfg != nil { // cfg itself can be nil
		// KernelConfig is a struct, not a pointer, so it's always there if cfg.Spec is.
		if len(cfg.Spec.KernelConfig.Modules) > 0 {
			kernelModules = cfg.Spec.KernelConfig.Modules
		}
		if len(cfg.Spec.KernelConfig.SysctlParams) > 0 {
			// Policy for sysctlParams: config replaces defaults entirely if provided.
			// A merge strategy could be implemented if needed.
			sysctlParams = cfg.Spec.KernelConfig.SysctlParams
		}
		// Example for SysctlConfigFilePath if it were added to KernelConfigSpec:
		// if cfg.Spec.KernelConfig.SysctlConfigFilePath != "" {
		//    sysctlConfigPath = cfg.Spec.KernelConfig.SysctlConfigFilePath
		// }
	}

	return &spec.TaskSpec{
		Name: "Setup Kernel Parameters and Modules",
		RunOnRoles: []string{},
		Steps: []spec.StepSpec{
			&stepPreflight.LoadKernelModulesStepSpec{
				Modules: kernelModules,
			},
			&stepPreflight.SetSystemConfigStepSpec{
				Params:         sysctlParams,
				ConfigFilePath: sysctlConfigPath,
				Reload:         &reloadSysctl, // Pass pointer to bool for reload
			},
		},
		Concurrency: 10,
		IgnoreError: false, // Kernel setup is usually critical
	}
}
