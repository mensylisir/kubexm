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
			name:  "nil config",
			input: nil,
			expected: nil,
		},
		{
			name: "empty config",
			input: &AddonConfig{}, // Minimal valid input for defaulting
			expected: &AddonConfig{
				Enabled:     boolPtr(true),
				Retries:     int32Ptr(0),
				Delay:       int32Ptr(5),
				Sources:     AddonSources{Chart: nil, Yaml: nil}, // Chart & Yaml are nil, so their sub-fields not defaulted here
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "empty config with empty sources to default chart/yaml internals",
			input: &AddonConfig{
				Sources: AddonSources{
					Chart: &ChartSource{}, // Chart is non-nil
					Yaml:  &YamlSource{},  // Yaml is non-nil
				},
			},
			expected: &AddonConfig{
				Enabled: boolPtr(true),
				Retries: int32Ptr(0),
				Delay:   int32Ptr(5),
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
			name: "enabled explicitly false",
			input: &AddonConfig{Enabled: boolPtr(false)},
			expected: &AddonConfig{
				Enabled:     boolPtr(false),
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
				Sources: AddonSources{
					Chart: &ChartSource{Wait: boolPtr(false)},
				},
			},
			expected: &AddonConfig{
				Enabled: boolPtr(true),
				Retries: int32Ptr(0),
				Delay:   int32Ptr(5),
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
				Namespace: "my-ns",
				Retries:   int32Ptr(3),
				Delay:     int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "nginx",
						Repo:       "https://charts.bitnami.com/bitnami",
						Path:       "charts/nginx",
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
				Namespace: "my-ns",
				Retries:   int32Ptr(3),
				Delay:     int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "nginx",
						Repo:       "https://charts.bitnami.com/bitnami",
						Path:       "charts/nginx",
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
			name: "retries and delay specified",
			input: &AddonConfig{Retries: int32Ptr(2), Delay: int32Ptr(15)},
			expected: &AddonConfig{
				Enabled:     boolPtr(true),
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
			name:        "valid chart addon",
			input:       &AddonConfig{Name: "metrics-server", Enabled: boolPtr(true), Sources: validBaseChartSource},
			expectErr:   false,
		},
		{
			name:        "valid yaml addon",
			input:       &AddonConfig{Name: "dashboard", Enabled: boolPtr(true), Sources: validBaseYamlSource},
			expectErr:   false,
		},
		{
			name:        "valid local chart addon",
			input:       &AddonConfig{Name: "local-chart", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Path: "./charts/mychart"}}},
			expectErr:   false,
		},
		{
			name:        "addon disabled, no sources needed",
			input:       &AddonConfig{Name: "disabled-addon", Enabled: boolPtr(false)},
			expectErr:   false, // Validation for no sources when enabled is commented out in original code
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
			errContains: []string{".sources.chart.name: chart name must be specified if chart.repo is set"},
		},
		{
			name:        "chart source with invalid repo URL",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "http://invalid domain/"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.repo: invalid URL format for chart repo"},
		},
		{
			name:        "chart source with valid repo URL",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com"}}},
			expectErr:   false,
		},
		{
			name:        "chart source with whitespace version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "   "}}},
			expectErr:   true,
			errContains: []string{".sources.chart.version: chart version cannot be only whitespace if specified"},
		},
		{
			name:        "chart source with valid version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1.2.3"}}},
			expectErr:   false,
		},
		{
			name:        "chart source with 'latest' version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "latest"}}},
			expectErr:   false,
		},
		{
			name:        "chart source with 'v1' version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "v1"}}},
			expectErr:   false,
		},
		{
			name:        "chart source with '1' version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1"}}},
			expectErr:   false,
		},
		{
			name:        "chart source with invalid char in version",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Version: "1.2.3_invalid"}}},
			expectErr:   true,
			errContains: []string{".sources.chart.version: chart version '1.2.3_invalid' is not a valid format"},
		},
		{
			name:        "chart source with invalid values format",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Values: []string{"keyvalue"}}}},
			expectErr:   true,
			errContains: []string{".sources.chart.values[0]: invalid format 'keyvalue', expected key=value"},
		},
		{
			name:        "chart source with valid values format",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{Name: "good-name", Repo: "https://charts.example.com", Values: []string{"key=value", "another.key=another.value"}}}},
			expectErr:   false,
		},
		{
			name:        "chart source with neither name nor path",
			input:       &AddonConfig{Name: "test", Enabled: boolPtr(true), Sources: AddonSources{Chart: &ChartSource{}}},
			expectErr:   true,
			errContains: []string{".sources.chart: either chart.name (with repo) or chart.path (for local chart) must be specified"},
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
		// As per original addon_types.go, the check for "enabled but no source" is commented out.
		// If it were active, this test would be relevant:
		// {
		//	name: "enabled but no source defined",
		//	input: &AddonConfig{Name: "no-source-addon", Enabled: boolPtr(true)},
		//	expectErr: true,
		//	errContains: []string{"addon 'no-source-addon' is enabled but has no chart or yaml sources defined"},
		// },
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
