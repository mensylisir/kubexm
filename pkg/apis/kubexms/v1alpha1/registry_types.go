package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"
	// "net/url" // For validating registry URLs - Removed as not used
)

// RegistryConfig defines configurations related to container image registries.
type RegistryConfig struct {
	// RegistryMirrors and InsecureRegistries are removed as they belong to ContainerRuntime config.
	// PrivateRegistry is the FQDN of the private registry.
	// Corresponds to `privateRegistry` in YAML.
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	// NamespaceOverride to prepend to all images if the private registry doesn't support nested namespaces.
	// Corresponds to `namespaceOverride` in YAML.
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	// Auths provides authentication details for registries.
	// The key is the registry address. Corresponds to `auths` in YAML.
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	// Type specifies the type of local registry to deploy (e.g., "registry", "harbor").
	// Corresponds to `type` in YAML (under `registry` block).
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"`
	// DataRoot for the local registry if deployed by KubeXMS.
	// Corresponds to `registryDataDir` in YAML.
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"`
	NamespaceRewrite  *NamespaceRewriteConfig `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"` // Not in provided YAML, but a common feature
}

// RegistryAuth defines authentication credentials for a specific registry.
// Corresponds to an entry in `registry.auths` in YAML.
type RegistryAuth struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty" yaml:"auth,omitempty"` // Base64 encoded "username:password"
	// SkipTLSVerify allows contacting registries over HTTPS with failed TLS verification.
	// Corresponds to `skipTLSVerify` in YAML.
	SkipTLSVerify *bool  `json:"skipTLSVerify,omitempty" yaml:"skipTLSVerify,omitempty"`
	// PlainHTTP allows contacting registries over HTTP.
	// Corresponds to `plainHTTP` in YAML.
	PlainHTTP     *bool  `json:"plainHTTP,omitempty" yaml:"plainHTTP,omitempty"`
	// CertsPath to use certificates at path (*.crt, *.cert, *.key) to connect to the registry.
	// Corresponds to `certsPath` in YAML.
	CertsPath     string `json:"certsPath,omitempty" yaml:"certsPath,omitempty"`
}

