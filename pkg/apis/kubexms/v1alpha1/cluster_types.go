package v1alpha1

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusters,scope=Namespaced,shortName=kc
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetes.version",description="Kubernetes Version"
// +kubebuilder:printcolumn:name="Hosts",type="integer",JSONPath=".spec.hostsCount",description="Number of hosts" // Example, need field
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the top-level configuration object.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	// Status ClusterStatus `json:"status,omitempty"` // Add when status is defined
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	Global             *GlobalSpec             `json:"global,omitempty"`
	Hosts              []HostSpec              `json:"hosts"`

	// Component configurations - will be pointers to specific config types
	ContainerRuntime   *ContainerRuntimeConfig `json:"containerRuntime,omitempty"`
	Containerd         *ContainerdConfig       `json:"containerd,omitempty"`
	Etcd               *EtcdConfig             `json:"etcd,omitempty"`
	Kubernetes         *KubernetesConfig       `json:"kubernetes,omitempty"`
	Network            *NetworkConfig          `json:"network,omitempty"`
	HighAvailability   *HighAvailabilityConfig `json:"highAvailability,omitempty"`
	Preflight          *PreflightConfig        `json:"preflight,omitempty"`
	Kernel             *KernelConfig           `json:"kernel,omitempty"`
	Addons             []AddonConfig           `json:"addons,omitempty"` // Slice of AddonConfig
	// HostsCount int `json:"hostsCount,omitempty"` // Example for printcolumn
}

// GlobalSpec contains settings applicable to the entire cluster or as defaults for hosts.
type GlobalSpec struct {
	User              string        `json:"user,omitempty"`
	Port              int           `json:"port,omitempty"`
	Password          string        `json:"password,omitempty"`
	PrivateKey        string        `json:"privateKey,omitempty"`
	PrivateKeyPath    string        `json:"privateKeyPath,omitempty"`
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty"`
	WorkDir           string        `json:"workDir,omitempty"`
	Verbose           bool          `json:"verbose,omitempty"`
	IgnoreErr         bool          `json:"ignoreErr,omitempty"`
	SkipPreflight     bool          `json:"skipPreflight,omitempty"`
}

// HostSpec defines the configuration for a single host.
type HostSpec struct {
	Name            string            `json:"name"`
	Address         string            `json:"address"`
	InternalAddress string            `json:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty"`
	User            string            `json:"user,omitempty"`
	Password        string            `json:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty"`
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty"`
	Roles           []string          `json:"roles,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty"`
	Type            string            `json:"type,omitempty"`
}

// TaintSpec defines a Kubernetes node taint.
type TaintSpec struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// Placeholder structs for component configs (initial version)
// Removed placeholder ContainerRuntimeConfig
// Removed placeholder ContainerdConfig
// Removed placeholder EtcdConfig
// Removed placeholder KubernetesConfig (should be in kubernetes_types.go)
// Removed placeholder HighAvailabilityConfig (should be in ha_types.go)
// Removed placeholder PreflightConfig (should be in preflight_types.go)
// Removed placeholder KernelConfig (should be in kernel_types.go)
type NetworkConfig struct { // Placeholder
   Plugin string `json:"plugin,omitempty"`
   Version string `json:"version,omitempty"`
}
// Removed placeholder AddonConfig

