package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"net/url"
	"strings"
)

type Containerd struct {
	Endpoint         string              `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Version          string              `json:"version,omitempty" yaml:"version,omitempty"`
	Registry         *ContainerdRegistry `json:"registry,omitempty" yaml:"registry,omitempty"`
	UseSystemdCgroup *bool               `json:"useSystemdCgroup,omitempty" yaml:"useSystemdCgroup,omitempty"`
	ExtraTomlConfig  string              `json:"extraTomlConfig,omitempty" yaml:"extraTomlConfig,omitempty"`
	ConfigPath       *string             `json:"configPath,omitempty" yaml:"configPath,omitempty"`
	DisabledPlugins  []string            `json:"disabledPlugins,omitempty" yaml:"disabledPlugins,omitempty"`
	RequiredPlugins  []string            `json:"requiredPlugins,omitempty" yaml:"requiredPlugins,omitempty"`
	Imports          []string            `json:"imports,omitempty" yaml:"imports,omitempty"`
	Root             *string             `json:"root,omitempty" yaml:"root,omitempty"`
	State            *string             `json:"state,omitempty" yaml:"state,omitempty"`
	Pause            string              `json:"pause,omitempty" yaml:"pause,omitempty"`
}

type ContainerdRegistry struct {
	Mirrors map[ServerAddress]MirrorConfig `json:"mirrors,omitempty" yaml:"mirrors,omitempty"`
	Configs map[ServerAddress]AuthConfig   `json:"configs,omitempty" yaml:"configs,omitempty"`
}

type MirrorConfig struct {
	Endpoints []string `json:"endpoints" yaml:"endpoints"`
}

type ContainerdRegistryAuth struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	Password      string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty" yaml:"auth,omitempty"`
	IdentityToken string `json:"identityToken,omitempty" yaml:"identityToken,omitempty"`
}

