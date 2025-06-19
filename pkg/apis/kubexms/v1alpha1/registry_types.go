package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"
	"net/url" // For validating registry URLs
)

// RegistryConfig defines configurations related to container image registries.
type RegistryConfig struct {
	// RegistryMirrors is a list of mirrors for container registries.
	// Each string should be a valid registry URL.
	// Example: ["https://mymirror.example.com"] (applies to docker.io by default or needs more structure)
	// KubeKey's RegistryConfig had this as []string, implying global mirrors.
	// A richer structure might be map[string][]string like ContainerdConfig.RegistryMirrors.
	// For now, sticking to KubeKey's simple []string for global mirrors.
	RegistryMirrors []string `json:"registryMirrors,omitempty"`

	// InsecureRegistries is a list of registries that can be accessed over HTTP.
	// Each string should be a host or host:port. Example: ["my.insecure.registry:5000"]
	InsecureRegistries []string `json:"insecureRegistries,omitempty"`

	// PrivateRegistry specifies a default private registry to prefix images that don't have a registry specified.
	// Example: "mycompany.registry.com"
	// If set, an image like "myimage:latest" would be pulled as "mycompany.registry.com/myimage:latest".
	// If it includes a namespace like "mycompany.registry.com/mynamespace", then "myimage" becomes "mycompany.registry.com/mynamespace/myimage".
	PrivateRegistry string `json:"privateRegistry,omitempty"`

	// NamespaceOverride can be used to force all images into a specific namespace within a private registry.
	// This is often used in air-gapped environments. Example: "kube-images"
	// If PrivateRegistry is "myreg.com" and NamespaceOverride is "airgap", "nginx" becomes "myreg.com/airgap/nginx".
	NamespaceOverride string `json:"namespaceOverride,omitempty"`

	// Auths provides authentication credentials for private registries.
	// The key is the registry server address (e.g., "docker.io", "mycompany.registry.com").
	Auths map[string]RegistryAuth `json:"auths,omitempty"`

	// Type of the private registry, if one is being deployed or managed by this tool.
	// e.g., "harbor", "docker-registry". Not for client configuration.
	// This field might be better in a separate "LocalRegistryDeploymentConfig" struct.
	// For now, adding based on KubeKey's presence.
	Type *string `json:"type,omitempty"`

	// DataRoot for a locally deployed registry (if Type indicates one is managed).
	DataRoot *string `json:"dataRoot,omitempty"`
}

// RegistryAuth defines authentication credentials for a specific registry.
type RegistryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// Auth is a base64 encoded string of "username:password".
	// If Auth is provided, Username and Password might be ignored by some tools.
	Auth     string `json:"auth,omitempty"`
	// Email      string `json:"email,omitempty"` // Often included in Docker config.json
	// ServerAddress string `json:"serveraddress,omitempty"` // Key of the map is already server address
}

// --- Defaulting Functions ---

// SetDefaults_RegistryConfig sets default values for RegistryConfig.
func SetDefaults_RegistryConfig(cfg *RegistryConfig) {
	if cfg == nil {
		return
	}
	if cfg.RegistryMirrors == nil {
		cfg.RegistryMirrors = []string{}
	}
	if cfg.InsecureRegistries == nil {
		cfg.InsecureRegistries = []string{}
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[string]RegistryAuth)
	}
	// No defaults for Type or DataRoot; they are for specific deployment scenarios.
}

// --- Validation Functions ---

// Validate_RegistryConfig validates RegistryConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_RegistryConfig(cfg *RegistryConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	for i, mirror := range cfg.RegistryMirrors {
		if strings.TrimSpace(mirror) == "" {
			verrs.Add("%s.registryMirrors[%d]: mirror URL cannot be empty", pathPrefix, i)
		} else {
			// Basic URL validation
			_, err := url.ParseRequestURI(mirror)
			if err != nil {
				verrs.Add("%s.registryMirrors[%d]: invalid URL format '%s': %v", pathPrefix, i, mirror, err)
			}
		}
	}

	for i, insecureReg := range cfg.InsecureRegistries {
		if strings.TrimSpace(insecureReg) == "" {
			verrs.Add("%s.insecureRegistries[%d]: registry host cannot be empty", pathPrefix, i)
		}
		// Could add host:port validation if desired.
	}

	if cfg.PrivateRegistry != "" {
		// Basic validation, could check for valid hostname characters or URL-like structure.
		// For now, just ensure it's not obviously malformed if we had stricter rules.
	}

	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr) // Corrected formatting
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add("%s.auths: registry address key cannot be empty", pathPrefix)
		}
		hasUserPass := auth.Username != "" && auth.Password != ""
		hasAuthStr := auth.Auth != ""

		if !hasUserPass && !hasAuthStr {
			verrs.Add("%s: either username/password or auth string must be provided", authPathPrefix)
		}
		if hasAuthStr {
			_, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err != nil {
				verrs.Add("%s.auth: failed to decode base64 auth string: %v", authPathPrefix, err)
			}
		}
	}
	if cfg.Type != nil && strings.TrimSpace(*cfg.Type) == "" {
		verrs.Add("%s.type: cannot be empty if specified", pathPrefix)
	}
	if cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) == "" {
		verrs.Add("%s.dataRoot: cannot be empty if specified", pathPrefix)
	}
	if (cfg.Type != nil && *cfg.Type != "") && (cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "") {
		verrs.Add("%s.dataRoot: must be specified if registry type is set (for local deployment)", pathPrefix)
	}
	if (cfg.DataRoot != nil && *cfg.DataRoot != "") && (cfg.Type == nil || strings.TrimSpace(*cfg.Type) == "") {
		verrs.Add("%s.type: must be specified if dataRoot is set (for local deployment)", pathPrefix)
	}
}
