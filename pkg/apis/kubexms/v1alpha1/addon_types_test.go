package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// boolPtr and int32Ptr are defined in zz_helpers.go and available in the package.

func TestSetDefaults_AddonConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *AddonConfig
		expected *AddonConfig
	}{
		{
			name:     "nil config",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &AddonConfig{}, // Minimal valid input for defaulting
			expected: &AddonConfig{
				Enabled:     boolPtr(true),
				Namespace:   "", // Name is empty, so namespace is not defaulted
				Retries:     int32Ptr(0),
				Delay:       int32Ptr(5),
				Sources:     AddonSources{Chart: nil, Yaml: nil},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name:  "config with name, namespace should default",
			input: &AddonConfig{Name: "My Addon"},
			expected: &AddonConfig{
				Name:        "My Addon",
				Enabled:     boolPtr(true),
				Namespace:   "addon-my-addon", // Defaulted
				Retries:     int32Ptr(0),
				Delay:       int32Ptr(5),
				Sources:     AddonSources{Chart: nil, Yaml: nil},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name:  "config with name and existing namespace, namespace should not be overridden",
			input: &AddonConfig{Name: "My Addon", Namespace: "custom-ns"},
			expected: &AddonConfig{
				Name:        "My Addon",
				Enabled:     boolPtr(true),
				Namespace:   "custom-ns", // Not overridden
				Retries:     int32Ptr(0),
				Delay:       int32Ptr(5),
				Sources:     AddonSources{Chart: nil, Yaml: nil},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "empty config with empty sources to default chart/yaml internals",
			input: &AddonConfig{
				Name: "test-addon", // Add name for namespace defaulting
				Sources: AddonSources{
					Chart: &ChartSource{}, // Chart is non-nil
					Yaml:  &YamlSource{},  // Yaml is non-nil
				},
			},
			expected: &AddonConfig{
				Name:      "test-addon",
				Enabled:   boolPtr(true),
				Namespace: "addon-test-addon",
				Retries:   int32Ptr(0),
				Delay:     int32Ptr(5),
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait:   boolPtr(true),
						Values: []string{},
					},
					Yaml: &YamlSource{
						Path: []string{},
					},
				},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name:  "enabled explicitly false",
			input: &AddonConfig{Name: "off-addon", Enabled: boolPtr(false)},
			expected: &AddonConfig{
				Name:        "off-addon",
				Enabled:     boolPtr(false),
				Namespace:   "addon-off-addon",
				Retries:     int32Ptr(0),
				Delay:       int32Ptr(5),
				Sources:     AddonSources{Chart: nil, Yaml: nil},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "chart source with wait explicitly false",
			input: &AddonConfig{
				Name: "chart-wait-false",
				Sources: AddonSources{
					Chart: &ChartSource{Wait: boolPtr(false)},
				},
			},
			expected: &AddonConfig{
				Name:      "chart-wait-false",
				Enabled:   boolPtr(true),
				Namespace: "addon-chart-wait-false",
				Retries:   int32Ptr(0),
				Delay:     int32Ptr(5),
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait:   boolPtr(false),
						Values: []string{}, // Defaulted
					},
					Yaml: nil,
				},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "all fields specified, no overrides by defaults",
			input: &AddonConfig{
				Name:      "my-addon",
				Enabled:   boolPtr(false),
				Namespace: "my-ns", // User specified namespace
				Retries:   int32Ptr(3),
				Delay:     int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "nginx",
						Repo:       "https://charts.bitnami.com/bitnami",
						Path:       "", // Explicitly empty to test logic with Name/Repo
						Version:    "9.3.2",
						ValuesFile: "values.yaml",
						Values:     []string{"service.type=LoadBalancer"},
						Wait:       boolPtr(false),
					},
					Yaml: &YamlSource{
						Path: []string{"path/to/manifest.yaml"},
					},
				},
				PreInstall:  []string{"echo pre"},
				PostInstall: []string{"echo post"},
			},
			expected: &AddonConfig{
				Name:      "my-addon",
				Enabled:   boolPtr(false),
				Namespace: "my-ns", // User specified namespace, not overridden
				Retries:   int32Ptr(3),
				Delay:     int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "nginx",
						Repo:       "https://charts.bitnami.com/bitnami",
						Path:       "",
						Version:    "9.3.2",
						ValuesFile: "values.yaml",
						Values:     []string{"service.type=LoadBalancer"},
						Wait:       boolPtr(false),
					},
					Yaml: &YamlSource{
						Path: []string{"path/to/manifest.yaml"},
					},
				},
				PreInstall:  []string{"echo pre"},
				PostInstall: []string{"echo post"},
			},
		},
		{
			name:  "retries and delay specified",
			input: &AddonConfig{Name: "retry-addon", Retries: int32Ptr(2), Delay: int32Ptr(15)},
			expected: &AddonConfig{
				Name:        "retry-addon",
				Enabled:     boolPtr(true),
				Namespace:   "addon-retry-addon",
				Retries:     int32Ptr(2),
				Delay:       int32Ptr(15),
				Sources:     AddonSources{Chart: nil, Yaml: nil},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_AddonConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input, "SetDefaults_AddonConfig() mismatch")
			}
		})
	}
}

func TestIsValidChartVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"valid full semver", "1.2.3", true},
		{"valid full semver with v", "v1.2.3", true},
		{"valid minor version", "1.2", true},
		{"valid minor version with v", "v1.2", true},
		{"valid major version", "1", true},
		{"valid major version with v", "v1", true},
		{"valid latest", "latest", true},
		{"valid stable", "stable", true},
		{"invalid too many parts", "1.2.3.4", false},
		{"invalid chars", "1.2-alpha", false},
		{"invalid leading dot", ".1.2.3", false},
		{"invalid trailing dot", "1.2.3.", false},
		{"empty string", "", false},
		{"only v", "v", false},
		{"v with letters", "v1a", false},
		{"letters", "abc", false},
		{"version with space", "1.2.3 ", false},
		{"version with leading space", " 1.2.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidChartVersion(tt.version); got != tt.want {
				t.Errorf("isValidChartVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate_AddonConfig(t *testing.T) {
	validBaseChartSource := AddonSources{Chart: &ChartSource{Name: "metrics-server", Repo: "https://charts.bitnami.com/bitnami"}}
	validBaseYamlSource := AddonSources{Yaml: &YamlSource{Path: []string{"manifest.yaml"}}}

	tests := []struct {
		name        string
		input       *AddonConfig
		expectErr   bool
		errContains []string
	}{
		{
			name:      "valid chart addon",
			input:     &AddonConfig{Name: "metrics-server", Enabled: boolPtr(true), Sources: validBaseChartSource},
			expectErr: false,
		},
		{
			name:      "valid yaml addon",
			input:     &AddonConfig{Name: "dashboard", Enabled: boolPtr(true), Sources: validBaseYamlSource},
			expectErr: false,
		},
		{
			name:      "valid local chart addon",
			input:     &AddonConfig{Name: "local-chart", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Path: "./charts/mychart"}}},
			expectErr: false,
		},
		{
			name:      "addon disabled, no sources needed",
			input:     &AddonConfig{Name: "disabled-addon", Enabled: boolPtr(false)},
			expectErr: false,
		},
		{
			name:        "empty name",
			input:       &AddonConfig{Name: " ", Enabled: boolPtr(true), Sources: validBaseYamlSource},
			expectErr:   true,
			errContains: []string{".name: addon name cannot be empty"},
		},
		{
			name:        "chart source with repo but empty name",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Repo: "myrepo", Name: " "}}},
			expectErr:   true,
			errContains: []string{".sources.chart.name: chart.name must be specified if chart.repo ('myrepo') is set"},
		},
		{
			name:        "chart source with name but no repo (and no path)",
			input:       &AddonConfig{Name: "test-no-repo", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "mychart"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.repo: chart.repo must be specified if chart.name ('mychart') is set and chart.path is not set"},
		},
		{
			name:        "chart source with path and name",
			input:       &AddonConfig{Name: "test-path-and-name", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Path: "./local", Name: "mychart"}}},
			expectErr:   true,
			errContains: []string{".sources.chart: chart.path ('./local') cannot be set if chart.name ('mychart') or chart.repo ('') is also set"},
		},
		{
			name:        "chart source with path and repo",
			input:       &AddonConfig{Name: "test-path-and-repo", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Path: "./local", Repo: "myrepo"}}},
			expectErr:   true,
			errContains: []string{".sources.chart: chart.path ('./local') cannot be set if chart.name ('') or chart.repo ('myrepo') is also set"},
		},
		{
			name:        "chart source with path, name, and repo",
			input:       &AddonConfig{Name: "test-path-name-repo", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Path: "./local", Name: "mychart", Repo: "myrepo"}}},
			expectErr:   true,
			errContains: []string{".sources.chart: chart.path ('./local') cannot be set if chart.name ('mychart') or chart.repo ('myrepo') is also set"},
		},
		{
			name:        "chart source with invalid repo URL",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "http://invalid domain/"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.repo: invalid URL format for chart repo"},
		},
		{
			name:      "chart source with valid repo URL",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com"}}},
			expectErr: false,
		},
		{
			name:        "chart source with whitespace version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "   "}}},
			expectErr:   true,
			errContains: []string{".sources.chart.version: chart version cannot be only whitespace if specified"},
		},
		{
			name:      "chart source with valid version 1.2.3",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1.2.3"}}},
			expectErr: false,
		},
		{
			name:      "chart source with valid version v1.2",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "v1.2"}}},
			expectErr: false,
		},
		{
			name:      "chart source with valid version 1",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1"}}},
			expectErr: false,
		},
		{
			name:        "chart source with invalid version 1.2.3.4",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1.2.3.4"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.version: chart version '1.2.3.4' is not a valid format"},
		},
		{
			name:      "chart source with 'latest' version",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "latest"}}},
			expectErr: false,
		},
		{
			name:        "chart source with invalid char in version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1.2_invalid"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.version: chart version '1.2_invalid' is not a valid format"},
		},
		{
			name:        "chart source with invalid values format",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Values: []string{"keyvalue"}}}},
			expectErr:   true,
			errContains: []string{".sources.chart.values[0]: invalid format 'keyvalue', expected key=value"},
		},
		{
			name:      "chart source with valid values format",
			input:     &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Values: []string{"key=value", "another.key=another.value"}}}},
			expectErr: false,
		},
		{
			name:        "chart source with neither name nor path",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{}}},
			expectErr:   true,
			errContains: []string{".sources.chart: either chart.name (with chart.repo) or chart.path must be specified"},
		},
		{
			name:        "yaml source with empty path list",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Yaml: &YamlSource{Path: []string{}}}},
			expectErr:   true,
			errContains: []string{".sources.yaml.path: must contain at least one YAML path or URL"},
		},
		{
			name:        "yaml source with empty string in path list",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Yaml: &YamlSource{Path: []string{"", "valid.yaml"}}}},
			expectErr:   true,
			errContains: []string{".sources.yaml.path[0]: path/URL cannot be empty"},
		},
		{
			name:        "negative retries",
			input:       &AddonConfig{Name: "test", Retries: int32Ptr(-1), Sources: validBaseChartSource},
			expectErr:   true,
			errContains: []string{".retries: cannot be negative"},
		},
		{
			name:        "negative delay",
			input:       &AddonConfig{Name: "test", Delay: int32Ptr(-1), Sources: validBaseChartSource},
			expectErr:   true,
			errContains: []string{".delay: cannot be negative"},
		},
		{
			name:        "enabled but no source defined",
			input:       &AddonConfig{Name: "no-source-addon", Enabled: boolPtr(true)},
			expectErr:   true,
			errContains: []string{"addon 'no-source-addon' is enabled but has no chart or yaml sources defined"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults before validation, as the validation logic might expect defaulted fields.
			SetDefaults_AddonConfig(tt.input)

			verrs := &ValidationErrors{Errors: []string{}} // Assuming ValidationErrors is available from the package
			Validate_AddonConfig(tt.input, verrs, "spec.addons[0]")

			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected validation errors, but got none for test: %s", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
