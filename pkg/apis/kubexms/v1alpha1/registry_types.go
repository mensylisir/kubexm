package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type Registry struct {
	MirroringAndRewriting *RegistryMirroringAndRewriting `json:"mirroring,omitempty" yaml:"mirroring,omitempty"`
	Auths                 map[string]RegistryAuth        `json:"auths,omitempty" yaml:"auths,omitempty"`
	LocalDeployment       *LocalRegistryDeployment       `json:"local,omitempty" yaml:"local,omitempty"`
}

type RegistryMirroringAndRewriting struct {
	PrivateRegistry   string            `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string            `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	NamespaceRewrite  *NamespaceRewrite `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"`
}

type LocalRegistryDeployment struct {
	Type     string `json:"type,omitempty" yaml:"type,omitempty"`
	DataRoot string `json:"dataRoot,omitempty" yaml:"registryDataDir,omitempty"`
}

type RegistryAuth struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty" yaml:"auth,omitempty"`
	SkipTLSVerify *bool  `json:"skipTLSVerify,omitempty" yaml:"skipTLSVerify,omitempty"`
	PlainHTTP     *bool  `json:"plainHTTP,omitempty" yaml:"plainHTTP,omitempty"`
	CertsPath     string `json:"certsPath,omitempty" yaml:"certsPath,omitempty"`
}

type NamespaceRewrite struct {
	Enabled *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Rules   []NamespaceRewriteRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

type NamespaceRewriteRule struct {
	Registry     string `json:"registry,omitempty" yaml:"registry,omitempty"`
	OldNamespace string `json:"oldNamespace" yaml:"oldNamespace"`
	NewNamespace string `json:"newNamespace" yaml:"newNamespace"`
}

func SetDefaults_Registry(cfg *Registry) {
	if cfg == nil {
		return
	}
	if cfg.MirroringAndRewriting == nil {
		cfg.MirroringAndRewriting = &RegistryMirroringAndRewriting{}
	}
	SetDefaults_RegistryMirroringAndRewriting(cfg.MirroringAndRewriting)

	if cfg.Auths == nil {
		cfg.Auths = make(map[string]RegistryAuth)
	}
	for k, authEntry := range cfg.Auths {
		entryCopy := authEntry
		SetDefaults_RegistryAuth(&entryCopy)
		cfg.Auths[k] = entryCopy
	}

	if cfg.LocalDeployment == nil {
		cfg.LocalDeployment = &LocalRegistryDeployment{}
	}
	SetDefaults_LocalRegistryDeployment(cfg.LocalDeployment)
}

func SetDefaults_RegistryMirroringAndRewriting(cfg *RegistryMirroringAndRewriting) {
	if cfg == nil {
		return
	}
	if cfg.NamespaceOverride == "" {
		cfg.NamespaceOverride = common.DefaultNamespaceOverride
	}

	if cfg.NamespaceRewrite == nil {
		cfg.NamespaceRewrite = &NamespaceRewrite{}
	}
	if cfg.NamespaceRewrite.Enabled == nil {
		cfg.NamespaceRewrite.Enabled = helpers.BoolPtr(false)
	}
}

func SetDefaults_RegistryAuth(cfg *RegistryAuth) {
	if cfg == nil {
		return
	}
	if cfg.SkipTLSVerify == nil {
		cfg.SkipTLSVerify = helpers.BoolPtr(true)
	}
	if cfg.PlainHTTP == nil {
		cfg.PlainHTTP = helpers.BoolPtr(false)
	}
}

func SetDefaults_LocalRegistryDeployment(cfg *LocalRegistryDeployment) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = common.DefaultRegistryType
	}
	if cfg.DataRoot == "" {
		cfg.DataRoot = fmt.Sprintf("%s/%s", common.DefaultInstallRoot, common.DefaultRegistryType)
	}
}

func Validate_Registry(cfg *Registry, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if cfg.MirroringAndRewriting != nil {
		Validate_RegistryMirroringAndRewriting(cfg.MirroringAndRewriting, verrs, path.Join(p, "mirroring"))
	}
	for regAddr, auth := range cfg.Auths {
		authPath := fmt.Sprintf("%s.auths[\"%s\"]", p, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add(p + ".auths: registry address key cannot be empty")
			continue
		}
		if !helpers.IsValidHostPort(regAddr) {
			verrs.Add(fmt.Sprintf("%s.auths: registry address key '%s' is not a valid format", p, regAddr))
		}
		Validate_RegistryAuth(&auth, verrs, authPath)
	}

	if cfg.LocalDeployment != nil {
		Validate_LocalRegistryDeployment(cfg.LocalDeployment, verrs, path.Join(p, "local"))
	}
}

func Validate_RegistryMirroringAndRewriting(cfg *RegistryMirroringAndRewriting, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.PrivateRegistry != "" && !helpers.IsValidHostPort(cfg.PrivateRegistry) {
		verrs.Add(fmt.Sprintf("%s.privateRegistry: invalid format '%s'", pathPrefix, cfg.PrivateRegistry))
	}
	if cfg.NamespaceRewrite != nil && cfg.NamespaceRewrite.Enabled != nil && *cfg.NamespaceRewrite.Enabled {
		rewritePath := path.Join(pathPrefix, "namespaceRewrite")
		if len(cfg.NamespaceRewrite.Rules) == 0 {
			verrs.Add(rewritePath + ".rules: must contain at least one rule if rewrite is enabled")
		}
		for i, rule := range cfg.NamespaceRewrite.Rules {
			rulePath := fmt.Sprintf("%s.rules[%d]", rewritePath, i)
			if strings.TrimSpace(rule.OldNamespace) == "" {
				verrs.Add(rulePath + ".oldNamespace: cannot be empty")
			}
			if strings.TrimSpace(rule.NewNamespace) == "" {
				verrs.Add(rulePath + ".newNamespace: cannot be empty")
			}
			if rule.Registry != "" && !helpers.IsValidHostPort(rule.Registry) {
				verrs.Add(fmt.Sprintf("%s.registry: invalid format '%s'", rulePath, rule.Registry))
			}
		}
	}
}

func Validate_RegistryAuth(cfg *RegistryAuth, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	hasUserPass := cfg.Username != "" && cfg.Password != ""
	hasAuthStr := cfg.Auth != ""

	if !hasUserPass && !hasAuthStr {
		verrs.Add(pathPrefix + ": either username/password or auth string must be provided")
	}
	if hasUserPass && hasAuthStr {
		verrs.Add(pathPrefix + ": cannot provide both username/password and auth string simultaneously")
	}
	if hasAuthStr {
		decoded, err := base64.StdEncoding.DecodeString(cfg.Auth)
		if err != nil {
			verrs.Add(fmt.Sprintf("%s.auth: failed to decode base64 auth string: %v", pathPrefix, err))
		} else if !strings.Contains(string(decoded), ":") {
			verrs.Add(pathPrefix + ".auth: decoded auth string must be in 'username:password' format")
		}
	}
}

func Validate_LocalRegistryDeployment(cfg *LocalRegistryDeployment, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		verrs.Add(pathPrefix + ".type: is required for local deployment")
		return
	}
	if !helpers.ContainsString(common.ValidRegistryTypes, cfg.Type) {
		verrs.Add(fmt.Sprintf("%s.type: invalid type '%s', must be one of %v",
			pathPrefix, cfg.Type, common.ValidRegistryTypes))
	}
	if strings.TrimSpace(cfg.DataRoot) == "" {
		verrs.Add(pathPrefix + ".dataRoot: is required for local deployment")
	}
}
