package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	stepPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
	// "github.com/kubexms/kubexms/pkg/task" // No longer needed
)

// NewSetupKernelTask creates a task specification to configure kernel parameters and load modules.
// cfg can be used to make modules and params configurable.
func NewSetupKernelTask(cfg *config.Cluster) *spec.TaskSpec {
	// Default values, could be overridden by cfg
	kernelModules := []string{"br_netfilter", "overlay", "ip_vs"}
	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		// IPVS settings might be conditional based on config (e.g. if IPVS is the chosen kube-proxy mode)
		// "net.ipv4.vs.conn_reuse_mode":         "0",
        // "net.ipv4.vs.expire_nodest_conn":    "1",
        // "net.ipv4.vs.expire_quiescent_template": "1",
	}
	sysctlConfigPath := "/etc/sysctl.d/90-kubexms-kernel.conf" // Default path
	reloadSysctl := true // Default to reloading sysctl settings

	// Example: Override from cfg if structure exists
	// if cfg != nil && cfg.Spec.KernelSetup != nil {
	//    if len(cfg.Spec.KernelSetup.Modules) > 0 {
	//        kernelModules = cfg.Spec.KernelSetup.Modules
	//    }
	//    if len(cfg.Spec.KernelSetup.SysctlParams) > 0 {
	//        sysctlParams = cfg.Spec.KernelSetup.SysctlParams
	//    }
	//    if cfg.Spec.KernelSetup.SysctlConfigPath != "" {
	//        sysctlConfigPath = cfg.Spec.KernelSetup.SysctlConfigPath
	//    }
	//    if cfg.Spec.KernelSetup.ReloadSysctl != nil { // Assuming ReloadSysctl is a *bool in config
	//        reloadSysctl = *cfg.Spec.KernelSetup.ReloadSysctl
	//    }
	// }


	return &spec.TaskSpec{
		Name: "Setup Kernel Parameters and Modules",
		RunOnRoles: []string{}, // Typically for all nodes
		Steps: []spec.StepSpec{
			&stepPreflight.LoadKernelModulesStepSpec{
				Modules: kernelModules,
			},
			&stepPreflight.SetSystemConfigStepSpec{
				Params:         sysctlParams,
				ConfigFilePath: sysctlConfigPath,
				Reload:         &reloadSysctl, // Pass pointer to bool
			},
		},
		Concurrency: 10,
		IgnoreError: false, // Kernel setup is usually critical
	}
}
