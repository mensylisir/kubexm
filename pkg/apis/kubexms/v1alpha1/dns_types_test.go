package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_ExternalZone(t *testing.T) {
	tests := []struct {
		name     string
		input    *ExternalZone
		expected *ExternalZone
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty input",
			input: &ExternalZone{},
			expected: &ExternalZone{
				Zones:       []string{},
				Nameservers: []string{},
				Cache:       common.DefaultExternalZoneCacheSeconds,
				Rewrite:     []RewriteRule{},
			},
		},
		{
			name: "cache already set",
			input: &ExternalZone{Cache: 600},
			expected: &ExternalZone{
				Zones:       []string{},
				Nameservers: []string{},
				Cache:       600,
				Rewrite:     []RewriteRule{},
			},
		},
		{
			name: "all fields set",
			input: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"1.1.1.1"},
				Cache:       120,
				Rewrite:     []RewriteRule{{FromPattern: "a", ToTemplate: "b"}},
			},
			expected: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"1.1.1.1"},
				Cache:       120,
				Rewrite:     []RewriteRule{{FromPattern: "a", ToTemplate: "b"}},
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
	defaultUpstreams := []string{common.DefaultCoreDNSUpstreamGoogle, common.DefaultCoreDNSUpstreamCloudflare}
	tests := []struct {
		name     string
		input    *DNS
		expected *DNS
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty DNS config",
			input: &DNS{},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: defaultUpstreams,
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "coredns upstream specified",
			input: &DNS{
				CoreDNS: CoreDNS{UpstreamDNSServers: []string{"9.9.9.9"}},
			},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"9.9.9.9"},
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "coredns with external zone",
			input: &DNS{
				CoreDNS: CoreDNS{ExternalZones: []ExternalZone{{}}}, // Input an empty ExternalZone to test its defaulting
			},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: defaultUpstreams,
					ExternalZones: []ExternalZone{ // Expect the ExternalZone to be defaulted
						{Zones: []string{}, Nameservers: []string{}, Cache: common.DefaultExternalZoneCacheSeconds, Rewrite: []RewriteRule{}},
					},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{},
				},
			},
		},
		{
			name: "nodelocaldns with external zone",
			input: &DNS{
				NodeLocalDNS: NodeLocalDNS{ExternalZones: []ExternalZone{{Cache: 0}}}, // Cache 0 to test defaulting of Cache
			},
			expected: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: defaultUpstreams,
					ExternalZones:      []ExternalZone{},
				},
				NodeLocalDNS: NodeLocalDNS{
					ExternalZones: []ExternalZone{ // Expect the ExternalZone to be defaulted
						{Zones: []string{}, Nameservers: []string{}, Cache: common.DefaultExternalZoneCacheSeconds, Rewrite: []RewriteRule{}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_DNS(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_ExternalZone(t *testing.T) {
	tests := []struct {
		name        string
		input       *ExternalZone
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid external zone",
			input: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"8.8.8.8"},
				Cache:       300, // Defaulted by SetDefaults_ExternalZone if 0
			},
			expectError: false,
		},
		{
			name:        "valid zone with ip nameserver",
			input:       &ExternalZone{Zones: []string{"test.com"}, Nameservers: []string{"1.2.3.4"}, Cache: 300},
			expectError: false,
		},
		{
			name:        "valid zone with hostname nameserver",
			input:       &ExternalZone{Zones: []string{"test.com"}, Nameservers: []string{"ns1.example.com"}, Cache: 300},
			expectError: false,
		},
		{
			name:        "no zones",
			input:       &ExternalZone{Nameservers: []string{"8.8.8.8"}, Cache: 300},
			expectError: true,
			errorMsg:    ".zones: must contain at least one zone name",
		},
		{
			name:        "empty zone string",
			input:       &ExternalZone{Zones: []string{""}, Nameservers: []string{"8.8.8.8"}, Cache: 300},
			expectError: true,
			errorMsg:    ".zones[0]: zone name cannot be empty",
		},
		{
			name:        "invalid zone name format",
			input:       &ExternalZone{Zones: []string{"_example.com"}, Nameservers: []string{"8.8.8.8"}, Cache: 300},
			expectError: true,
			errorMsg:    ".zones[0]: invalid domain name format '_example.com'",
		},
		{
			name:        "no nameservers",
			input:       &ExternalZone{Zones: []string{"example.com"}, Cache: 300},
			expectError: true,
			errorMsg:    ".nameservers: must contain at least one nameserver",
		},
		{
			name:        "empty nameserver string",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{""}, Cache: 300},
			expectError: true,
			errorMsg:    ".nameservers[0]: nameserver address cannot be empty",
		},
		{
			name:        "invalid nameserver format",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"not-an-ip-or-domain"}, Cache: 300},
			expectError: true,
			errorMsg:    ".nameservers[0]: invalid nameserver address format 'not-an-ip-or-domain'",
		},
		{
			name:        "negative cache",
			input:       &ExternalZone{Zones: []string{"example.com"}, Nameservers: []string{"8.8.8.8"}, Cache: -1},
			expectError: true,
			errorMsg:    ".cache: cannot be negative, got -1",
		},
		{
			name: "rewrite rule with empty FromPattern",
			input: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"8.8.8.8"},
				Cache:       300,
				Rewrite:     []RewriteRule{{ToTemplate: "b"}},
			},
			expectError: true,
			errorMsg:    ".rewrite[0].fromPattern: cannot be empty",
		},
		{
			name: "rewrite rule with empty ToTemplate",
			input: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"8.8.8.8"},
				Cache:       300,
				Rewrite:     []RewriteRule{{FromPattern: "a"}},
			},
			expectError: true,
			errorMsg:    ".rewrite[0].toTemplate: cannot be empty",
		},
		{
			name: "valid rewrite rule",
			input: &ExternalZone{
				Zones:       []string{"example.com"},
				Nameservers: []string{"8.8.8.8"},
				Cache:       300,
				Rewrite:     []RewriteRule{{FromPattern: "(.*).example.com", ToTemplate: "{1}.internal"}},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults to ensure Cache is set if not specified, as validation depends on it or its default.
			if tt.input != nil && tt.input.Cache == 0 {
				SetDefaults_ExternalZone(tt.input)
			}
			verrs := &validation.ValidationErrors{}
			Validate_ExternalZone(tt.input, verrs, "testPrefix")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v", tt.name, tt.input)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v", tt.name, verrs.Error(), tt.input)
			}
		})
	}
}

