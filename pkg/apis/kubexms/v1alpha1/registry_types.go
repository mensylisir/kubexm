package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// RegistryConfig defines configurations related to container image registries.
type RegistryConfig struct {
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"`
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"`
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
		SetDefaults_RegistryAuth(&authEntry)
		cfg.Auths[k] = authEntry
	}
	if cfg.NamespaceRewrite == nil {
		cfg.NamespaceRewrite = &NamespaceRewriteConfig{}
	}
	if cfg.NamespaceRewrite.Rules == nil {
		cfg.NamespaceRewrite.Rules = []NamespaceRewriteRule{}
	}
	// If a local registry type is specified and DataRoot is not, set a default DataRoot.
	if cfg.Type != nil && *cfg.Type != "" && (cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "") {
		defaultDataRoot := "/var/lib/registry" // Default path for local registry data
		cfg.DataRoot = stringPtr(defaultDataRoot)
	}
}

// SetDefaults_RegistryAuth sets default values for RegistryAuth.
func SetDefaults_RegistryAuth(cfg *RegistryAuth) {
	if cfg == nil {
		return
	}
	if cfg.SkipTLSVerify == nil {
		cfg.SkipTLSVerify = boolPtr(false)
	}
	if cfg.PlainHTTP == nil {
		cfg.PlainHTTP = boolPtr(false)
	}
}

// Validate_RegistryConfig validates RegistryConfig.
func Validate_RegistryConfig(cfg *RegistryConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.PrivateRegistry != "" {
		if strings.TrimSpace(cfg.PrivateRegistry) == "" {
			verrs.Add(pathPrefix+".privateRegistry", "cannot be only whitespace if specified")
		} else if !util.IsValidDomainName(cfg.PrivateRegistry) && !util.IsValidIP(cfg.PrivateRegistry) && !util.ValidateHostPortString(cfg.PrivateRegistry) {
			verrs.Add(pathPrefix+".privateRegistry", fmt.Sprintf("invalid hostname/IP or host:port format '%s'", cfg.PrivateRegistry))
		}
	}

	if cfg.NamespaceOverride != "" && strings.TrimSpace(cfg.NamespaceOverride) == "" {
		verrs.Add(pathPrefix+".namespaceOverride", "cannot be only whitespace if specified")
	}

	for regAddr, auth := range cfg.Auths {
		authMapPath := pathPrefix + ".auths"
		authEntryPath := fmt.Sprintf("%s[\"%s\"]", authMapPath, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add(authMapPath, "registry address key cannot be empty")
		} else if !util.IsValidDomainName(regAddr) && !util.ValidateHostPortString(regAddr) {
			verrs.Add(authEntryPath, fmt.Sprintf("registry address key '%s' is not a valid hostname or host:port", regAddr))
		}
		Validate_RegistryAuth(&auth, verrs, authEntryPath)
	}

	if cfg.Type != nil && strings.TrimSpace(*cfg.Type) == "" {
		verrs.Add(pathPrefix+".type", "cannot be empty if specified")
	}
	// DataRoot validation: if Type is set, DataRoot must now be set (either by user or by new default).
	// If DataRoot is set (e.g. by user explicitly), Type must also be set.
	if cfg.Type != nil && *cfg.Type != "" {
		if cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "" {
			// This case should ideally not happen if defaults are applied correctly.
			// However, validation should still catch it if defaults were bypassed or user provided empty string.
			verrs.Add(pathPrefix+".registryDataDir (dataRoot)", "must be specified if registry type is set for local deployment and not defaulted")
		}
	}
	if (cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) != "") && (cfg.Type == nil || strings.TrimSpace(*cfg.Type) == "") {
		verrs.Add(pathPrefix+".type", "must be specified if registryDataDir (dataRoot) is set for local deployment")
	}


	if cfg.NamespaceRewrite != nil {
		if cfg.NamespaceRewrite.Enabled {
			if len(cfg.NamespaceRewrite.Rules) == 0 {
				verrs.Add(pathPrefix+".namespaceRewrite.rules", "must contain at least one rule if rewrite is enabled")
			}
			for i, rule := range cfg.NamespaceRewrite.Rules {
				rulePathPrefix := fmt.Sprintf("%s.namespaceRewrite.rules[%d]", pathPrefix, i)
				if strings.TrimSpace(rule.OldNamespace) == "" {
					verrs.Add(rulePathPrefix+".oldNamespace", "cannot be empty")
				}
				if strings.TrimSpace(rule.NewNamespace) == "" {
					verrs.Add(rulePathPrefix+".newNamespace", "cannot be empty")
				}
				if rule.Registry != "" {
					if strings.TrimSpace(rule.Registry) == "" {
						verrs.Add(rulePathPrefix+".registry", "cannot be only whitespace if specified")
					} else if !util.IsValidDomainName(rule.Registry) && !util.ValidateHostPortString(rule.Registry) {
						verrs.Add(rulePathPrefix+".registry", fmt.Sprintf("invalid hostname or host:port format '%s'", rule.Registry))
					}
				}
			}
		}
	}
}

// Validate_RegistryAuth validates RegistryAuth.
func Validate_RegistryAuth(cfg *RegistryAuth, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	hasUserPass := cfg.Username != "" && cfg.Password != ""
	hasAuthStr := cfg.Auth != ""

	if !hasUserPass && !hasAuthStr {
		verrs.Add(pathPrefix, "either username/password or auth string must be provided")
	}
	if hasAuthStr {
		decoded, err := base64.StdEncoding.DecodeString(cfg.Auth)
		if err != nil {
			verrs.Add(pathPrefix+".auth", fmt.Sprintf("failed to decode base64 auth string: %v", err))
		} else if !strings.Contains(string(decoded), ":") {
			verrs.Add(pathPrefix+".auth", "decoded auth string must be in 'username:password' format")
		}
	}
	if cfg.CertsPath != "" && strings.TrimSpace(cfg.CertsPath) == "" {
		verrs.Add(pathPrefix+".certsPath", "cannot be only whitespace if specified")
	}
}
