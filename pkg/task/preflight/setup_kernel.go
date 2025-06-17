package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/preflight"
	"github.com/kubexms/kubexms/pkg/task"
)

// NewSetupKernelTask creates a task to configure kernel parameters and load modules.
// cfg can be used to make modules and params configurable in the future.
func NewSetupKernelTask(cfg *config.Cluster) *task.Task {
	// These values would ideally come from cfg (e.g., cfg.Spec.Kernel.Modules)
	// For now, they are hardcoded common K8s prerequisites.
	kernelModules := []string{"br_netfilter", "overlay", "ip_vs"}
	// Some systems might require more specific module names like nf_conntrack for certain ip_vs functionalities,
	// but ip_vs itself is often the primary one to ensure.
	// Consider adding nf_conntrack if issues arise: "nf_conntrack" (or "nf_conntrack_ipv4" on older systems)

	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		// For IPVS, common settings (ensure ip_vs module is loaded first):
		// These might not be strictly necessary for all IPVS setups but are common recommendations.
		// "net.ipv4.vs.conn_reuse_mode":         "0", // Set to 0 to disable connection reuse for some schedulers
        // "net.ipv4.vs.expire_nodest_conn":    "1", // Expire connection if destination server is not available
        // "net.ipv4.vs.expire_quiescent_template": "1", // Expire quiescent services
	}
	// Config file path for sysctl can also be from cfg or a constant.
	// Using the default from SetSystemConfigStep if empty there, or specify one here.
	sysctlConfigPath := "/etc/sysctl.d/90-kubexms-kernel.conf"


	return &task.Task{
		Name: "Setup Kernel Parameters and Modules",
		RunOnRoles: []string{}, // Typically run on all nodes that will be part of Kubernetes
		Steps: []step.Step{
			&preflight.LoadKernelModulesStep{
				Modules: kernelModules,
			},
			&preflight.SetSystemConfigStep{
				Params:         sysctlParams,
				ConfigFilePath: sysctlConfigPath,
				// Reload defaults to true in SetSystemConfigStep if Reload field is nil
			},
		},
		Concurrency: 10,
		IgnoreError: false,
	}
}