// SetDefaults_Cluster sets default values for the Cluster configuration.
func SetDefaults_Cluster(cfg *Cluster) {
	if cfg == nil {
		return
	}
	cfg.SetGroupVersionKind(SchemeGroupVersion.WithKind("Cluster"))
	if cfg.Spec.Global == nil {
		cfg.Spec.Global = &GlobalSpec{}
	}
	g := cfg.Spec.Global
	if g.Port == 0 { g.Port = 22 }
	if g.ConnectionTimeout == 0 { g.ConnectionTimeout = 30 * time.Second }
	if g.WorkDir == "" { g.WorkDir = "/tmp/kubexms_work" }

	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i]
		if host.Port == 0 && g != nil { host.Port = g.Port }
		if host.User == "" && g != nil { host.User = g.User }
		if host.PrivateKeyPath == "" && g != nil { host.PrivateKeyPath = g.PrivateKeyPath }
		if host.Type == "" { host.Type = "ssh" }
		if host.Labels == nil { host.Labels = make(map[string]string) }
		if host.Taints == nil { host.Taints = []TaintSpec{} }
		if host.Roles == nil { host.Roles = []string{} }
	}

	// Initialize component configs if nil (initial placeholder logic)
	// Integrate ContainerRuntime and Containerd defaulting
	if cfg.Spec.ContainerRuntime == nil {
	    cfg.Spec.ContainerRuntime = &ContainerRuntimeConfig{}
	}
	SetDefaults_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime)
	if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
	    if cfg.Spec.Containerd == nil {
	        cfg.Spec.Containerd = &ContainerdConfig{}
	    }
	    SetDefaults_ContainerdConfig(cfg.Spec.Containerd)
	}

	// Integrate EtcdConfig defaulting
	if cfg.Spec.Etcd == nil {
	    cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd)

	if cfg.Spec.Kubernetes == nil {
	    cfg.Spec.Kubernetes = &KubernetesConfig{}
	}
	SetDefaults_KubernetesConfig(cfg.Spec.Kubernetes, cfg.ObjectMeta.Name)
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.ClusterName == "" && cfg.ObjectMeta.Name != "" {
	   cfg.Spec.Kubernetes.ClusterName = cfg.ObjectMeta.Name
	}

	if cfg.Spec.Network == nil { cfg.Spec.Network = &NetworkConfig{} }
	// Call SetDefaults_NetworkConfig if/when defined

	if cfg.Spec.HighAvailability == nil {
	    cfg.Spec.HighAvailability = &HighAvailabilityConfig{}
	}
	SetDefaults_HighAvailabilityConfig(cfg.Spec.HighAvailability)

	if cfg.Spec.Preflight == nil {
	    cfg.Spec.Preflight = &PreflightConfig{}
	}
	SetDefaults_PreflightConfig(cfg.Spec.Preflight)

	if cfg.Spec.Kernel == nil {
	    cfg.Spec.Kernel = &KernelConfig{}
	}
	SetDefaults_KernelConfig(cfg.Spec.Kernel)

	if cfg.Spec.Addons == nil {
	    cfg.Spec.Addons = []AddonConfig{}
	}
	for i := range cfg.Spec.Addons { // Iterate by index to modify items in place
	    SetDefaults_AddonConfig(&cfg.Spec.Addons[i])
	}
}