type TLSConfig struct {
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	CAFile             string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	CertFile           string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile            string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

type AuthConfig struct {
	Auth *ContainerdRegistryAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
	TLS  *TLSConfig              `json:"tls,omitempty" yaml:"tls,omitempty"`
}

func SetDefaults_ContainerdConfig(cfg *Containerd) {
	if cfg == nil {
		return
	}
	if cfg.Registry == nil {
		cfg.Registry = &ContainerdRegistry{}
	}
	if cfg.Registry.Mirrors == nil {
		cfg.Registry.Mirrors = make(map[ServerAddress]MirrorConfig)
	}
	if cfg.Registry.Configs == nil {
		cfg.Registry.Configs = make(map[ServerAddress]AuthConfig)
	}
	if cfg.UseSystemdCgroup == nil {
		cfg.UseSystemdCgroup = helpers.BoolPtr(true)
	}
	if cfg.ConfigPath == nil {
		cfg.ConfigPath = helpers.StrPtr(common.ContainerdDefaultConfigFile)
	}
	if cfg.DisabledPlugins == nil {
		cfg.DisabledPlugins = []string{}
	}
	if cfg.RequiredPlugins == nil {
		cfg.RequiredPlugins = []string{common.ContainerdPluginCRI}
	}
	if cfg.Imports == nil {
		cfg.Imports = []string{}
	}
	if cfg.Root == nil {
		cfg.Root = helpers.StrPtr(common.ContainerdDefaultRoot)
	}
	if cfg.State == nil {
		cfg.State = helpers.StrPtr(common.ContainerdDefaultState)
	}
	if cfg.Version == "" {
		cfg.Version = common.DefaultContainerdVersion
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = common.ContainerdDefaultEndpoint
	}
	if cfg.Pause == "" {
		cfg.Pause = common.DefaultPauseImage
	}
}

func Validate_ContainerdConfig(cfg *Containerd, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Version != "" {
		if strings.TrimSpace(cfg.Version) == "" {
			verrs.Add(pathPrefix+".version", "cannot be only whitespace if specified")
		} else if !helpers.IsValidRuntimeVersion(cfg.Version) {
			verrs.Add(pathPrefix + ".version: '" + cfg.Version + "' is not a recognized version format")
		}
	}

	if cfg.Registry != nil {
		registryPath := pathPrefix + ".registry"
		for reg, mirrorCfg := range cfg.Registry.Mirrors {
			mirrorMapPath := registryPath + ".mirrors"
			mirrorEntryPath := fmt.Sprintf("%s[\"%s\"]", mirrorMapPath, reg)
			if strings.TrimSpace(string(reg)) == "" {
				verrs.Add(mirrorMapPath, "registry host key cannot be empty")
			} else if !helpers.ValidateHostPortString(string(reg)) && !helpers.IsValidDomainName(string(reg)) {
				verrs.Add(mirrorEntryPath, "registry key '"+string(reg)+"' is not a valid hostname or host:port")
			}
			if len(mirrorCfg.Endpoints) == 0 {
				verrs.Add(mirrorEntryPath, "must contain at least one endpoint URL")
			}
			for i, endpointURL := range mirrorCfg.Endpoints {
				endpointPath := fmt.Sprintf("%s.endpoints[%d]", mirrorEntryPath, i)
				u, err := url.ParseRequestURI(endpointURL)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
					verrs.Add(endpointPath, "invalid URL format for endpoint '"+endpointURL+"' (must be http or https)")
				}
			}
		}

		for reg, authCfg := range cfg.Registry.Configs {
			configMapPath := registryPath + ".configs"
			configEntryPath := fmt.Sprintf("%s[\"%s\"]", configMapPath, reg)
			if strings.TrimSpace(string(reg)) == "" {
				verrs.Add(configMapPath, "registry host key cannot be empty")
			} else if !helpers.ValidateHostPortString(string(reg)) && !helpers.IsValidDomainName(string(reg)) { // <-- 建议增加
				verrs.Add(configEntryPath, "registry key '"+string(reg)+"' is not a valid hostname or host:port")
			}
			if authCfg.Auth != nil {
				authPath := configEntryPath + ".auth"
				auth := authCfg.Auth
				hasUserPass := auth.Username != "" && auth.Password != ""
				hasAuthStr := auth.Auth != ""
				hasIdentityToken := auth.IdentityToken != ""
				if !hasUserPass && !hasAuthStr && !hasIdentityToken {
					verrs.Add(authPath, "one of username/password, auth string, or identityToken must be provided")
				}
				if hasAuthStr {
					decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
					if err != nil {
						verrs.Add(authPath+".auth", "failed to decode base64 auth string")
					} else if !strings.Contains(string(decoded), ":") {
						verrs.Add(authPath+".auth", "decoded auth string must be in 'username:password' format")
					}
				}
			}
		}
	}
	if cfg.ConfigPath != nil && strings.TrimSpace(*cfg.ConfigPath) == "" {
		verrs.Add(pathPrefix+".configPath", "cannot be empty if specified")
	}
	disabledSet := make(map[string]struct{})
	for i, plug := range cfg.DisabledPlugins {
		if strings.TrimSpace(plug) == "" {
			verrs.Add(fmt.Sprintf("%s.disabledPlugins[%d]: plugin name cannot be empty", pathPrefix, i))
		}
		disabledSet[plug] = struct{}{}
	}
	for i, plug := range cfg.RequiredPlugins {
		if strings.TrimSpace(plug) == "" {
			verrs.Add(fmt.Sprintf("%s.requiredPlugins[%d]: plugin name cannot be empty", pathPrefix, i))
		}
		if _, found := disabledSet[plug]; found {
			verrs.Add(pathPrefix, fmt.Sprintf("plugin '%s' cannot be in both requiredPlugins and disabledPlugins", plug))
		}
	}
	if cfg.Root != nil && strings.TrimSpace(*cfg.Root) == "" {
		verrs.Add(pathPrefix+".root", "cannot be empty if specified")
	}
	if cfg.State != nil && strings.TrimSpace(*cfg.State) == "" {
		verrs.Add(pathPrefix+".state", "cannot be empty if specified")
	}
	if cfg.Pause != "" {
		pausePath := pathPrefix + ".pause"
		if strings.TrimSpace(cfg.Pause) == "" {
			verrs.Add(pausePath, "cannot be only whitespace if specified")
		} else if !helpers.IsValidImageReference(cfg.Pause) {
			verrs.Add(pausePath, "invalid image reference format: '"+cfg.Pause+"'")
		}
	}
	for i, imp := range cfg.Imports {
		if strings.TrimSpace(imp) == "" {
			verrs.Add(fmt.Sprintf("%s.imports[%d]: import path cannot be empty", pathPrefix, i))
		}
	}
}
