package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_AddonConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *AddonConfig
		expected *AddonConfig
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty input",
			input: &AddonConfig{},
			expected: &AddonConfig{
				Enabled:   util.BoolPtr(true),
				Namespace: "addon-", // Name is empty, so namespace is "addon-"
				Retries:   util.Int32Ptr(0),
				Delay:     util.Int32Ptr(5),
				Sources: AddonSources{
					Chart: nil, // Chart is nil initially
					Yaml:  nil, // Yaml is nil initially
				},
				PreInstall:     []string{},
				PostInstall:    []string{},
				TimeoutSeconds: util.Int32Ptr(300),
			},
		},
		{
			name: "basic addon with name",
			input: &AddonConfig{
				Name: "My Addon",
			},
			expected: &AddonConfig{
				Name:        "My Addon",
				Enabled:     util.BoolPtr(true),
				Namespace:   "addon-my-addon",
				Retries:     util.Int32Ptr(0),
				Delay:       util.Int32Ptr(5),
				PreInstall:  []string{},
				PostInstall: []string{},
				TimeoutSeconds: util.Int32Ptr(300),
			},
		},
		{
			name: "addon with chart source",
			input: &AddonConfig{
				Name: "Helm Addon",
				Sources: AddonSources{
					Chart: &ChartSource{},
				},
			},
			expected: &AddonConfig{
				Name:      "Helm Addon",
				Enabled:   util.BoolPtr(true),
				Namespace: "addon-helm-addon",
				Retries:   util.Int32Ptr(0),
				Delay:     util.Int32Ptr(5),
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait:   util.BoolPtr(true),
						Values: []string{},
					},
				},
				PreInstall:     []string{},
				PostInstall:    []string{},
				TimeoutSeconds: util.Int32Ptr(300),
			},
		},
		{
			name: "addon with yaml source",
			input: &AddonConfig{
				Name: "Yaml Addon",
				Sources: AddonSources{
					Yaml: &YamlSource{},
				},
			},
			expected: &AddonConfig{
				Name:      "Yaml Addon",
				Enabled:   util.BoolPtr(true),
				Namespace: "addon-yaml-addon",
				Retries:   util.Int32Ptr(0),
				Delay:     util.Int32Ptr(5),
				Sources: AddonSources{
					Yaml: &YamlSource{
						Path: []string{},
					},
				},
				PreInstall:     []string{},
				PostInstall:    []string{},
				TimeoutSeconds: util.Int32Ptr(300),
			},
		},
		{
			name: "all fields specified",
			input: &AddonConfig{
				Name:      "Full Addon",
				Enabled:   util.BoolPtr(false),
				Namespace: "custom-ns",
				Retries:   util.Int32Ptr(3),
				Delay:     util.Int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "my-chart",
						Repo:       "http://charts.example.com",
						Path:       "", // Explicitly empty to test no override
						Version:    "1.0.0",
						ValuesFile: "values.yaml",
						Values:     []string{"key=val"},
						Wait:       util.BoolPtr(false),
					},
					Yaml: &YamlSource{
						Path: []string{"path/to/manifest.yaml"},
					},
				},
				PreInstall:     []string{"echo pre"},
				PostInstall:    []string{"echo post"},
				TimeoutSeconds: util.Int32Ptr(120), // User specified value
			},
			expected: &AddonConfig{
				Name:      "Full Addon",
				Enabled:   util.BoolPtr(false),
				Namespace: "custom-ns",
				Retries:   util.Int32Ptr(3),
				Delay:     util.Int32Ptr(10),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:       "my-chart",
						Repo:       "http://charts.example.com",
						Path:       "",
						Version:    "1.0.0",
						ValuesFile: "values.yaml",
						Values:     []string{"key=val"},
						Wait:       util.BoolPtr(false),
					},
					Yaml: &YamlSource{
						Path: []string{"path/to/manifest.yaml"},
					},
				},
				PreInstall:     []string{"echo pre"},
				PostInstall:    []string{"echo post"},
				TimeoutSeconds: util.Int32Ptr(120),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_AddonConfig(tt.input)
			if tt.input == nil { // For nil input, expected is also nil effectively
				assert.Nil(t, tt.input)
			} else {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestValidate_AddonConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       *AddonConfig
		expectError bool
		errorMsg    []string // Expected error messages (substrings)
	}{
		{
			name: "valid addon with chart",
			input: &AddonConfig{
				Name:    "Valid Chart Addon",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart", Repo: "http://example.com/charts", Version: "1.2.3"},
				},
			},
			expectError: false,
		},
		{
			name: "valid addon with chart path",
			input: &AddonConfig{
				Name:    "Valid Chart Path Addon",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Path: "./charts/my-local-chart"},
				},
			},
			expectError: false,
		},
		{
			name: "valid addon with yaml",
			input: &AddonConfig{
				Name:    "Valid Yaml Addon",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Yaml: &YamlSource{Path: []string{"http://example.com/manifest.yaml"}},
				},
			},
			expectError: false,
		},
		{
			name: "addon name empty",
			input: &AddonConfig{
				Name: "  ",
			},
			expectError: true,
			errorMsg:    []string{"addon name cannot be empty"},
		},
		{
			name: "chart path and name/repo conflict",
			input: &AddonConfig{
				Name:    "Chart Conflict",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Path: "./local", Name: "my-chart", Repo: "http://example.com"},
				},
			},
			expectError: true,
			errorMsg:    []string{"chart.path", "cannot be set if chart.name", "or chart.repo", "is also set"},
		},
		{
			name: "chart no path and no name",
			input: &AddonConfig{
				Name:    "Chart Incomplete",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Repo: "http://example.com"},
				},
			},
			expectError: true,
			errorMsg:    []string{"either chart.name (with chart.repo) or chart.path must be specified"},
		},
		{
			name: "chart repo without name",
			input: &AddonConfig{
				Name:    "Chart Repo No Name",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Repo: "http://example.com"},
				},
			},
			expectError: true,
			// This will trigger "either chart.name ... or chart.path" first based on current logic
			// and also "chart.name must be specified if chart.repo ... is set"
			errorMsg: []string{"chart.name must be specified if chart.repo", "is set"},
		},
		{
			name: "chart name without repo (and no path)",
			input: &AddonConfig{
				Name:    "Chart Name No Repo",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart"},
				},
			},
			expectError: true,
			errorMsg:    []string{"chart.repo must be specified if chart.name", "is set and chart.path is not set"},
		},
		{
			name: "chart invalid repo url",
			input: &AddonConfig{
				Name:    "Chart Invalid Repo",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart", Repo: "not a url"},
				},
			},
			expectError: true,
			errorMsg:    []string{"invalid URL format for chart repo"},
		},
		{
			name: "chart invalid version format",
			input: &AddonConfig{
				Name:    "Chart Invalid Version",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart", Repo: "http://example.com", Version: "v1.2.3.4.5"},
				},
			},
			expectError: true,
			errorMsg:    []string{"is not a valid format"},
		},
		{
			name: "chart version only whitespace",
			input: &AddonConfig{
				Name:    "Chart Whitespace Version",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart", Repo: "http://example.com", Version: "  "},
				},
			},
			expectError: true,
			errorMsg:    []string{"chart version cannot be only whitespace"},
		},
		{
			name: "chart invalid values format",
			input: &AddonConfig{
				Name:    "Chart Invalid Values",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{Name: "my-chart", Repo: "http://example.com", Values: []string{"keyvalue"}},
				},
			},
			expectError: true,
			errorMsg:    []string{"invalid format 'keyvalue', expected key=value"},
		},
		{
			name: "yaml source path empty list",
			input: &AddonConfig{
				Name:    "Yaml Empty Path List",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Yaml: &YamlSource{Path: []string{}},
				},
			},
			expectError: true,
			errorMsg:    []string{"must contain at least one YAML path or URL"},
		},
		{
			name: "yaml source path contains empty string",
			input: &AddonConfig{
				Name:    "Yaml Empty Path String",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{
					Yaml: &YamlSource{Path: []string{"  "}},
				},
			},
			expectError: true,
			errorMsg:    []string{"path/URL cannot be empty"},
		},
		{
			name: "enabled addon with no sources",
			input: &AddonConfig{
				Name:    "Enabled No Sources",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{}, // No chart or yaml
			},
			expectError: true,
			errorMsg:    []string{"enabled but has no chart or yaml sources defined"},
		},
		{
			name: "negative retries",
			input: &AddonConfig{
				Name:    "Negative Retries",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{Yaml: &YamlSource{Path: []string{"a.yaml"}}}, // Valid source
				Retries: util.Int32Ptr(-1),
			},
			expectError: true,
			errorMsg:    []string{"retries: cannot be negative"},
		},
		{
			name: "negative delay",
			input: &AddonConfig{
				Name:    "Negative Delay",
				Enabled: util.BoolPtr(true),
				Sources: AddonSources{Yaml: &YamlSource{Path: []string{"a.yaml"}}}, // Valid source
				Delay:   util.Int32Ptr(-1),
			},
			expectError: true,
			errorMsg:    []string{"delay: cannot be negative"},
		},
		{
			name: "disabled addon with no sources (should be valid)",
			input: &AddonConfig{
				Name:    "Disabled No Sources",
				Enabled: util.BoolPtr(false),
			},
			expectError: false,
		},
		{
			name: "valid timeoutSeconds",
			input: &AddonConfig{
				Name:           "Timeout Addon",
				Enabled:        util.BoolPtr(true),
				Sources:        AddonSources{Yaml: &YamlSource{Path: []string{"a.yaml"}}},
				TimeoutSeconds: util.Int32Ptr(300),
			},
			expectError: false,
		},
		{
			name: "zero timeoutSeconds",
			input: &AddonConfig{
				Name:           "Zero Timeout Addon",
				Enabled:        util.BoolPtr(true),
				Sources:        AddonSources{Yaml: &YamlSource{Path: []string{"a.yaml"}}},
				TimeoutSeconds: util.Int32Ptr(0),
			},
			expectError: false,
		},
		{
			name: "negative timeoutSeconds",
			input: &AddonConfig{
				Name:           "Negative Timeout Addon",
				Enabled:        util.BoolPtr(true),
				Sources:        AddonSources{Yaml: &YamlSource{Path: []string{"a.yaml"}}},
				TimeoutSeconds: util.Int32Ptr(-5),
			},
			expectError: true,
			errorMsg:    []string{".timeoutSeconds", "cannot be negative"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			Validate_AddonConfig(tt.input, verrs, "spec.addons[0]")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected validation errors but got none for test: %s", tt.name)
				if len(tt.errorMsg) > 0 {
					fullErrorMsg := verrs.Error()
					for _, subMsg := range tt.errorMsg {
						assert.Contains(t, fullErrorMsg, subMsg, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, subMsg, fullErrorMsg)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no validation errors but got: %s for test: %s", verrs.Error(), tt.name)
			}
		})
	}
}

