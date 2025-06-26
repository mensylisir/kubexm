package v1alpha1

import "strings"

// AddonConfig defines the configuration for a single addon.
type AddonConfig struct {
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled,omitempty"` // Pointer for optionality, defaults to true or based on addon

	Namespace   string   `json:"namespace,omitempty"`
	Retries     *int32   `json:"retries,omitempty"`
	Delay       *int32   `json:"delay,omitempty"` // Delay in seconds between retries

	Sources     AddonSources `json:"sources,omitempty"`

	// PreInstall and PostInstall scripts. For simplicity, these are string arrays.
	PreInstall  []string `json:"preInstall,omitempty"`
	PostInstall []string `json:"postInstall,omitempty"`
}

// AddonSources defines the sources for an addon's manifests or charts.
type AddonSources struct {
	Chart *ChartSource `json:"chart,omitempty"`
	Yaml  *YamlSource  `json:"yaml,omitempty"`
}

// ChartSource defines how to install an addon from a Helm chart.
type ChartSource struct {
	Name       string   `json:"name,omitempty"` // Chart name
	Repo       string   `json:"repo,omitempty"` // Chart repository URL
	Path       string   `json:"path,omitempty"` // Path to chart in repo (if not just name) or local path
	Version    string   `json:"version,omitempty"`
	ValuesFile string   `json:"valuesFile,omitempty"` // Path to a custom values file
	Values     []string `json:"values,omitempty"`     // Inline values (e.g., "key1=value1,key2.subkey=value2")
	Wait       *bool    `json:"wait,omitempty"`       // Whether to wait for chart resources to be ready
}

// YamlSource defines how to install an addon from YAML manifests.
type YamlSource struct {
	// Paths is a list of URLs or local file paths to YAML manifests.
	Path []string `json:"path,omitempty"`
}

// SetDefaults_AddonConfig sets default values for AddonConfig.
func SetDefaults_AddonConfig(cfg *AddonConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(true) // Default most addons to enabled unless specified
	}
	// Default namespace could be addon name, or a global addons namespace.
	// For now, leave empty; specific addons might default this if cfg.Namespace == ""

	if cfg.Retries == nil {
		cfg.Retries = int32Ptr(0) // Default to 0 retries (1 attempt)
	}
	if cfg.Delay == nil {
		cfg.Delay = int32Ptr(5) // Default delay 5s
	}

	if cfg.Sources.Chart != nil {
		if cfg.Sources.Chart.Wait == nil {
			cfg.Sources.Chart.Wait = boolPtr(true) // Default wait to true for charts
		}
		if cfg.Sources.Chart.Values == nil {
			cfg.Sources.Chart.Values = []string{}
		}
	}
	if cfg.Sources.Yaml != nil && cfg.Sources.Yaml.Path == nil {
		cfg.Sources.Yaml.Path = []string{}
	}

	if cfg.PreInstall == nil { cfg.PreInstall = []string{} }
	if cfg.PostInstall == nil { cfg.PostInstall = []string{} }
}

// Validate_AddonConfig validates AddonConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_AddonConfig(cfg *AddonConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return // Should not happen if called from a loop over an initialized slice
	}
	if strings.TrimSpace(cfg.Name) == "" {
		verrs.Add("%s.name: addon name cannot be empty", pathPrefix)
	}

	hasSource := false
	if cfg.Sources.Chart != nil {
		hasSource = true
		csPath := pathPrefix + ".sources.chart"
		// Chart name is usually required if repo is specified, or path if local.
		if strings.TrimSpace(cfg.Sources.Chart.Name) == "" && strings.TrimSpace(cfg.Sources.Chart.Path) == "" {
			verrs.Add("%s: either chart.name (with repo) or chart.path (for local chart) must be specified", csPath)
		}
		if strings.TrimSpace(cfg.Sources.Chart.Repo) != "" && strings.TrimSpace(cfg.Sources.Chart.Name) == "" {
		   verrs.Add("%s.name: chart name must be specified if chart.repo is set", csPath)
		}
		// Further validation for Repo URL format, Version format, ValuesFile path existence (runtime check) etc. could be added.
	}

	if cfg.Sources.Yaml != nil {
		hasSource = true
		ysPath := pathPrefix + ".sources.yaml"
		if len(cfg.Sources.Yaml.Path) == 0 {
			verrs.Add("%s.path: must contain at least one YAML path or URL", ysPath)
		}
		for i, p := range cfg.Sources.Yaml.Path {
			if strings.TrimSpace(p) == "" {
				verrs.Add("%s.path[%d]: path/URL cannot be empty", ysPath, i)
			}
		}
	}

	if cfg.Enabled != nil && *cfg.Enabled && !hasSource {
		// This validation can be tricky. Some "addons" might just be flags that enable features,
		// not necessarily deploying manifests. For now, we'll comment it out.
		// It might be better to validate sources only if a type (chart/yaml) is explicitly chosen.
		// verrs.Add("%s: addon '%s' is enabled but has no chart or yaml sources defined", pathPrefix, cfg.Name)
	}

	if cfg.Retries != nil && *cfg.Retries < 0 {
	   verrs.Add("%s.retries: cannot be negative, got %d", pathPrefix, *cfg.Retries)
	}
	if cfg.Delay != nil && *cfg.Delay < 0 {
	   verrs.Add("%s.delay: cannot be negative, got %d", pathPrefix, *cfg.Delay)
	}
}
