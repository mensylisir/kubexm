package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type Addon struct {
	Name           string        `json:"name" yaml:"name"`
	Enabled        *bool         `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Retries        *int32        `json:"retries,omitempty" yaml:"retries,omitempty"`
	Delay          *int32        `json:"delay,omitempty" yaml:"delay,omitempty"`
	Sources        []AddonSource `json:"sources,omitempty" yaml:"sources,omitempty"`
	PreInstall     []string      `json:"preInstall,omitempty" yaml:"preInstall,omitempty"`
	PostInstall    []string      `json:"postInstall,omitempty" yaml:"postInstall,omitempty"`
	TimeoutSeconds *int32        `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

type AddonSource struct {
	Namespace string       `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Chart     *ChartSource `json:"chart,omitempty" yaml:"chart,omitempty"`
	Yaml      *YamlSource  `json:"yaml,omitempty" yaml:"yaml,omitempty"`
}

type ChartSource struct {
	Name       string   `json:"name,omitempty" yaml:"name,omitempty"`
	Repo       string   `json:"repo,omitempty" yaml:"repo,omitempty"`
	Path       string   `json:"path,omitempty" yaml:"path,omitempty"`
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	ValuesFile string   `json:"valuesFile,omitempty" yaml:"valuesFile,omitempty"`
	Values     []string `json:"values,omitempty" yaml:"values,omitempty"`
	Wait       *bool    `json:"wait,omitempty" yaml:"wait,omitempty"`
}

type YamlSource struct {
	Version string   `json:"version,omitempty" yaml:"version,omitempty"`
	Path    []string `json:"path,omitempty" yaml:"path,omitempty"`
}

func SetDefaults_Addon(cfg *Addon) {
	if cfg == nil {
		return
	}

	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(false)
	}
	if cfg.Retries == nil {
		cfg.Retries = helpers.Int32Ptr(0)
	}
	if cfg.Delay == nil {
		cfg.Delay = helpers.Int32Ptr(5)
	}
	if cfg.TimeoutSeconds == nil {
		cfg.TimeoutSeconds = helpers.Int32Ptr(300)
	}

	if cfg.Sources == nil {
		cfg.Sources = []AddonSource{}
	}
	if cfg.PreInstall == nil {
		cfg.PreInstall = []string{}
	}
	if cfg.PostInstall == nil {
		cfg.PostInstall = []string{}
	}

	for i := range cfg.Sources {
		SetDefaults_AddonSource(&cfg.Sources[i], cfg.Name, i)
	}
}

func SetDefaults_AddonSource(cfg *AddonSource, addonName string, index int) {
	if cfg == nil {
		return
	}
	if cfg.Namespace == "" && addonName != "" {
		cleanName := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(addonName), " ", "-"))
		cfg.Namespace = fmt.Sprintf("addon-%s-%d", cleanName, index)
	} else if cfg.Namespace == "" {
		cfg.Namespace = fmt.Sprintf("addon-unnamed-%d", index)
	}

	if cfg.Chart != nil {
		if cfg.Chart.Wait == nil {
			cfg.Chart.Wait = helpers.BoolPtr(true)
		}
		if cfg.Chart.Values == nil {
			cfg.Chart.Values = []string{}
		}
	}

	if cfg.Yaml != nil {
		if cfg.Yaml.Path == nil {
			cfg.Yaml.Path = []string{}
		}
	}
}

func Validate_Addon(cfg *Addon, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if strings.TrimSpace(cfg.Name) == "" {
		verrs.Add(p + ".name: cannot be empty")
	} else if !helpers.IsValidK8sName(cfg.Name) {
		verrs.Add(fmt.Sprintf("%s.name: '%s' is not a valid Kubernetes name (DNS-1123 compliant)", p, cfg.Name))
	}

	if cfg.Retries != nil && *cfg.Retries < 0 {
		verrs.Add(p + ".retries: cannot be negative")
	}
	if cfg.Delay != nil && *cfg.Delay < 0 {
		verrs.Add(p + ".delay: cannot be negative")
	}
	if cfg.TimeoutSeconds != nil && *cfg.TimeoutSeconds <= 0 {
		verrs.Add(p + ".timeoutSeconds: must be positive")
	}

	sourcesPath := path.Join(p, "sources")
	if cfg.Enabled != nil && *cfg.Enabled && len(cfg.Sources) == 0 {
		verrs.Add(sourcesPath + ": must contain at least one source when addon is enabled")
	}

	for i, source := range cfg.Sources {
		sourcePath := fmt.Sprintf("%s[%d]", sourcesPath, i)
		Validate_AddonSource(&source, verrs, sourcePath)
	}
}

func Validate_AddonSource(cfg *AddonSource, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if strings.TrimSpace(cfg.Namespace) == "" {
		verrs.Add(pathPrefix + ".namespace: cannot be empty")
	} else if !helpers.IsValidK8sName(cfg.Namespace) {
		verrs.Add(fmt.Sprintf("%s.namespace: '%s' is not a valid Kubernetes namespace name", pathPrefix, cfg.Namespace))
	}

	hasChart := cfg.Chart != nil
	hasYaml := cfg.Yaml != nil

	if !hasChart && !hasYaml {
		verrs.Add(pathPrefix + ": each source must have either a 'chart' or a 'yaml' definition")
		return
	}
	if hasChart && hasYaml {
		verrs.Add(pathPrefix + ": cannot define both 'chart' and 'yaml' in a single source element")
		return
	}

	if hasChart {
		Validate_ChartSource(cfg.Chart, verrs, path.Join(pathPrefix, "chart"))
	}
	if hasYaml {
		Validate_YamlSource(cfg.Yaml, verrs, path.Join(pathPrefix, "yaml"))
	}
}

func Validate_ChartSource(cfg *ChartSource, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	hasPath := strings.TrimSpace(cfg.Path) != ""
	hasName := strings.TrimSpace(cfg.Name) != ""
	hasRepo := strings.TrimSpace(cfg.Repo) != ""

	if hasPath && (hasName || hasRepo) {
		verrs.Add(fmt.Sprintf("%s: 'path' is mutually exclusive with 'name' and 'repo'", pathPrefix))
	}
	if !hasPath && !hasName {
		verrs.Add(pathPrefix + ": either 'name' (with 'repo') or 'path' must be specified")
	}
	if hasName && !hasRepo && !hasPath {
		verrs.Add(path.Join(pathPrefix, "repo") + ": must be specified when 'name' is set and 'path' is not")
	}
	if hasRepo && !hasName {
		verrs.Add(path.Join(pathPrefix, "name") + ": must be specified when 'repo' is set")
	}

	if hasRepo && !helpers.IsValidURL(cfg.Repo) {
		verrs.Add(fmt.Sprintf("%s.repo: invalid URL format for '%s'", pathPrefix, cfg.Repo))
	}

	if cfg.Version != "" && !helpers.IsValidChartVersion(cfg.Version) {
		verrs.Add(fmt.Sprintf("%s.version: invalid chart version format for '%s'", pathPrefix, cfg.Version))
	}

	for i, val := range cfg.Values {
		if !strings.Contains(val, "=") {
			verrs.Add(fmt.Sprintf("%s.values[%d]: invalid format '%s', expected key=value", pathPrefix, i, val))
		}
	}
}

func Validate_YamlSource(cfg *YamlSource, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if len(cfg.Path) == 0 {
		verrs.Add(pathPrefix + ".path: must contain at least one YAML path or URL")
	}
	for i, p := range cfg.Path {
		if strings.TrimSpace(p) == "" {
			verrs.Add(fmt.Sprintf("%s.path[%d]: path/URL cannot be empty", pathPrefix, i))
		}
	}
}