// NamespaceRewriteConfig defines rules for rewriting image namespaces. (Advanced feature, not in provided YAML)
type NamespaceRewriteConfig struct {
	Enabled bool                   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Rules   []NamespaceRewriteRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// NamespaceRewriteRule defines a single namespace rewrite rule.
type NamespaceRewriteRule struct {
	Registry     string `json:"registry,omitempty" yaml:"registry,omitempty"` // Target registry for this rule, e.g., "docker.io"
	OldNamespace string `json:"oldNamespace" yaml:"oldNamespace"`             // Original namespace, e.g., "library"
	NewNamespace string `json:"newNamespace" yaml:"newNamespace"`             // Namespace to rewrite to, e.g., "mycorp"
}

// --- Defaulting Functions ---

// SetDefaults_RegistryConfig sets default values for RegistryConfig.
func SetDefaults_RegistryConfig(cfg *RegistryConfig) {
	if cfg == nil {
		return
	}
	// RegistryMirrors and InsecureRegistries removed from this struct.

	if cfg.Auths == nil {
		cfg.Auths = make(map[string]RegistryAuth)
	}
	for k, authEntry := range cfg.Auths { // Iterate to set defaults for each auth entry
		SetDefaults_RegistryAuth(&authEntry)
		cfg.Auths[k] = authEntry // Assign back because authEntry is a copy
	}

	if cfg.PrivateRegistry == "" {
		// Consider if a default private registry FQDN makes sense or should be left empty.
		// cfg.PrivateRegistry = "dockerhub.kubexm.local" // Example from YAML
	}
	// Defaulting of DataRoot based on Type is removed, as validation expects user to provide it if Type is set.
	// if cfg.Type != nil && *cfg.Type != "" { // If a local registry type is specified
	//	if cfg.DataRoot == nil || *cfg.DataRoot == "" {
	//		cfg.DataRoot = stringPtr("/mnt/registry") // Default from 21-其他说明.md
	//	}
	// }
	// No default for Type itself.
	// Removed defaulting of DataRoot when Type is set, as validation expects user to provide it.
	// if cfg.Type != nil && *cfg.Type != "" { // If a local registry type is specified
	// 	if cfg.DataRoot == nil || *cfg.DataRoot == "" {
	// 		cfg.DataRoot = stringPtr("/mnt/registry")
	// 	}
	// }
	if cfg.NamespaceRewrite == nil {
		cfg.NamespaceRewrite = &NamespaceRewriteConfig{}
	}
	if cfg.NamespaceRewrite.Rules == nil {
		cfg.NamespaceRewrite.Rules = []NamespaceRewriteRule{}
	}
	// NamespaceRewrite.Enabled defaults to false (zero value for bool).
}

// SetDefaults_RegistryAuth sets default values for RegistryAuth.
func SetDefaults_RegistryAuth(cfg *RegistryAuth) {
	if cfg == nil {
		return
	}
	if cfg.SkipTLSVerify == nil {
		cfg.SkipTLSVerify = boolPtr(false) // Default to verifying TLS
	}
	if cfg.PlainHTTP == nil {
		cfg.PlainHTTP = boolPtr(false) // Default to not using plain HTTP
	}
}

// --- Validation Functions ---

// Validate_RegistryConfig validates RegistryConfig.
func Validate_RegistryConfig(cfg *RegistryConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	// Validation for RegistryMirrors and InsecureRegistries removed.

	if cfg.PrivateRegistry != "" {
		if strings.TrimSpace(cfg.PrivateRegistry) == "" {
			verrs.Add("%s.privateRegistry: cannot be only whitespace if specified", pathPrefix)
		} else if !isValidDomainName(cfg.PrivateRegistry) && !isValidHostOrIP(cfg.PrivateRegistry) { // Allow IP as well for private registry
			verrs.Add("%s.privateRegistry: invalid hostname/IP format '%s'", pathPrefix, cfg.PrivateRegistry)
		}
	}

	if cfg.NamespaceOverride != "" && strings.TrimSpace(cfg.NamespaceOverride) == "" {
		verrs.Add("%s.namespaceOverride: cannot be only whitespace if specified", pathPrefix)
	}

	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add("%s.auths: registry address key cannot be empty", pathPrefix)
		} else if !isValidDomainName(regAddr) && !isValidRegistryHostPort(regAddr) { // Allow host:port for registry auth keys
			verrs.Add("%s.auths: registry address key '%s' is not a valid hostname or host:port", pathPrefix, regAddr)
		}
		Validate_RegistryAuth(&auth, verrs, authPathPrefix)
	}

	if cfg.Type != nil && strings.TrimSpace(*cfg.Type) == "" {
		verrs.Add("%s.type: cannot be empty if specified", pathPrefix)
	}
	if cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) == "" {
		verrs.Add("%s.registryDataDir (dataRoot): cannot be empty if specified", pathPrefix)
	}
	if (cfg.Type != nil && *cfg.Type != "") && (cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "") {
		verrs.Add("%s.registryDataDir (dataRoot): must be specified if registry type is set for local deployment", pathPrefix)
	}
	if (cfg.DataRoot != nil && *cfg.DataRoot != "") && (cfg.Type == nil || strings.TrimSpace(*cfg.Type) == "") {
		verrs.Add("%s.type: must be specified if registryDataDir (dataRoot) is set for local deployment", pathPrefix)
	}

	if cfg.NamespaceRewrite != nil {
		if cfg.NamespaceRewrite.Enabled { // Only validate rules if rewrite is enabled
			if len(cfg.NamespaceRewrite.Rules) == 0 {
				verrs.Add("%s.namespaceRewrite.rules: must contain at least one rule if rewrite is enabled", pathPrefix)
			}
			for i, rule := range cfg.NamespaceRewrite.Rules {
				rulePathPrefix := fmt.Sprintf("%s.namespaceRewrite.rules[%d]", pathPrefix, i)
				if strings.TrimSpace(rule.OldNamespace) == "" {
					verrs.Add("%s.oldNamespace: cannot be empty", rulePathPrefix)
				}
				if strings.TrimSpace(rule.NewNamespace) == "" {
					verrs.Add("%s.newNamespace: cannot be empty", rulePathPrefix)
				}
				if rule.Registry != "" {
					if strings.TrimSpace(rule.Registry) == "" {
						verrs.Add("%s.registry: cannot be only whitespace if specified", rulePathPrefix)
					} else if !isValidDomainName(rule.Registry) && !isValidRegistryHostPort(rule.Registry) {
						verrs.Add("%s.registry: invalid hostname or host:port format '%s'", rulePathPrefix, rule.Registry)
					}
				}
			}
		}
	}
}

// Validate_RegistryAuth validates RegistryAuth.
func Validate_RegistryAuth(cfg *RegistryAuth, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	hasUserPass := cfg.Username != "" && cfg.Password != ""
	hasAuthStr := cfg.Auth != ""

	if !hasUserPass && !hasAuthStr {
		verrs.Add("%s: either username/password or auth string must be provided", pathPrefix)
	}
	if hasAuthStr {
		decoded, err := base64.StdEncoding.DecodeString(cfg.Auth)
		if err != nil {
			verrs.Add("%s.auth: failed to decode base64 auth string: %v", pathPrefix, err)
		} else if !strings.Contains(string(decoded), ":") {
			verrs.Add("%s.auth: decoded auth string must be in 'username:password' format", pathPrefix)
		}
	}
	if cfg.CertsPath != "" && strings.TrimSpace(cfg.CertsPath) == "" {
		verrs.Add("%s.certsPath: cannot be only whitespace if specified", pathPrefix)
	}
	// SkipTLSVerify and PlainHTTP are booleans, type checking is sufficient.
}
