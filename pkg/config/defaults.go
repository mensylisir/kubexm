package config

import (
	"time"
	// "strings" // May be needed for some default logic
)

// SetDefaults applies default values to the Cluster configuration
// for fields that were not explicitly set by the user.
// This function modifies the passed cfg *Cluster in place.
func SetDefaults(cfg *Cluster) {
	if cfg == nil {
		return // Cannot set defaults on a nil config
	}

	// Default APIVersion and Kind if not set
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	if cfg.Kind == "" {
		cfg.Kind = ClusterKind
	}

	// Default Metadata if not set (e.g., Name might be required by validation later if still empty)
	if cfg.Metadata.Name == "" {
		// cfg.Metadata.Name = "kubexms-cluster" // Example default name, validation might enforce presence
	}

	// GlobalSpec defaults
	// Initialize GlobalSpec if it's a zero struct (not a pointer, so always present)
	// No explicit check needed unless we make GlobalSpec a pointer.

	if cfg.Spec.Global.Port == 0 {
		cfg.Spec.Global.Port = 22 // Default SSH port
	}
	if cfg.Spec.Global.ConnectionTimeout == 0 {
		cfg.Spec.Global.ConnectionTimeout = 30 * time.Second
	}
	if cfg.Spec.Global.WorkDir == "" {
		cfg.Spec.Global.WorkDir = "/tmp/kubexms_work" // Default remote work directory
	}
	// cfg.Spec.Global.User might default to current OS user, or be required. Let validation handle if empty.
	// cfg.Spec.Global.PrivateKeyPath might default to "~/.ssh/id_rsa". Let validation handle if critical and empty.


	// HostSpec defaults (iterate through hosts)
	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i] // Get pointer to modify in place

		if host.Port == 0 {
			host.Port = cfg.Spec.Global.Port // Inherit from global
		}
		if host.User == "" {
			host.User = cfg.Spec.Global.User // Inherit from global
		}
		if host.PrivateKeyPath == "" {
			host.PrivateKeyPath = cfg.Spec.Global.PrivateKeyPath // Inherit from global
		}
		if host.WorkDir == "" {
			host.WorkDir = cfg.Spec.Global.WorkDir // Inherit from global
			if host.WorkDir == "" { // Fallback if global was also empty
				host.WorkDir = fmt.Sprintf("/tmp/kubexms_work_%s", host.Name) // Host-specific fallback
			}
		}
		if host.Type == "" {
			host.Type = "ssh" // Default host type
		}
		// Initialize slices/maps if nil to prevent nil pointer dereferences later
		if host.Roles == nil {
		    host.Roles = []string{}
		}
		if host.Labels == nil {
		    host.Labels = make(map[string]string)
		}
		if host.Taints == nil {
		    host.Taints = []TaintSpec{}
		}
	}

	// ContainerRuntime defaults
	if cfg.Spec.ContainerRuntime == nil {
		cfg.Spec.ContainerRuntime = &ContainerRuntimeSpec{}
	}
	if cfg.Spec.ContainerRuntime.Type == "" {
		cfg.Spec.ContainerRuntime.Type = "containerd" // Default runtime
	}
	// cfg.Spec.ContainerRuntime.Version could default here if desired.

	// ContainerdSpec defaults (if containerd is the type)
	if cfg.Spec.ContainerRuntime.Type == "containerd" {
		if cfg.Spec.Containerd == nil {
			cfg.Spec.Containerd = &ContainerdSpec{} // Initialize if nil
		}
		// UseSystemdCgroup is a bool, defaults to false if not set.
		// If a true default is desired when the ContainerdSpec section is present:
		// This requires knowing if it was explicitly set to false vs. just omitted.
		// A common pattern for this is using a pointer (*bool) or an explicit "enable" field.
		// Given current struct, if we want `UseSystemdCgroup: true` to be the default
		// when `containerd:` section exists, we can't distinguish "omitted" from "false".
		// Let's assume steps will default it to true if not specified, or config must be explicit.
		// Here, we just initialize maps/slices.
		if cfg.Spec.Containerd.RegistryMirrors == nil { // Was RegistryMirrorsConfig
		    cfg.Spec.Containerd.RegistryMirrors = make(map[string][]string)
		}
		if cfg.Spec.Containerd.InsecureRegistries == nil {
		    cfg.Spec.Containerd.InsecureRegistries = []string{}
		}
	}

	// EtcdSpec defaults
	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdSpec{}
	}
	if cfg.Spec.Etcd.Type == "" {
		cfg.Spec.Etcd.Type = "stacked" // Default to stacked etcd
	}
	if cfg.Spec.Etcd.Nodes == nil {
	    cfg.Spec.Etcd.Nodes = []string{}
	}


	// KubernetesSpec defaults
	if cfg.Spec.Kubernetes == nil {
		cfg.Spec.Kubernetes = &KubernetesSpec{}
	}
	if cfg.Spec.Kubernetes.ClusterName == "" && cfg.Metadata.Name != "" {
		cfg.Spec.Kubernetes.ClusterName = cfg.Metadata.Name
	}
	if cfg.Spec.Kubernetes.PodSubnet == "" {
		// cfg.Spec.Kubernetes.PodSubnet = "10.244.0.0/16" // Common default, but better set by user or CNI
	}
	if cfg.Spec.Kubernetes.ServiceSubnet == "" {
		// cfg.Spec.Kubernetes.ServiceSubnet = "10.96.0.0/12" // Common default
	}
	if cfg.Spec.Kubernetes.FeatureGates == nil {
	    cfg.Spec.Kubernetes.FeatureGates = make(map[string]bool)
	}
	// Initialize sub-specs if nil
	if cfg.Spec.Kubernetes.APIServer == nil { cfg.Spec.Kubernetes.APIServer = &APIServerSpec{} }
	if cfg.Spec.Kubernetes.ControllerManager == nil { cfg.Spec.Kubernetes.ControllerManager = &CMKSpec{} }
	if cfg.Spec.Kubernetes.Scheduler == nil { cfg.Spec.Kubernetes.Scheduler = &SchedulerSpec{} }
	if cfg.Spec.Kubernetes.Kubelet == nil { cfg.Spec.Kubernetes.Kubelet = &KubeletSpec{} }
	if cfg.Spec.Kubernetes.KubeProxy == nil { cfg.Spec.Kubernetes.KubeProxy = &KubeProxySpec{} }


	// NetworkSpec defaults
	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &NetworkSpec{}
	}
	if cfg.Spec.Network.Plugin == "" {
		// cfg.Spec.Network.Plugin = "calico" // Example default CNI
	}

	// HighAvailabilitySpec defaults
	if cfg.Spec.HighAvailability == nil {
		cfg.Spec.HighAvailability = &HighAvailabilitySpec{}
	}
	// Example: Default HA type if multiple control plane nodes and no external LB specified
	// This requires more complex logic based on other fields (e.g. APIServerSpec.ExternalLoadBalancer)
	// and the number of hosts with control-plane roles.
	// if cfg.Spec.HighAvailability.Type == "" &&
	//    (cfg.Spec.Kubernetes.APIServer == nil || cfg.Spec.Kubernetes.APIServer.ExternalLoadBalancer == "") && // Assuming such a field
	//    len(cfg.Spec.GetControlPlaneHosts()) > 1 {
	// 	// cfg.Spec.HighAvailability.Type = "keepalived"
	// }


	// Addons: Initialize slices if nil
	if cfg.Spec.Addons == nil {
	    cfg.Spec.Addons = []AddonSpec{}
	}
	// Defaults for individual addon fields (like Enabled) are usually handled by the addon's own logic/spec.
}

// Helper method (example, could be on ClusterSpec) to get control plane hosts
// Used for HA defaulting logic example.
func (cs *ClusterSpec) GetControlPlaneHosts() []HostSpec {
    var cpHosts []HostSpec
    for _, h := range cs.Hosts {
        for _, role := range h.Roles {
            if role == "master" || role == "control-plane" {
                cpHosts = append(cpHosts, h)
                break
            }
        }
    }
    return cpHosts
}

// Helper method (example) for APIServerSpec, if it had an ExternalLoadBalancer field
// func (as *APIServerSpec) ExternalLoadBalancer() string {
//     return as.LoadBalancerHost // if such a field existed
// }
