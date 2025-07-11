package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"
	// Assuming ValidationErrors is in cluster_types.go or a shared util in this package
)

// RegistryConfig defines configurations related to container image registries.
type RegistryConfig struct {
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"`
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"` // Matches YAML registryDataDir
	NamespaceRewrite  *NamespaceRewriteConfig `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"`
}

// RegistryAuth defines authentication credentials for a specific registry.
type RegistryAuth struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty" yaml:"auth,omitempty"`
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
	Registry     string `json:"registry,omitempty" yaml:"registry,omitempty"`
	OldNamespace string `json:"oldNamespace" yaml:"oldNamespace"`
	NewNamespace string `json:"newNamespace" yaml:"newNamespace"`
}

// SetDefaults_RegistryConfig sets default values for RegistryConfig.
func SetDefaults_RegistryConfig(cfg *RegistryConfig) {
	if cfg == nil {
		return
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[string]RegistryAuth)
	}
	for k, authEntry := range cfg.Auths {
		// Create a new variable for the copy to take its address
		entryCopy := authEntry
		SetDefaults_RegistryAuth(&entryCopy)
		cfg.Auths[k] = entryCopy
	}
	if cfg.Type != nil && *cfg.Type != "" {
		if cfg.DataRoot == nil || *cfg.DataRoot == "" {
			defaultDataRoot := "/mnt/registry"
			cfg.DataRoot = &defaultDataRoot
		}
	}
	if cfg.NamespaceRewrite == nil {
		cfg.NamespaceRewrite = &NamespaceRewriteConfig{}
	}
	if cfg.NamespaceRewrite.Rules == nil {
		cfg.NamespaceRewrite.Rules = []NamespaceRewriteRule{}
	}
}

// SetDefaults_RegistryAuth sets default values for RegistryAuth.
func SetDefaults_RegistryAuth(cfg *RegistryAuth) {
	if cfg == nil {
		return
	}
	if cfg.SkipTLSVerify == nil {
		b := false
		cfg.SkipTLSVerify = &b
	}
	if cfg.PlainHTTP == nil {
		b := false
		cfg.PlainHTTP = &b
	}
}

// Validate_RegistryConfig validates RegistryConfig.
func Validate_RegistryConfig(cfg *RegistryConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.PrivateRegistry != "" && strings.TrimSpace(cfg.PrivateRegistry) == "" {
		verrs.Add("%s.privateRegistry: cannot be only whitespace if specified", pathPrefix)
	}
	if cfg.NamespaceOverride != "" && strings.TrimSpace(cfg.NamespaceOverride) == "" {
		verrs.Add("%s.namespaceOverride: cannot be only whitespace if specified", pathPrefix)
	}
	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add("%s.auths: registry address key cannot be empty", pathPrefix)
		}
		Validate_RegistryAuth(&auth, verrs, authPathPrefix) // Pass address of auth
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
		if cfg.NamespaceRewrite.Enabled {
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
}
// Assuming ValidationErrors is defined in cluster_types.go or a shared util.
// NOTE: DeepCopy methods should be generated by controller-gen.
// Updated SetDefaults_RegistryConfig to correctly pass address of authEntry copy.
// Updated Validate_RegistryConfig to correctly pass address of auth.
// Added import "encoding/base64", "fmt", "strings".
