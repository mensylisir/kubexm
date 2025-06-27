package v1alpha1

import (
	// "fmt" // Potentially needed for more complex validation error prefixing if used directly
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Assuming ValidationErrors is defined in cluster_types.go and available.
// Assuming boolPtr, int32Ptr etc are in zz_helpers.go and available.

func TestSetDefaults_ExternalZone(t *testing.T) {
	tests := []struct {
		name     string
		input    *ExternalZone
		expected *ExternalZone
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty input",
			input: &ExternalZone{},
			expected: &ExternalZone{
				Zones:       []string{},
				Nameservers: []string{},
				Cache:       300,
				Rewrite:     []string{},
			},
		},
		{
			name:  "cache already set",
			input: &ExternalZone{Cache: 600},
			expected: &ExternalZone{
				Zones:       []string{},
				Nameservers: []string{},
				Cache:       600, // Should not be overridden
				Rewrite:     []string{},
			},
		},
		{
			name:  "all fields set",
			input: &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"1.1.1.1"}, Cache: 150, Rewrite: []string{"rule1"}},
			expected: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"1.1.1.1"},
				Cache:       150,
				Rewrite:     []string{"rule1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ExternalZone(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestSetDefaults_DNS(t *testing.T) {
	tests := []struct {
		name     string
		input    *DNS
		expected *DNS
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty DNS config",
			input: &DNS{},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"8.8.8.8", "1.1.1.1"},
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "coredns upstream specified",
			input: &DNS{CoreDNS: CoreDNS{UpstreamDNSServers: []string{"9.9.9.9"}}},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"9.9.9.9"}, // Not overridden
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "coredns with external zone",
			input: &DNS{CoreDNS: CoreDNS{ExternalZones: []ExternalZone{{}}}},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"8.8.8.8", "1.1.1.1"},
					ExternalZones: []ExternalZone{
						{Zones: []string{}, Nameservers: []string{}, Cache: 300, Rewrite: []string{}},
					},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "nodelocaldns with external zone",
			input: &DNS{NodeLocalDNS: NodeLocalDNS{ExternalZones: []ExternalZone{{Cache: 500}}}},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"8.8.8.8", "1.1.1.1"},
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{
						{Zones: []string{}, Nameservers: []string{}, Cache: 500, Rewrite: []string{}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_DNS(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				// Use assert for better diff output
				assert.Equal(t, tt.expected, tt.input, "SetDefaults_DNS() mismatch")
			}
		})
	}
}

func TestValidate_ExternalZone(t *testing.T) {
	tests := []struct {
		name        string
		input       *ExternalZone
		expectErr   bool
		errContains []string
	}{
		{
			name:        "valid external zone",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"1.2.3.4"}, Cache: 300},
			expectErr:   false,
		},
		{
			name:        "no zones",
			input:       &ExternalZone{Nameservers: []string{"1.2.3.4"}},
			expectErr:   true,
			errContains: []string{".zones: must contain at least one zone name"},
		},
		{
			name:        "empty zone string",
			input:       &ExternalZone{Zones: []string{" "}, Nameservers: []string{"1.2.3.4"}},
			expectErr:   true,
			errContains: []string{".zones[0]: zone name cannot be empty"},
		},
		{
			name:        "no nameservers",
			input:       &ExternalZone{Zones: []string{"example.com"}},
			expectErr:   true,
			errContains: []string{".nameservers: must contain at least one nameserver"},
		},
		{
			name:        "empty nameserver string",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{" "}},
			expectErr:   true,
			errContains: []string{".nameservers[0]: nameserver address cannot be empty"},
		},
		{
			name:        "negative cache",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"1.2.3.4"}, Cache: -100},
			expectErr:   true,
			errContains: []string{".cache: cannot be negative"},
		},
		{
			name:        "empty rewrite rule if rewrite array is present",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"1.2.3.4"}, Rewrite: []string{" "}},
			expectErr:   true,
			errContains: []string{".rewrite[0]: rewrite rule cannot be empty"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ExternalZone(tt.input) // Apply defaults first
			verrs := &ValidationErrors{}
			Validate_ExternalZone(tt.input, verrs, "test.zone")
			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected error for %s but got none", tt.name)
				for _, c := range tt.errContains {
					assert.Contains(t, verrs.Error(), c, "Error for %s did not contain %s", tt.name, c)
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no error for %s but got: %s", tt.name, verrs.Error())
			}
		})
	}
}

func TestValidate_DNS(t *testing.T) {
	tests := []struct {
		name        string
		input       *DNS
		expectErr   bool
		errContains []string
	}{
		{
			name:        "valid empty DNS (after defaults)",
			input:       &DNS{}, // Will be filled by SetDefaults_DNS
			expectErr:   false,
		},
		{
			name: "valid coredns with upstream and external zone",
			input: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"8.8.8.8"},
					ExternalZones: []ExternalZone{
						{Zones: []string{"corp.local"}, Nameservers: []string{"10.0.0.1"}},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "coredns empty upstream server",
			input: &DNS{CoreDNS: CoreDNS{UpstreamDNSServers: []string{" "}}},
			expectErr:   true,
			errContains: []string{".coredns.upstreamDNSServers[0]: server address cannot be empty"},
		},
		{
			name: "coredns invalid external zone (no nameservers)",
			input: &DNS{CoreDNS: CoreDNS{ExternalZones: []ExternalZone{{Zones: []string{"fail.com"}}}}},
			expectErr:   true,
			errContains: []string{".coredns.externalZones[0].nameservers: must contain at least one nameserver"},
		},
		{
			name: "nodelocaldns invalid external zone (no zones)",
			input: &DNS{NodeLocalDNS: NodeLocalDNS{ExternalZones: []ExternalZone{{Nameservers: []string{"1.1.1.1"}}}}},
			expectErr:   true,
			errContains: []string{".nodelocaldns.externalZones[0].zones: must contain at least one zone name"},
		},
		{
			name:        "dnsEtcHosts is only whitespace",
			input:       &DNS{DNSEtcHosts: "   "},
			expectErr:   true,
			errContains: []string{".dnsEtcHosts: cannot be only whitespace if specified"},
		},
		{
			name:        "nodeEtcHosts is only whitespace",
			input:       &DNS{NodeEtcHosts: "   "},
			expectErr:   true,
			errContains: []string{".nodeEtcHosts: cannot be only whitespace if specified"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_DNS(tt.input) // Apply defaults first
			verrs := &ValidationErrors{}
			Validate_DNS(tt.input, verrs, "spec.dns")

			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected error for %s but got none", tt.name)
				for _, c := range tt.errContains {
					assert.Contains(t, verrs.Error(), c, "Error for %s did not contain %s", tt.name, c)
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no error for %s but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
