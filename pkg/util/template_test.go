package util

import (
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	type simpleData struct {
		Version string
		Arch    string
	}
	type complexData struct {
		Project struct {
			Name    string
			Version string
		}
		OS   string
		Arch string
	}

	tests := []struct {
		name          string
		tmplStr       string
		data          interface{}
		expectedStr   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "simple_success",
			tmplStr:     "file-{{.Version}}-{{.Arch}}.zip",
			data:        simpleData{Version: "v1.0", Arch: "amd64"},
			expectedStr: "file-v1.0-amd64.zip",
			expectError: false,
		},
		{
			name:          "malformed_template_parse_error",
			tmplStr:       "file-{{.Version", // Missing closing braces
			data:          simpleData{Version: "v1.0", Arch: "amd64"},
			expectError:   true,
			errorContains: "template: :1: unclosed action",
		},
		{
			name:          "execution_error_missing_key_is_error_by_default",
			tmplStr:       "Name: {{.Name}}, Value: {{.MissingValue}}",
			data:          struct{ Name string }{Name: "Test"},
			expectedStr:   "", // Output is not relevant when an error is expected
			expectError:   true,
			errorContains: "can't evaluate field MissingValue",
		},
		{
			name:        "empty_template_string",
			tmplStr:     "",
			data:        simpleData{Version: "v1.0", Arch: "amd64"},
			expectedStr: "",
			expectError: false,
		},
		{
			name:        "nil_data_renders_no_value",
			tmplStr:     "Version: {{.Version}}",
			data:        nil,
			expectedStr: "Version: <no value>", // Default rendering for missing field on nil
			expectError: false,
		},
		{
			name:        "empty_struct_data",
			tmplStr:     "Version: {{.Version}}, Arch: {{.Arch}}",
			data:        simpleData{}, // Version and Arch will be empty strings
			expectedStr: "Version: , Arch: ",
			expectError: false,
		},
		{
			name: "complex_data_structure",
			tmplStr: "Project: {{.Project.Name}}-{{.Project.Version}} for OS {{.OS}}/{{.Arch}}",
			data: complexData{
				Project: struct {Name string; Version string}{Name: "Kubexm", Version: "v1.alpha"},
				OS:      "linux",
				Arch:    "arm64",
			},
			expectedStr: "Project: Kubexm-v1.alpha for OS linux/arm64",
			expectError: false,
		},
		{
			name: "template_with_no_substitutions",
			tmplStr: "This is a static string.",
			data: nil,
			expectedStr: "This is a static string.",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, err := RenderTemplate(tt.tmplStr, tt.data)

			if tt.expectError {
				if err == nil {
					t.Errorf("RenderTemplate() expected an error, but got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("RenderTemplate() error = %q, expected to contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("RenderTemplate() returned an unexpected error: %v", err)
				}
				if gotStr != tt.expectedStr {
					t.Errorf("RenderTemplate() got %q, want %q", gotStr, tt.expectedStr)
				}
			}
		})
	}
}
