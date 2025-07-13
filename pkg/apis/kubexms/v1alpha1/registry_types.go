package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"strings"
	// Assuming ValidationErrors is in cluster_types.go or a shared util in this package
)

// RegistryConfig defines configurations related to container image registries.
type Registry struct {
	PrivateRegistry   string                  `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string                  `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	Auths             map[string]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Type              *string                 `json:"type,omitempty" yaml:"type,omitempty"`
	DataRoot          *string                 `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"` // Matches YAML registryDataDir
	NamespaceRewrite  *NamespaceRewrite       `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"`
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

type NamespaceRewritePolicy string
type NamespaceRewrite struct {
	Policy NamespaceRewritePolicy `yaml:"policy" json:"policy"`
	Src    []string               `yaml:"src" json:"src"`
	Dest   string                 `yaml:"dest" json:"dest"`
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
		if cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "" {
			defaultDataRoot := "/var/lib/registry"
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
		verrs.Add(pathPrefix + ".privateRegistry: cannot be only whitespace if specified")
	} else if cfg.PrivateRegistry != "" && !isValidRegistryAddress(cfg.PrivateRegistry) {
		verrs.Add(pathPrefix + ".privateRegistry: invalid hostname/IP or host:port format '" + cfg.PrivateRegistry + "'")
	}
	if cfg.NamespaceOverride != "" && strings.TrimSpace(cfg.NamespaceOverride) == "" {
		verrs.Add(pathPrefix + ".namespaceOverride: cannot be only whitespace if specified")
	}
	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add(pathPrefix + ".auths: registry address key cannot be empty")
		} else if !isValidRegistryAddress(regAddr) {
			verrs.Add(fmt.Sprintf("%s.auths[\"%s\"]: registry address key '%s' is not a valid hostname or host:port", pathPrefix, regAddr, regAddr))
		}
		Validate_RegistryAuth(&auth, verrs, authPathPrefix) // Pass address of auth
	}
	if cfg.Type != nil && strings.TrimSpace(*cfg.Type) == "" {
		verrs.Add(pathPrefix + ".type: cannot be empty if specified")
	}
	if cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) == "" {
		verrs.Add(pathPrefix + ".dataRoot: cannot be only whitespace if specified")
	}
	if (cfg.Type != nil && *cfg.Type != "") && (cfg.DataRoot == nil || strings.TrimSpace(*cfg.DataRoot) == "") {
		verrs.Add(pathPrefix + ".registryDataDir (dataRoot): must be specified if registry type is set for local deployment")
	}
	if (cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) != "") && (cfg.Type == nil || strings.TrimSpace(*cfg.Type) == "") {
		verrs.Add(pathPrefix + ".type: must be specified if registryDataDir (dataRoot) is set for local deployment")
	}

	if cfg.NamespaceRewrite != nil {
		if cfg.NamespaceRewrite.Enabled {
			if len(cfg.NamespaceRewrite.Rules) == 0 {
				verrs.Add(pathPrefix + ".namespaceRewrite.rules: must contain at least one rule if rewrite is enabled")
			}
			for i, rule := range cfg.NamespaceRewrite.Rules {
				rulePathPrefix := fmt.Sprintf("%s.namespaceRewrite.rules[%d]", pathPrefix, i)
				if strings.TrimSpace(rule.OldNamespace) == "" {
					verrs.Add(rulePathPrefix + ".oldNamespace: cannot be empty")
				}
				if strings.TrimSpace(rule.NewNamespace) == "" {
					verrs.Add(rulePathPrefix + ".newNamespace: cannot be empty")
				}
				if rule.Registry != "" && strings.TrimSpace(rule.Registry) == "" {
					verrs.Add(rulePathPrefix + ".registry: cannot be only whitespace if specified")
				}
				if rule.Registry != "" && !isValidRegistryAddress(rule.Registry) {
					verrs.Add(rulePathPrefix + ".registry: invalid hostname or host:port format '" + rule.Registry + "'")
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
		verrs.Add(pathPrefix + ": either username/password or auth string must be provided")
	}
	if hasAuthStr {
		decoded, err := base64.StdEncoding.DecodeString(cfg.Auth)
		if err != nil {
			verrs.Add(pathPrefix + ".auth: failed to decode base64 auth string: " + fmt.Sprintf("%v", err))
		} else if !strings.Contains(string(decoded), ":") {
			verrs.Add(pathPrefix + ".auth: decoded auth string must be in 'username:password' format")
		}
	}
	if cfg.CertsPath != "" && strings.TrimSpace(cfg.CertsPath) == "" {
		verrs.Add(pathPrefix + ".certsPath: cannot be only whitespace if specified")
	}
}

// isValidRegistryAddress validates registry address which can be:
// - hostname
// - hostname:port
// - IP
// - IP:port
func isValidRegistryAddress(addr string) bool {
	if addr == "" {
		return false
	}

	// Check if it contains port
	parts := strings.Split(addr, ":")
	if len(parts) == 1 {
		// No port, just hostname or IP
		return util.IsValidIP(addr) || util.IsValidDomainName(addr)
	} else if len(parts) == 2 {
		// hostname:port or IP:port
		host := parts[0]
		port := parts[1]
		if port == "" {
			return false
		}
		return (util.IsValidIP(host) || util.IsValidDomainName(host)) && util.IsValidPort(port)
	}
	return false
}

// Assuming ValidationErrors is defined in cluster_types.go or a shared util.
// NOTE: DeepCopy methods should be generated by controller-gen.
// Updated SetDefaults_RegistryConfig to correctly pass address of authEntry copy.
// Updated Validate_RegistryConfig to correctly pass address of auth.
// Added import "encoding/base64", "fmt", "strings".
