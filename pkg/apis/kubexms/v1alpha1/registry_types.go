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
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"` // For local registry deployment (e.g., "registry", "harbor")
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"` // For local registry deployment data, matches "registryDataDir" from YAML notes
	NamespaceRewrite  *NamespaceRewriteConfig `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"`
}

// RegistryAuth defines authentication credentials for a specific registry.
type RegistryAuth struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty" yaml:"auth,omitempty"` // Base64 encoded "username:password"
	SkipTLSVerify *bool  `json:"skipTLSVerify,omitempty" yaml:"skipTLSVerify,omitempty"`
	PlainHTTP     *bool  `json:"plainHTTP,omitempty" yaml:"plainHTTP,omitempty"`
	CertsPath     string `json:"certsPath,omitempty" yaml:"certsPath,omitempty"`
}

// NamespaceRewriteConfig defines rules for rewriting image namespaces.
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
	if cfg.Type != nil && *cfg.Type != "" { // If a local registry type is specified
		if cfg.DataRoot == nil || *cfg.DataRoot == "" {
			defaultDataRoot := "/mnt/registry" // Default from 21-其他说明.md
			cfg.DataRoot = &defaultDataRoot
		}
	}
	// No default for Type itself.
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
		b := false // Default to verifying TLS
		cfg.SkipTLSVerify = &b
	}
	if cfg.PlainHTTP == nil {
		b := false // Default to not using plain HTTP
		cfg.PlainHTTP = &b
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
		// Could validate if it's a valid hostname/domain.
		// For now, ensure it's not just whitespace if set.
		if strings.TrimSpace(cfg.PrivateRegistry) == "" && cfg.PrivateRegistry != "" {
			verrs.Add("%s.privateRegistry: cannot be only whitespace if specified", pathPrefix)
		}
	}

	if cfg.NamespaceOverride != "" && strings.TrimSpace(cfg.NamespaceOverride) == "" {
		verrs.Add("%s.namespaceOverride: cannot be only whitespace if specified", pathPrefix)
	}


	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add("%s.auths: registry address key cannot be empty", pathPrefix)
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
				// Registry field in rule is optional.
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
