package v1alpha1

import (
	"strings"
	"testing"

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
			name:  "nil config",
			input: nil,
		},
		{
			name: "empty config",
			input: &AddonConfig{
				Name: "test-addon",
			},
			expected: &AddonConfig{
				Name:      "test-addon",
				Enabled:   boolPtr(true),
				Namespace: "addon-test-addon",
				Retries:   int32Ptr(0),
				Delay:     int32Ptr(5),
				Sources: AddonSources{
					Chart: nil, // Not initialized if not present
					Yaml:  nil, // Not initialized if not present
				},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "config with chart",
			input: &AddonConfig{
				Name: " prometheus ", // Test trimming and lowercasing
				Sources: AddonSources{
					Chart: &ChartSource{},
				},
			},
			expected: &AddonConfig{
				Name:      " prometheus ",
				Enabled:   boolPtr(true),
				Namespace: "addon-prometheus",
				Retries:   int32Ptr(0),
				Delay:     int32Ptr(5),
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait:   boolPtr(true),
						Values: []string{},
					},
					Yaml: nil,
				},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "config with yaml",
			input: &AddonConfig{
				Name: "nginx-ingress",
				Sources: AddonSources{
					Yaml: &YamlSource{},
				},
			},
			expected: &AddonConfig{
				Name:      "nginx-ingress",
				Enabled:   boolPtr(true),
				Namespace: "addon-nginx-ingress",
				Retries:   int32Ptr(0),
				Delay:     int32Ptr(5),
				Sources: AddonSources{
					Chart: nil,
					Yaml: &YamlSource{
						Path: []string{},
					},
				},
				PreInstall:  []string{},
				PostInstall: []string{},
			},
		},
		{
			name: "config with some values pre-filled",
			input: &AddonConfig{
				Name:      "my-addon",
				Enabled:   boolPtr(false),
				Namespace: "custom-ns",
				Retries:   int32Ptr(3),
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait: boolPtr(false),
					},
				},
				PreInstall: []string{"echo pre"},
			},
			expected: &AddonConfig{
				Name:      "my-addon",
				Enabled:   boolPtr(false),
				Namespace: "custom-ns",
				Retries:   int32Ptr(3),
				Delay:     int32Ptr(5), // Delay should still be defaulted
				Sources: AddonSources{
					Chart: &ChartSource{
						Wait:   boolPtr(false),
						Values: []string{}, // Values should be defaulted
					},
					Yaml: nil,
				},
				PreInstall:  []string{"echo pre"},
				PostInstall: []string{}, // PostInstall should be defaulted
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_AddonConfig(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_AddonConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       *AddonConfig
		expectError bool
		errorMsg    string // Substring to check in error message
	}{
		{
			name: "valid config with chart",
			input: &AddonConfig{
				Name:    "metrics-server",
				Enabled: boolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{
						Name:    "metrics-server",
						Repo:    "https://kubernetes-sigs.github.io/metrics-server/",
						Version: "v0.5.0",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with local chart path",
			input: &AddonConfig{
				Name:    "my-local-chart",
				Enabled: boolPtr(true),
				Sources: AddonSources{
					Chart: &ChartSource{
						Path: "./charts/my-local-chart",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with yaml URL",
			input: &AddonConfig{
				Name:    "dashboard",
				Enabled: boolPtr(true),
				Sources: AddonSources{
					Yaml: &YamlSource{
						Path: []string{"https://raw.githubusercontent.com/kubernetes/dashboard/v2.0.0/aio/deploy/recommended.yaml"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with yaml local path",
			input: &AddonConfig{
				Name:    "my-manifests",
				Enabled: boolPtr(true),
				Sources: AddonSources{
					Yaml: &YamlSource{
						Path: []string{"./manifests/my-addon.yaml"},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil config (should not error directly, but pathPrefix would be empty)",
			input:       nil,
			expectError: false, // Validate_AddonConfig handles nil input by returning
		},
		{
			name: "empty name",
			input: &AddonConfig{
				Name: " ", Enabled: boolPtr(true),
				Sources: AddonSources{Chart: &ChartSource{Path: "./c"}}},
			expectError: true,
			errorMsg:    "addon name cannot be empty",
		},
		{
			name: "enabled but no sources",
			input: &AddonConfig{
				Name: "no-source-addon", Enabled: boolPtr(true)},
			expectError: true,
			errorMsg:    "is enabled but has no chart or yaml sources defined",
		},
		// ChartSource Validations
		{
			name: "chart path with name also set",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Path: "./c", Name: "c-name"}}},
			expectError: true,
			errorMsg:    "chart.path",
		},
		{
			name: "chart path with repo also set",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Path: "./c", Repo: "http://example.com"}}},
			expectError: true,
			errorMsg:    "chart.path",
		},
		{
			name: "chart no path and no name",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Repo: "http://example.com"}}},
			expectError: true,
			errorMsg:    "either chart.name (with chart.repo) or chart.path must be specified",
		},
		{
			name: "chart repo without name",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Repo: "http://example.com"}}},
			expectError: true,
			errorMsg:    "chart.name must be specified if chart.repo",
		},
		{
			name: "chart name without repo (and no path)",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name"}}},
			expectError: true,
			errorMsg:    "chart.repo must be specified if chart.name",
		},
		{
			name: "invalid chart repo URL",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "not a url"}}},
			expectError: true,
			errorMsg:    "invalid URL format for chart repo",
		},
		{
			name: "whitespace chart version",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: " "}}},
			expectError: true,
			errorMsg:    "chart version cannot be only whitespace",
		},
		{
			name: "invalid chart version format",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "1.2.3.4.5"}}},
			expectError: true,
			errorMsg:    "is not a valid format",
		},
		{
			name: "valid chart version: latest",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "latest"}}},
			expectError: false,
		},
		{
			name: "valid chart version: stable",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "stable"}}},
			expectError: false,
		},
		{
			name: "valid chart version: v1.2.3",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "v1.2.3"}}},
			expectError: false,
		},
		{
			name: "valid chart version: 1.0",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "1.0"}}},
			expectError: false,
		},
		{
			name: "valid chart version: 2",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://example.com", Version: "2"}}},
			expectError: false,
		},
		{
			name: "invalid chart values format",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Chart: &ChartSource{Name: "c-name", Repo: "http://e.com", Values: []string{"keyonly"}}}},
			expectError: true,
			errorMsg:    "expected key=value",
		},
		// YamlSource Validations
		{
			name: "yaml source with no paths",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Yaml: &YamlSource{Path: []string{}}}},
			expectError: true,
			errorMsg:    "must contain at least one YAML path or URL",
		},
		{
			name: "yaml source with empty path string",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Sources: AddonSources{
				Yaml: &YamlSource{Path: []string{" "}}}},
			expectError: true,
			errorMsg:    "path/URL cannot be empty",
		},
		// Retries and Delay
		{
			name: "negative retries",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Retries: int32Ptr(-1),
				Sources: AddonSources{Chart: &ChartSource{Path: "./c"}}},
			expectError: true,
			errorMsg:    "retries: cannot be negative",
		},
		{
			name: "negative delay",
			input: &AddonConfig{Name: "a", Enabled: boolPtr(true), Delay: int32Ptr(-1),
				Sources: AddonSources{Chart: &ChartSource{Path: "./c"}}},
			expectError: true,
			errorMsg:    "delay: cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			// Defaulting is usually called before validation in real scenarios
			SetDefaults_AddonConfig(tt.input)
			Validate_AddonConfig(tt.input, verrs, "addon")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "expected error but got none for test: %s", tt.name)
				if tt.errorMsg != "" {
					found := false
					errStrings := ""
					if verrs.HasErrors() {
						errStrings = verrs.Error()
					}
					for _, errStr := range strings.Split(errStrings, "\n") { // ValidationErrors.Error() returns errors separated by \n
						if strings.Contains(errStr, tt.errorMsg) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected error message to contain '%s', but got: %s for test: %s", tt.errorMsg, errStrings, tt.name)
				}
			} else {
				assert.False(t, verrs.HasErrors(), "expected no error, but got: %s for test: %s", verrs.Error(), tt.name)
			}
		})
	}
}

// boolPtr and int32Ptr are already defined in addon_types.go
// If they were moved to a common util, tests for them would go there.

func TestHelperBoolPtr(t *testing.T) {
	bTrue := true
	bFalse := false
	assert.Equal(t, &bTrue, boolPtr(true))
	assert.Equal(t, &bFalse, boolPtr(false))
	assert.True(t, *boolPtr(true))
	assert.False(t, *boolPtr(false))
}

func TestHelperInt32Ptr(t *testing.T) {
	var val1 int32 = 0
	var val2 int32 = 100
	var val3 int32 = -50

	assert.Equal(t, &val1, int32Ptr(0))
	assert.Equal(t, &val2, int32Ptr(100))
	assert.Equal(t, &val3, int32Ptr(-50))

	assert.Equal(t, int32(0), *int32Ptr(0))
	assert.Equal(t, int32(100), *int32Ptr(100))
	assert.Equal(t, int32(-50), *int32Ptr(-50))
}