func TestValidate_DNS(t *testing.T) {
	tests := []struct {
		name        string
		input       *DNS
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid empty DNS (after defaults)",
			input:       &DNS{},
			expectError: false,
		},
		{
			name: "valid coredns with upstream and external zone",
			input: &DNS{
				CoreDNS: CoreDNS{
					UpstreamDNSServers: []string{"1.1.1.1"},
					ExternalZones: []ExternalZone{
						{Zones: []string{"my.zone"}, Nameservers: []string{"10.0.0.1"}, Cache: 300},
					},
				},
			},
			expectError: false,
		},
		{
			name: "coredns empty upstream server",
			input: &DNS{
				CoreDNS: CoreDNS{UpstreamDNSServers: []string{""}},
			},
			expectError: true,
			errorMsg:    ".coredns.upstreamDNSServers[0]: server address cannot be empty",
		},
		{
			name: "coredns invalid upstream server format",
			input: &DNS{
				CoreDNS: CoreDNS{UpstreamDNSServers: []string{"not-valid"}},
			},
			expectError: true,
			errorMsg:    ".coredns.upstreamDNSServers[0]: invalid server address format 'not-valid'",
		},
		{
			name: "coredns invalid external zone (no nameservers)",
			input: &DNS{
				CoreDNS: CoreDNS{ExternalZones: []ExternalZone{{Zones: []string{"bad.zone"}, Cache: 300}}},
			},
			expectError: true,
			errorMsg:    ".coredns.externalZones[0].nameservers: must contain at least one nameserver",
		},
		{
			name: "nodelocaldns invalid external zone (no zones)",
			input: &DNS{
				NodeLocalDNS: NodeLocalDNS{ExternalZones: []ExternalZone{{Nameservers: []string{"1.2.3.4"}, Cache: 300}}},
			},
			expectError: true,
			errorMsg:    ".nodelocaldns.externalZones[0].zones: must contain at least one zone name",
		},
		{
			name: "dnsEtcHosts is only whitespace",
			input: &DNS{
				DNSEtcHosts: "   ",
			},
			expectError: true,
			errorMsg:    ".dnsEtcHosts: cannot be only whitespace if specified",
		},
		{
			name: "nodeEtcHosts is only whitespace",
			input: &DNS{
				NodeEtcHosts: "   ",
			},
			expectError: true,
			errorMsg:    ".nodeEtcHosts: cannot be only whitespace if specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputToTest := tt.input
			// Apply defaults to simulate the state before validation
			if inputToTest != nil {
				SetDefaults_DNS(inputToTest) // This will also default nested ExternalZones
			}

			verrs := &validation.ValidationErrors{}
			Validate_DNS(inputToTest, verrs, "spec.dns")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v, Defaulted: %+v", tt.name, tt.input, inputToTest)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v, Defaulted: %+v", tt.name, verrs.Error(), tt.input, inputToTest)
			}
		})
	}
}
