package v1alpha1

import (
	"net/url"
	"regexp"
	"strings"
)

// AddonConfig defines the detailed configuration for a single addon.
// This struct is typically used when a more fine-grained configuration for addons is needed,
// potentially managed separately or through a dedicated 'addons' section in the cluster YAML
// that allows for more than just enabling/disabling by name (as seen in ClusterSpec.Addons which is []string).
// It allows specifying sources like Helm charts or YAML manifests, along with other parameters.
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
		if strings.TrimSpace(cfg.Sources.Chart.Repo) != "" {
			if strings.TrimSpace(cfg.Sources.Chart.Name) == "" {
				verrs.Add("%s.name: chart name must be specified if chart.repo is set", csPath)
			}
			// Validate Repo URL format
			_, err := url.ParseRequestURI(cfg.Sources.Chart.Repo)
			if err != nil {
				verrs.Add("%s.repo: invalid URL format for chart repo '%s': %v", csPath, cfg.Sources.Chart.Repo, err)
			}
		}
		// Validate Version format
		if cfg.Sources.Chart.Version != "" {
			if strings.TrimSpace(cfg.Sources.Chart.Version) == "" {
				verrs.Add("%s.version: chart version cannot be only whitespace if specified", csPath)
			} else if !isValidChartVersion(cfg.Sources.Chart.Version) {
				verrs.Add("%s.version: chart version '%s' is not a valid format (e.g., v1.2.3, 1.0.0, latest, stable)", csPath, cfg.Sources.Chart.Version)
			}
		}

		for i, val := range cfg.Sources.Chart.Values {
			if !strings.Contains(val, "=") {
				verrs.Add("%s.values[%d]: invalid format '%s', expected key=value", csPath, i, val)
			}
		}
		// ValuesFile path existence is more of a runtime check.
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
			// Basic check for http/https or local path (does not check existence)
			if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "/") && !strings.Contains(p, "./") {
				// This is a very basic check and might not cover all valid local path cases,
				// especially on Windows or relative paths not starting with "./".
				// Consider if a more robust path validation is needed or if this is sufficient.
				// For now, it suggests an intention for URL or absolute/relative local paths.
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

// isValidChartVersion checks if the version string matches common chart version patterns.
// Allows "latest", "stable", or versions like "1.2.3", "v1.2.3", "1.0", "v2".
// Does not aim for strict SemVer compliance but common usage.
func isValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	// Regex for versions like: v1.2.3, 1.2.3, v1.0, 1.0, v1, 1
	// Allows optional 'v', followed by digits, optionally followed by (.digits)+
	// Does not match complex pre-release/build metadata for simplicity, as chart versions often don't use them
	// or they are handled as opaque strings by Helm.
	matched, _ := regexp.MatchString(`^v?([0-9]+)(\.[0-9]+)*$`, version)
	return matched
}