// Validate_Cluster validates the Cluster configuration.
func Validate_Cluster(cfg *Cluster) error {
	verrs := &ValidationErrors{}
	if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
		verrs.Add("apiVersion: must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion)
	}
	if cfg.Kind != "Cluster" {
		verrs.Add("kind: must be Cluster, got %s", cfg.Kind)
	}
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name: cannot be empty")
	}
	if cfg.Spec.Global != nil {
		g := cfg.Spec.Global
		if g.Port !=0 && (g.Port <= 0 || g.Port > 65535) {
			verrs.Add("spec.global.port: %d is invalid, must be between 1 and 65535 or 0 for default", g.Port)
		}
	}
	if len(cfg.Spec.Hosts) == 0 {
		verrs.Add("spec.hosts: must contain at least one host")
	}
	hostNames := make(map[string]bool)
	for i, host := range cfg.Spec.Hosts {
		pathPrefix := fmt.Sprintf("spec.hosts[%d:%s]", i, host.Name)
		if strings.TrimSpace(host.Name) == "" {
			pathPrefix = fmt.Sprintf("spec.hosts[%d]", i)
			verrs.Add("%s.name: cannot be empty", pathPrefix)
		} else {
			if _, exists := hostNames[host.Name]; exists { verrs.Add("%s.name: '%s' is duplicated", pathPrefix, host.Name) }
			hostNames[host.Name] = true
		}
		if strings.TrimSpace(host.Address) == "" { verrs.Add("%s.address: cannot be empty", pathPrefix)
		} else {
			if net.ParseIP(host.Address) == nil {
				if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, host.Address); !matched {
					verrs.Add("%s.address: '%s' is not a valid IP address or hostname", pathPrefix, host.Address)
				}
			}
		}
		if host.Port <= 0 || host.Port > 65535 { verrs.Add("%s.port: %d is invalid, must be between 1 and 65535", pathPrefix, host.Port) }
		if strings.TrimSpace(host.User) == "" { verrs.Add("%s.user: cannot be empty (after defaults)", pathPrefix) }
		if strings.ToLower(host.Type) != "local" {
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add("%s: no SSH authentication method provided for non-local host", pathPrefix)
			}
		}
	}
	// Initial basic validation for Kubernetes (will be expanded when kubernetes_types.go is integrated)
	// This specific validation is now moved to Validate_KubernetesConfig
	// if cfg.Spec.Kubernetes != nil && strings.TrimSpace(cfg.Spec.Kubernetes.Version) == "" {
	//    verrs.Add("spec.kubernetes.version: cannot be empty")
	// }

	// Integrate ContainerRuntime and Containerd validation
	if cfg.Spec.ContainerRuntime != nil {
	    Validate_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime, verrs, "spec.containerRuntime")
	    if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
	        if cfg.Spec.Containerd == nil {
	            verrs.Add("spec.containerd: must be defined if containerRuntime.type is '%s'", ContainerRuntimeContainerd)
	        } else {
	            Validate_ContainerdConfig(cfg.Spec.Containerd, verrs, "spec.containerd")
	        }
	    }
	}

	// Integrate EtcdConfig validation
	if cfg.Spec.Etcd != nil {
	    Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	}

	// Integrate KubernetesConfig validation
	if cfg.Spec.Kubernetes == nil {
	    verrs.Add("spec.kubernetes: section is required")
	} else {
	    Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes")
	}

	// Integrate HighAvailability validation
	if cfg.Spec.HighAvailability != nil {
	    Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
	}

	// Integrate Preflight validation
	if cfg.Spec.Preflight != nil {
	    Validate_PreflightConfig(cfg.Spec.Preflight, verrs, "spec.preflight")
	}

	// Integrate Kernel validation
	if cfg.Spec.Kernel != nil {
	    Validate_KernelConfig(cfg.Spec.Kernel, verrs, "spec.kernel")
	}

	if cfg.Spec.Addons != nil { // Check if Addons slice itself is nil
	    for i := range cfg.Spec.Addons {
	        addonNameForPath := cfg.Spec.Addons[i].Name
	        if addonNameForPath == "" { // Handle case where addon name might be empty during validation
	            addonNameForPath = fmt.Sprintf("index_%d", i)
	        }
	        addonPathPrefix := fmt.Sprintf("spec.addons[%s]", addonNameForPath)
	        Validate_AddonConfig(&cfg.Spec.Addons[i], verrs, addonPathPrefix)
	    }
	}
	// ... and so on for Network etc. (Network is still a placeholder)

	if !verrs.IsEmpty() { return verrs }
	return nil
}

// ValidationErrors (simple version, can be moved to a common errors file)
type ValidationErrors struct { Errors []string }
func (ve *ValidationErrors) Add(format string, args ...interface{}) { ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...)) }
func (ve *ValidationErrors) Error() string { if len(ve.Errors) == 0 { return "no validation errors" }; return strings.Join(ve.Errors, "; ") }
func (ve *ValidationErrors) IsEmpty() bool { return len(ve.Errors) == 0 }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
