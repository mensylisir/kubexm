package validation

import (
	"testing"
)

func TestValidationErrors(t *testing.T) {
	verrs := &ValidationErrors{}
	if verrs.HasErrors() {
		t.Error("New ValidationErrors should not have errors")
	}
	if verrs.Error() != "" { // Empty ValidationErrors should produce an empty string
		t.Errorf("Expected empty error string, got '%s'", verrs.Error())
	}

	verrs.Add("field1", "is required")
	if !verrs.HasErrors() {
		t.Error("ValidationErrors should have errors after Add")
	}
	expectedError1 := "field1: is required"
	if verrs.Error() != expectedError1 {
		t.Errorf("Expected error string '%s', got '%s'", expectedError1, verrs.Error())
	}

	verrs.Add("field2.subfield", "must be positive")
	expectedError2 := "field1: is required\nfield2.subfield: must be positive"
	if verrs.Error() != expectedError2 {
		t.Errorf("Expected error string '%s', got '%s'", expectedError2, verrs.Error())
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid http", "http://example.com", true},
		{"valid https", "https://example.com/path?query=value", true},
		{"valid ftp", "ftp://user:pass@example.com:21/path", true},
		{"missing scheme", "example.com", false},
		{"invalid scheme", "htp://example.com", false},
		{"empty string", "", false},
		{"just scheme", "http://", false}, // url.ParseRequestURI considers "http://" valid
		{"scheme with space", "http:// example.com", false},
		{"uri with fragment", "http://example.com#fragment", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidURL(tt.url); got != tt.want {
				t.Errorf("IsValidURL(%q) = %v, want %v", tt.url, got, tt.want)
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
		{"latest", "latest", true},
		{"stable", "stable", true},
		{"simple version", "1.0.0", true},
		{"version with v", "v1.2.3", true},
		{"short version", "1.2", true},
		{"short version with v", "v1.2", true},
		{"single number version", "1", true},
		{"single number version with v", "v2", true},
		{"invalid chars", "1.0.0-alpha", false}, // Regex does not match pre-releases
		{"too many parts", "1.2.3.4", false},
		{"leading dot", ".1.2.3", false},
		{"trailing dot", "1.2.3.", false},
		{"empty string", "", false},
		{"non-numeric", "abc", false},
		{"version with spaces", "v 1.2.3", false},
		{"valid patch", "1.2.33", true},
		{"valid minor", "1.22", true},
		{"valid major", "11", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidChartVersion(tt.version); got != tt.want {
				t.Errorf("IsValidChartVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
