package config

import (
	"fmt"
	"net"
	"reflect" // For checking if pointer fields are nil without causing panic
	"regexp"
	"strings"
	// "time" // Not directly needed for validation logic itself, but for some types
)

// ValidationErrors holds a list of validation errors found.
type ValidationErrors struct {
	Errors []string
}

// Add appends a new error message to the list.
func (ve *ValidationErrors) Add(format string, args ...interface{}) {
	ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...))
}

// Error returns a string representation of all validation errors.
func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors" // Should not be called if IsEmpty is true
	}
	if len(ve.Errors) == 1 {
		return ve.Errors[0]
	}
	// Use a more standard multi-error format
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation errors occurred:", len(ve.Errors)))
	for _, errStr := range ve.Errors {
		sb.WriteString(fmt.Sprintf("\n\t* %s", errStr))
	}
	return sb.String()
}

// IsEmpty checks if any validation errors were recorded.
func (ve *ValidationErrors) IsEmpty() bool {
	return len(ve.Errors) == 0
}


// Validate performs comprehensive validation checks on the loaded and defaulted Cluster configuration.
// It returns an aggregated error of type ValidationErrors if issues are found.
func Validate(cfg *Cluster) error {
	if cfg == nil {
		return fmt.Errorf("cluster configuration is nil, cannot validate")
	}

	verrs := &ValidationErrors{}

	// Top-level checks
	if cfg.APIVersion == "" { // Assuming SetDefaults populated it if it was initially empty
		verrs.Add("apiVersion: cannot be empty after defaults (expected default: %s)", DefaultAPIVersion)
	} else if cfg.APIVersion != DefaultAPIVersion {
		verrs.Add("apiVersion: must be %s, got %s", DefaultAPIVersion, cfg.APIVersion)
	}

	if cfg.Kind == "" { // Assuming SetDefaults populated it
		verrs.Add("kind: cannot be empty after defaults (expected default: %s)", ClusterKind)
	} else if cfg.Kind != ClusterKind {
		verrs.Add("kind: must be %s, got %s", ClusterKind, cfg.Kind)
	}

	if strings.TrimSpace(cfg.Metadata.Name) == "" {
		verrs.Add("metadata.name: cannot be empty")
	}

	// GlobalSpec validation
	// Most global fields are optional or have defaults. Some critical ones might be checked.
	// Example: if cfg.Spec.Global.User == "" and no host has a user, it might be an error.
	// This is complex due to per-host overrides, so handled in host loop.

	// Hosts validation
	if len(cfg.Spec.Hosts) == 0 {
		verrs.Add("spec.hosts: must contain at least one host")
	}
	hostNames := make(map[string]bool)
	for i, host := range cfg.Spec.Hosts {
		pathPrefix := fmt.Sprintf("spec.hosts[%d:%s]", i, host.Name) // Use index and name if available
		if host.Name == "" { // If name itself is empty, pathPrefix won't have it.
			pathPrefix = fmt.Sprintf("spec.hosts[%d]", i)
		}


		if strings.TrimSpace(host.Name) == "" {
			verrs.Add("%s.name: cannot be empty", pathPrefix)
		} else {
			if _, exists := hostNames[host.Name]; exists {
				verrs.Add("%s.name: '%s' is duplicated", pathPrefix, host.Name)
			}
			hostNames[host.Name] = true
		}
		if strings.TrimSpace(host.Address) == "" {
			verrs.Add("%s.address: cannot be empty", pathPrefix)
		} else {
			if net.ParseIP(host.Address) == nil {
				// Not an IP, try as hostname. Regex for basic validation.
				// This regex allows leading/trailing hyphens if not careful with full spec.
				// A simpler check might be to ensure it's not empty and doesn't contain invalid chars.
				// For a more robust hostname validation, a library might be better.
				// RFC 1123 allows LDH (letters, digits, hyphen).
				if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, host.Address); !matched {
				    verrs.Add("%s.address: '%s' is not a valid IP address or hostname", pathPrefix, host.Address)
				}
			}
		}
		if host.Port <= 0 || host.Port > 65535 { // Port is already defaulted from Global or to 22 by SetDefaults
			verrs.Add("%s.port: %d is invalid, must be between 1 and 65535 (after defaults)", pathPrefix, host.Port)
		}
		if strings.TrimSpace(host.User) == "" { // User is defaulted from Global by SetDefaults
			verrs.Add("%s.user: cannot be empty (after defaults from global.user)", pathPrefix)
		}

		// Auth validation (after defaults):
		// At least one auth method must be configured for SSH hosts. "local" type doesn't need this.
		if strings.ToLower(host.Type) != "local" {
			hasAuth := false
			if host.Password != "" { hasAuth = true }
			if host.PrivateKey != "" { hasAuth = true } // Base64 content
			if host.PrivateKeyPath != "" { hasAuth = true } // Path to key
			if !hasAuth {
				verrs.Add("%s: no SSH authentication method provided (password, privateKey content, or privateKeyPath)", pathPrefix)
			}
		}

		// Validate roles if there's a predefined set of valid roles
		// for _, role := range host.Roles { if !isValidRole(role) { verrs.Add(...) } }
	}

	// ContainerRuntime validation
	if cfg.Spec.ContainerRuntime != nil { // It's a pointer, could be nil if completely omitted
		validRuntimeTypes := []string{"containerd", "docker"}
		isRuntimeValid := false
		for _, vrt := range validRuntimeTypes { if cfg.Spec.ContainerRuntime.Type == vrt { isRuntimeValid = true; break } }
		// If Type is empty, SetDefaults made it "containerd", which is valid.
		if cfg.Spec.ContainerRuntime.Type != "" && !isRuntimeValid  {
			verrs.Add("spec.containerRuntime.type: invalid type '%s', must be one of %v (or empty for default 'containerd')", cfg.Spec.ContainerRuntime.Type, validRuntimeTypes)
		}
		if cfg.Spec.ContainerRuntime.Type == "containerd" {
			if cfg.Spec.Containerd == nil {
				// SetDefaults initializes this if ContainerRuntime.Type is "containerd".
				// So, this should not be nil if type is containerd after defaults.
				// verrs.Add("spec.containerd: must be defined if containerRuntime.type is 'containerd'")
			} else {
				// Validate ContainerdSpec fields if any, e.g., version format
			}
		}
	} else {
		// This implies ContainerRuntime was not specified at all. SetDefaults initializes it.
		// verrs.Add("spec.containerRuntime: section is required")
	}


	// Etcd validation
	if cfg.Spec.Etcd != nil { // EtcdSpec is a pointer
		validEtcdTypes := []string{"stacked", "external"}
		isEtcdTypeValid := false
		for _, vet := range validEtcdTypes { if cfg.Spec.Etcd.Type == vet { isEtcdTypeValid = true; break } }
		// If Type is empty, SetDefaults made it "stacked", which is valid.
		if cfg.Spec.Etcd.Type != "" && !isEtcdTypeValid {
			verrs.Add("spec.etcd.type: invalid type '%s', must be one of %v (or empty for default 'stacked')", cfg.Spec.Etcd.Type, validEtcdTypes)
		}
		if cfg.Spec.Etcd.Type == "external" {
			// Example: if cfg.Spec.Etcd.External == nil || len(cfg.Spec.Etcd.External.Endpoints) == 0 {
			// 	verrs.Add("spec.etcd.external.endpoints: must be defined if etcd.type is 'external'")
			// }
		}
		// If stacked, ensure etcd roles are on master nodes or dedicated etcd nodes.
		// This might be too complex for basic validation here, more of a cross-field/semantic check.
	}

	// Kubernetes validation
	if cfg.Spec.Kubernetes != nil { // KubernetesSpec is a pointer
		if strings.TrimSpace(cfg.Spec.Kubernetes.Version) == "" {
			verrs.Add("spec.kubernetes.version: cannot be empty")
		}
		if cfg.Spec.Kubernetes.PodSubnet != "" {
			if _, _, err := net.ParseCIDR(cfg.Spec.Kubernetes.PodSubnet); err != nil {
				verrs.Add("spec.kubernetes.podSubnet: invalid CIDR '%s': %v", cfg.Spec.Kubernetes.PodSubnet, err)
			}
		} else {
			// verrs.Add("spec.kubernetes.podSubnet: cannot be empty") // If required
		}
		if cfg.Spec.Kubernetes.ServiceSubnet != "" {
			if _, _, err := net.ParseCIDR(cfg.Spec.Kubernetes.ServiceSubnet); err != nil {
				verrs.Add("spec.kubernetes.serviceSubnet: invalid CIDR '%s': %v", cfg.Spec.Kubernetes.ServiceSubnet, err)
			}
		} else {
			// verrs.Add("spec.kubernetes.serviceSubnet: cannot be empty") // If required
		}
	} else {
		verrs.Add("spec.kubernetes: section is required")
	}


	// Network validation
	if cfg.Spec.Network != nil { // NetworkSpec is a pointer
		if cfg.Spec.Network.Plugin != "" {
			validCNIs := []string{"calico", "flannel", "cilium", "weave"} // Example
			isCNIValid := false
			for _, vcn := range validCNIs { if cfg.Spec.Network.Plugin == vcn { isCNIValid = true; break } }
			if !isCNIValid {
				verrs.Add("spec.network.plugin: invalid plugin '%s', supported examples: %v", cfg.Spec.Network.Plugin, validCNIs)
			}
		} else {
			// verrs.Add("spec.network.plugin: cannot be empty") // If CNI plugin is mandatory
		}
	} else {
		// verrs.Add("spec.network: section is required") // If network config is mandatory
	}


	// HighAvailability validation
	if cfg.Spec.HighAvailability != nil { // HighAvailabilitySpec is a pointer
		if cfg.Spec.HighAvailability.Type != "" {
			// Example: if cfg.Spec.HighAvailability.Type == "keepalived" && cfg.Spec.HighAvailability.VIP == "" {
			// 	verrs.Add("spec.highAvailability.vip: must be set if type is 'keepalived'")
			// }
		}
	}

	if !verrs.IsEmpty() {
		return verrs
	}
	return nil
}

// isNil is a helper, currently not used as direct nil checks on pointers are preferred.
func isNil(i interface{}) bool {
    if i == nil { return true }
    val := reflect.ValueOf(i)
    // Check if it's a pointer, interface, map, slice, or chan and if it's nil.
    switch val.Kind() {
    case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan:
        return val.IsNil()
    }
    return false
}