// TestMain is a standard way to set up and tear down tests if needed.
// For this specific test file, it's not strictly necessary but good practice to include.
func TestMain(m *testing.M) {
	// Example:
	// setup()
	code := m.Run()
	// shutdown()
	// os.Exit(code) // Usually not needed as `go test` handles exit codes.
	if code != 0 {
		// Perform any specific logging or actions on test failure if needed.
	}
}

// TestIsValidChartVersionViaValidation provides more targeted tests for IsValidChartVersion behavior.
// This helps confirm the understanding of the validation logic used in AddonConfig.
func TestIsValidChartVersionViaValidation(t *testing.T) {
	// Versions considered valid by the current common.ValidChartVersionRegexString and validation.IsValidChartVersion
	correctedValidVersions := []string{"1.0.0", "v1.2.3", "latest", "stable", "1.2", "v1", "1", "10.20.30", "v0.0.1"}

	// Versions considered invalid by the current common.ValidChartVersionRegexString and validation.IsValidChartVersion
	correctedInvalidVersions := []string{
		"",                 // Empty
		"v1.2.3.4",         // Too many segments
		"1.2.3-$%",         // Invalid characters
		"0.1.0-alpha.1",    // Contains pre-release (not supported by current regex)
		"1.2.3-rc1+build42", // Contains pre-release and build meta (not supported by current regex)
		"v-beta",           // Starts with 'v-'
		"a.b.c",            // Non-numeric segments
		"1.0.0 ",           // Trailing space
		" 1.0.0",           // Leading space
		"latest1",          // "latest" with suffix
		"stable-pre",       // "stable" with suffix
	}

	for _, v := range correctedValidVersions {
		t.Run("corrected_valid_version_"+v, func(t *testing.T) {
			assert.True(t, validation.IsValidChartVersion(v), "Expected version '%s' to be valid by current IsValidChartVersion", v)
		})
	}
	for _, v := range correctedInvalidVersions {
		t.Run("corrected_invalid_version_"+v, func(t *testing.T) {
			assert.False(t, validation.IsValidChartVersion(v), "Expected version '%s' to be invalid by current IsValidChartVersion", v)
		})
	}
}
