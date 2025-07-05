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

	if cfg.Namespace == "" && strings.TrimSpace(cfg.Name) != "" {
		cfg.Namespace = "addon-" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(cfg.Name), " ", "-"))
	}

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

	hasChartSource := cfg.Sources.Chart != nil
	hasYamlSource := cfg.Sources.Yaml != nil

	if hasChartSource {
		csPath := pathPrefix + ".sources.chart"
		chart := cfg.Sources.Chart

		hasChartPath := strings.TrimSpace(chart.Path) != ""
		hasChartName := strings.TrimSpace(chart.Name) != ""
		hasChartRepo := strings.TrimSpace(chart.Repo) != ""

		if hasChartPath && (hasChartName || hasChartRepo) {
			verrs.Add("%s: chart.path ('%s') cannot be set if chart.name ('%s') or chart.repo ('%s') is also set", csPath, chart.Path, chart.Name, chart.Repo)
		}
		if !hasChartPath && !hasChartName {
			verrs.Add("%s: either chart.name (with chart.repo) or chart.path must be specified", csPath)
		}
		if hasChartRepo && !hasChartName {
			verrs.Add("%s.name: chart.name must be specified if chart.repo ('%s') is set", csPath, chart.Repo)
		}
		if hasChartName && !hasChartRepo && !hasChartPath { // Name without repo implies local path, but path field is preferred for clarity
			verrs.Add("%s.repo: chart.repo must be specified if chart.name ('%s') is set and chart.path is not set", csPath, chart.Name)
		}

		if hasChartRepo {
			_, err := url.ParseRequestURI(chart.Repo)
			if err != nil {
				verrs.Add("%s.repo: invalid URL format for chart repo '%s': %v", csPath, chart.Repo, err)
			}
		}

		if chart.Version != "" {
			if strings.TrimSpace(chart.Version) == "" {
				verrs.Add("%s.version: chart version cannot be only whitespace if specified", csPath)
			} else if !isValidChartVersion(chart.Version) {
				verrs.Add("%s.version: chart version '%s' is not a valid format (e.g., v1.2.3, 1.0.0, latest, stable)", csPath, chart.Version)
			}
		}

		for i, val := range chart.Values {
			if !strings.Contains(val, "=") {
				verrs.Add("%s.values[%d]: invalid format '%s', expected key=value", csPath, i, val)
			}
		}
	}

	if hasYamlSource {
		ysPath := pathPrefix + ".sources.yaml"
		yamlSource := cfg.Sources.Yaml
		if len(yamlSource.Path) == 0 {
			verrs.Add("%s.path: must contain at least one YAML path or URL", ysPath)
		}
		for i, p := range yamlSource.Path {
			if strings.TrimSpace(p) == "" {
				verrs.Add("%s.path[%d]: path/URL cannot be empty", ysPath, i)
			}
			// Basic check for http/https or local path (does not check existence)
			// This check is indicative and not exhaustive.
			if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") {
				// verrs.Add("%s.path[%d]: path '%s' does not appear to be a valid URL or recognizable local path pattern", ysPath, i, p)
			}
		}
	}

	if cfg.Enabled != nil && *cfg.Enabled && !hasChartSource && !hasYamlSource {
		verrs.Add("%s: addon '%s' is enabled but has no chart or yaml sources defined", pathPrefix, cfg.Name)
	}

	if cfg.Retries != nil && *cfg.Retries < 0 {
		verrs.Add("%s.retries: cannot be negative, got %d", pathPrefix, *cfg.Retries)
	}
	if cfg.Delay != nil && *cfg.Delay < 0 {
		verrs.Add("%s.delay: cannot be negative, got %d", pathPrefix, *cfg.Delay)
	}
}

// isValidChartVersion checks if the version string matches common chart version patterns.
// Allows "latest", "stable", or versions like "1.2.3", "v1.2.3", "1.2", "v1.0", "1", "v2".
// It aims to be flexible but avoid overly long or clearly incorrect versions.
func isValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	// Regex for versions like: v1.2.3, 1.2.3, v1.2, 1.2, v1, 1
	// Allows optional 'v'.
	// Requires at least one digit.
	// Allows up to two dot-separated digit sequences after the first (e.g., .x.y, not .x.y.z).
	matched, _ := regexp.MatchString(`^v?([0-9]+)(\.[0-9]+){0,2}$`, version)
	return matched
}
