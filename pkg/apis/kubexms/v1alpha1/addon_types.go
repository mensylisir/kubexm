package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/util" // Import util package
	"github.com/mensylisir/kubexm/pkg/util/validation" // Import validation package
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
		cfg.Enabled = util.BoolPtr(true) // Default most addons to enabled unless specified
	}

	if cfg.Namespace == "" {
		if strings.TrimSpace(cfg.Name) != "" {
			cfg.Namespace = "addon-" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(cfg.Name), " ", "-"))
		} else {
			cfg.Namespace = "addon-" // Default namespace prefix even if name is empty
		}
	}

	if cfg.Retries == nil {
		cfg.Retries = util.Int32Ptr(0) // Default to 0 retries (1 attempt)
	}
	if cfg.Delay == nil {
		cfg.Delay = util.Int32Ptr(5) // Default delay 5s
	}

	if cfg.Sources.Chart != nil {
		if cfg.Sources.Chart.Wait == nil {
			cfg.Sources.Chart.Wait = util.BoolPtr(true) // Default wait to true for charts
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
func Validate_AddonConfig(cfg *AddonConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return // Should not happen if called from a loop over an initialized slice
	}
	if strings.TrimSpace(cfg.Name) == "" {
		verrs.Add(pathPrefix+".name", "addon name cannot be empty")
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
			verrs.Add(csPath, "chart.path ('"+chart.Path+"') cannot be set if chart.name ('"+chart.Name+"') or chart.repo ('"+chart.Repo+"') is also set")
		}
		if !hasChartPath && !hasChartName {
			verrs.Add(csPath, "either chart.name (with chart.repo) or chart.path must be specified")
		}
		if hasChartRepo && !hasChartName {
			verrs.Add(csPath+".name", "chart.name must be specified if chart.repo ('"+chart.Repo+"') is set")
		}
		if hasChartName && !hasChartRepo && !hasChartPath { // Name without repo implies local path, but path field is preferred for clarity
			verrs.Add(csPath+".repo", "chart.repo must be specified if chart.name ('"+chart.Name+"') is set and chart.path is not set")
		}

		if hasChartRepo {
			if !validation.IsValidURL(chart.Repo) {
				verrs.Add(csPath+".repo", "invalid URL format for chart repo '"+chart.Repo+"'")
			}
		}

		if chart.Version != "" {
			if strings.TrimSpace(chart.Version) == "" {
				verrs.Add(csPath+".version", "chart version cannot be only whitespace if specified")
			} else if !validation.IsValidChartVersion(chart.Version) {
				verrs.Add(csPath+".version", "chart version '"+chart.Version+"' is not a valid format (e.g., v1.2.3, 1.0.0, latest, stable)")
			}
		}

		for i, val := range chart.Values {
			if !strings.Contains(val, "=") {
				verrs.Add(fmt.Sprintf("%s.values[%d]", csPath, i), fmt.Sprintf("invalid format '%s', expected key=value", val))
			}
		}
	}

	if hasYamlSource {
		ysPath := pathPrefix + ".sources.yaml"
		yamlSource := cfg.Sources.Yaml
		if len(yamlSource.Path) == 0 {
			verrs.Add(ysPath+".path", "must contain at least one YAML path or URL")
		}
		for i, p := range yamlSource.Path {
			if strings.TrimSpace(p) == "" {
				verrs.Add(fmt.Sprintf("%s.path[%d]", ysPath, i), "path/URL cannot be empty")
			}
			// Basic check for http/https or local path (does not check existence)
			// This check is indicative and not exhaustive.
			// Consider using validation.IsValidURL for URL parts if strictness is needed.
			// if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") {
			// verrs.Add(ysPath+".path["+string(i)+"]", "path '"+p+"' does not appear to be a valid URL or recognizable local path pattern")
			// }
		}
	}

	if cfg.Enabled != nil && *cfg.Enabled && !hasChartSource && !hasYamlSource {
		verrs.Add(pathPrefix, "addon '"+cfg.Name+"' is enabled but has no chart or yaml sources defined")
	}

	if cfg.Retries != nil && *cfg.Retries < 0 {
		verrs.Add(pathPrefix+".retries", fmt.Sprintf("cannot be negative, got %d", *cfg.Retries))
	}
	if cfg.Delay != nil && *cfg.Delay < 0 {
		verrs.Add(pathPrefix+".delay", fmt.Sprintf("cannot be negative, got %d", *cfg.Delay))
	}
}
