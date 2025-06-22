package config

import (
	"time"
	// "fmt" // Removed as unused
	// "strings" // May be needed for some default logic
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Added import for v1alpha1 types
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
	if cfg.ObjectMeta.Name == "" { // Changed Metadata to ObjectMeta
		// cfg.ObjectMeta.Name = "kubexms-cluster" // Example default name, validation might enforce presence
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
		// host.WorkDir removed as it's not in v1alpha1.HostSpec
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
		    host.Taints = []v1alpha1.TaintSpec{} // Changed to v1alpha1.TaintSpec
		}
	}

	// ContainerRuntime defaults
	if cfg.Spec.ContainerRuntime == nil {
		cfg.Spec.ContainerRuntime = &v1alpha1.ContainerRuntimeConfig{} // Changed to v1alpha1.ContainerRuntimeConfig
	}
	if cfg.Spec.ContainerRuntime.Type == "" {
		cfg.Spec.ContainerRuntime.Type = v1alpha1.ContainerRuntimeContainerd // Default runtime using const
	}
	// cfg.Spec.ContainerRuntime.Version could default here if desired.

	// ContainerdSpec defaults (if containerd is the type)
	if cfg.Spec.ContainerRuntime.Type == v1alpha1.ContainerRuntimeContainerd { // Using const
		if cfg.Spec.ContainerRuntime.Containerd == nil { // Field name is Containerd in v1alpha1.ContainerRuntimeConfig
			cfg.Spec.ContainerRuntime.Containerd = &v1alpha1.ContainerdConfig{} // Changed to v1alpha1.ContainerdConfig
		}
		// UseSystemdCgroup is a bool, defaults to false if not set.
		// If a true default is desired when the ContainerdSpec section is present:
		// This requires knowing if it was explicitly set to false vs. just omitted.
		// A common pattern for this is using a pointer (*bool) or an explicit "enable" field.
		// Given current struct, if we want `UseSystemdCgroup: true` to be the default
		// when `containerd:` section exists, we can't distinguish "omitted" from "false".
		// Let's assume steps will default it to true if not specified, or config must be explicit.
		// Here, we just initialize maps/slices.
		// Accessing through cfg.Spec.ContainerRuntime.Containerd
		if cfg.Spec.ContainerRuntime.Containerd.RegistryMirrors == nil {
		    cfg.Spec.ContainerRuntime.Containerd.RegistryMirrors = make(map[string][]string)
		}
		if cfg.Spec.ContainerRuntime.Containerd.InsecureRegistries == nil {
		    cfg.Spec.ContainerRuntime.Containerd.InsecureRegistries = []string{}
		}
	}

	// EtcdSpec defaults
	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &v1alpha1.EtcdConfig{} // Changed to v1alpha1.EtcdConfig
	}
	if cfg.Spec.Etcd.Type == "" {
		cfg.Spec.Etcd.Type = v1alpha1.EtcdTypeKubeXMSInternal // Default to stacked etcd using const
	}
	// v1alpha1.EtcdConfig does not have a direct Nodes []string field.
	// External Etcd has Endpoints. Stacked Etcd nodes are derived from HostSpec roles.
	// So, initializing cfg.Spec.Etcd.Nodes is removed.


	// KubernetesSpec defaults
	if cfg.Spec.Kubernetes == nil {
		cfg.Spec.Kubernetes = &v1alpha1.KubernetesConfig{} // Changed to v1alpha1.KubernetesConfig
	}
	if cfg.Spec.Kubernetes.ClusterName == "" && cfg.ObjectMeta.Name != "" { // Changed Metadata to ObjectMeta
		cfg.Spec.Kubernetes.ClusterName = cfg.ObjectMeta.Name // Changed Metadata to ObjectMeta
	}
	// PodSubnet and ServiceSubnet are now in NetworkConfig (KubePodsCIDR, KubeServiceCIDR)
	// Their defaults should be handled by SetDefaults_NetworkConfig if any.
	// if cfg.Spec.Kubernetes.PodSubnet == "" {
	// // cfg.Spec.Kubernetes.PodSubnet = "10.244.0.0/16" // Common default, but better set by user or CNI
	// }
	// if cfg.Spec.Kubernetes.ServiceSubnet == "" {
	// // cfg.Spec.Kubernetes.ServiceSubnet = "10.96.0.0/12" // Common default
	// }
	if cfg.Spec.Kubernetes.FeatureGates == nil {
	    cfg.Spec.Kubernetes.FeatureGates = make(map[string]bool)
	}
	// Initialize sub-specs if nil
	if cfg.Spec.Kubernetes.APIServer == nil { cfg.Spec.Kubernetes.APIServer = &v1alpha1.APIServerConfig{} } // Changed
	if cfg.Spec.Kubernetes.ControllerManager == nil { cfg.Spec.Kubernetes.ControllerManager = &v1alpha1.ControllerManagerConfig{} } // Changed
	if cfg.Spec.Kubernetes.Scheduler == nil { cfg.Spec.Kubernetes.Scheduler = &v1alpha1.SchedulerConfig{} } // Changed
	if cfg.Spec.Kubernetes.Kubelet == nil { cfg.Spec.Kubernetes.Kubelet = &v1alpha1.KubeletConfig{} } // Changed
	if cfg.Spec.Kubernetes.KubeProxy == nil { cfg.Spec.Kubernetes.KubeProxy = &v1alpha1.KubeProxyConfig{} } // Changed


	// NetworkSpec defaults
	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &v1alpha1.NetworkConfig{} // Changed
	}
	if cfg.Spec.Network.Plugin == "" {
		// cfg.Spec.Network.Plugin = "calico" // Example default CNI
	}

	// HighAvailabilitySpec defaults
	if cfg.Spec.HighAvailability == nil {
		cfg.Spec.HighAvailability = &v1alpha1.HighAvailabilityConfig{} // Changed
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
	    cfg.Spec.Addons = []string{} // Corrected type to []string
	}
	// Defaults for individual addon fields (like Enabled) are usually handled by the addon's own logic/spec.
}

// Removed GetControlPlaneHosts helper function as it was unused and incorrectly defined.
// If needed later, it can be implemented correctly as a standalone function or method on the appropriate type.

// Helper method (example) for APIServerSpec, if it had an ExternalLoadBalancer field
// func (as *APIServerSpec) ExternalLoadBalancer() string {
//     return as.LoadBalancerHost // if such a field existed
// }
