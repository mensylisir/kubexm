package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"
	"net/url" // For validating registry URLs
)

// RegistryConfig defines configurations related to container image registries.
type RegistryConfig struct {
	RegistryMirrors   []string                `json:"registryMirrors,omitempty" yaml:"registryMirrors,omitempty"`
	InsecureRegistries []string                `json:"insecureRegistries,omitempty" yaml:"insecureRegistries,omitempty"`
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"` // For local registry deployment
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"dataRoot,omitempty"` // For local registry deployment data
	NamespaceRewrite  *NamespaceRewriteConfig `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"` // New field
}

// RegistryAuth defines authentication credentials for a specific registry.
type RegistryAuth struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth     string `json:"auth,omitempty" yaml:"auth,omitempty"` // Base64 encoded "username:password"
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
	if cfg.RegistryMirrors == nil {
		cfg.RegistryMirrors = []string{}
	}
	if cfg.InsecureRegistries == nil {
		cfg.InsecureRegistries = []string{}
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[string]RegistryAuth)
	}
	if cfg.PrivateRegistry == "" {
		cfg.PrivateRegistry = "dockerhub.kubexm.local"
	}
	if cfg.Type != nil && *cfg.Type != "" { // If a local registry type is specified
		if cfg.DataRoot == nil || *cfg.DataRoot == "" {
			defaultDataRoot := "/mnt/registry"
			cfg.DataRoot = &defaultDataRoot
		}
	}
	// No default for Type itself.
	if cfg.NamespaceRewrite == nil {
		cfg.NamespaceRewrite = &NamespaceRewriteConfig{} // Initialize if nil
	}
	if cfg.NamespaceRewrite.Rules == nil {
		cfg.NamespaceRewrite.Rules = []NamespaceRewriteRule{} // Initialize Rules slice
	}
	// Default NamespaceRewrite.Enabled to false? Or assume if rules are present, it's enabled?
	// For now, no default for NamespaceRewrite.Enabled.
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
