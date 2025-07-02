package v1alpha1

import (
	"net/url"
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
		// Validate Version format (basic check for non-empty if specified)
		if cfg.Sources.Chart.Version != "" && strings.TrimSpace(cfg.Sources.Chart.Version) == "" {
			verrs.Add("%s.version: chart version cannot be only whitespace if specified", csPath)
		}
		// Example of a more specific version format validation (e.g., semantic versioning)
		// This is a basic example, a proper semver library might be used for stricter checks.
		if cfg.Sources.Chart.Version != "" && !isValidVersion(cfg.Sources.Chart.Version) {
			// verrs.Add("%s.version: chart version '%s' is not a valid semantic version (e.g., v1.2.3, 1.0.0)", csPath, cfg.Sources.Chart.Version)
			// Allowing non-semantic versions for now as charts can have other versioning schemes.
			// We'll just ensure it's not just whitespace if set.
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

// isValidVersion is a simple helper to validate common version string patterns.
// It allows versions like "1.2.3", "v1.2.3", "0.1.0-alpha+001", "latest", "stable".
// This is not a strict semantic version validator but covers many common chart versions.
func isValidVersion(version string) bool {
	if strings.TrimSpace(version) == "" {
		return false // Empty or whitespace-only is not valid if version field is used
	}
	// Regex to match common version patterns:
	// - Starts with 'v' or a digit.
	// - Can include dots, hyphens (for pre-releases), plus (for build metadata).
	// - Allows "latest", "stable" as special keywords.
	// This regex is quite permissive.
	// For strict semver, a dedicated library or more complex regex would be needed.
	// Example: ^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$
	// For now, we are only checking if it's not just whitespace if set, so this function is more illustrative.
	// The actual check `cfg.Sources.Chart.Version != "" && strings.TrimSpace(cfg.Sources.Chart.Version) == ""`
	// in `Validate_AddonConfig` handles the "not just whitespace" part.
	// If a more specific validation was enabled (like the commented out semver check), this function would be used.
	return true // Simplified: actual non-whitespace check is done inline.
}
