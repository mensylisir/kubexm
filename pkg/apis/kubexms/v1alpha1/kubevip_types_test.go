package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetDefaults_KubeVIPConfig tests the SetDefaults_KubeVIPConfig function.
func TestSetDefaults_KubeVIPConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *KubeVIPConfig
		expected *KubeVIPConfig
	}{
		{
			name:     "nil config",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &KubeVIPConfig{},
			expected: &KubeVIPConfig{
				Mode:                 stringPtr(KubeVIPModeARP),
				EnableControlPlaneLB: boolPtr(true),
				EnableServicesLB:     boolPtr(false),
				ExtraArgs:            []string{},
			},
		},
		{
			name:  "mode BGP, BGPConfig nil",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP)},
			expected: &KubeVIPConfig{
				Mode:                 stringPtr(KubeVIPModeBGP),
				EnableControlPlaneLB: boolPtr(true),
				EnableServicesLB:     boolPtr(false),
				ExtraArgs:            []string{},
				BGPConfig:            &KubeVIPBGPConfig{}, // Should be initialized
			},
		},
		{
			name:  "mode BGP, BGPConfig present",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1"}},
			expected: &KubeVIPConfig{
				Mode:                 stringPtr(KubeVIPModeBGP),
				EnableControlPlaneLB: boolPtr(true),
				EnableServicesLB:     boolPtr(false),
				ExtraArgs:            []string{},
				BGPConfig:            &KubeVIPBGPConfig{RouterID: "1.1.1.1"}, // Not overridden
			},
		},
		{
			name:  "custom settings",
			input: &KubeVIPConfig{EnableControlPlaneLB: boolPtr(false), EnableServicesLB: boolPtr(true), Image: stringPtr("myimage")},
			expected: &KubeVIPConfig{
				Mode:                 stringPtr(KubeVIPModeARP),
				EnableControlPlaneLB: boolPtr(false),
				EnableServicesLB:     boolPtr(true),
				Image:                stringPtr("myimage"),
				ExtraArgs:            []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_KubeVIPConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

// TestValidate_KubeVIPConfig tests the Validate_KubeVIPConfig function.
func TestValidate_KubeVIPConfig(t *testing.T) {
	validVIP := "192.168.1.200"
	validInterface := "eth0"

	tests := []struct {
		name        string
		input       *KubeVIPConfig
		expectErr   bool
		errContains []string
	}{
		{
			name:        "valid ARP mode",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr(validVIP), Interface: stringPtr(validInterface)},
			expectErr:   false,
		},
		{
			name: "valid BGP mode",
			input: &KubeVIPConfig{
				Mode: stringPtr(KubeVIPModeBGP),
				VIP:  stringPtr(validVIP),
				BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerAddress: "10.0.0.1", PeerASN: 65001, ASN: 65000, SourceAddress: "10.0.0.2"},
			},
			expectErr: false,
		},
		{
			name:        "nil config",
			input:       nil, // Should not error, validator returns early
			expectErr:   false,
		},
		{
			name:        "invalid mode",
			input:       &KubeVIPConfig{Mode: stringPtr("STP"), VIP: stringPtr(validVIP)},
			expectErr:   true,
			errContains: []string{".mode: invalid mode 'STP'"},
		},
		{
			name:        "missing VIP",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), Interface: stringPtr(validInterface)},
			expectErr:   true,
			errContains: []string{".vip: virtual IP address must be specified"},
		},
		{
			name:        "empty VIP",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr(" "), Interface: stringPtr(validInterface)},
			expectErr:   true,
			errContains: []string{".vip: virtual IP address must be specified"},
		},
		{
			name:        "invalid VIP format",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr("not-an-ip"), Interface: stringPtr(validInterface)},
			expectErr:   true,
			errContains: []string{".vip: invalid IP address format 'not-an-ip'"},
		},
		{
			name:        "ARP mode missing interface",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr(validVIP)},
			expectErr:   true,
			errContains: []string{".interface: network interface must be specified for ARP mode"},
		},
		{
			name:        "ARP mode empty interface",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr(validVIP), Interface: stringPtr(" ")},
			expectErr:   true,
			errContains: []string{".interface: network interface must be specified for ARP mode"},
		},
		{
			name:        "empty image if specified",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeARP), VIP: stringPtr(validVIP), Interface: stringPtr(validInterface), Image: stringPtr(" ")},
			expectErr:   true,
			errContains: []string{".image: cannot be empty if specified"},
		},
		{
			name:        "BGP mode missing BGPConfig (after defaults)",
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP)}, // BGPConfig would be initialized by defaults
			expectErr:   true, // Validation should still fail due to missing required BGP fields
			errContains: []string{".bgpConfig.routerID: router ID must be specified"}, // Example of a missing required field in BGPConfig
		},
		{
			name: "BGP mode missing RouterID",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{PeerAddress: "10.0.0.1", PeerASN: 65001, ASN: 65000}},
			expectErr:   true,
			errContains: []string{".bgpConfig.routerID: router ID must be specified for BGP mode"},
		},
		{
			name: "BGP mode missing ASN",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerAddress: "10.0.0.1", PeerASN: 65001}},
			expectErr:   true,
			errContains: []string{".bgpConfig.asn: local ASN must be specified for BGP mode"},
		},
		{
			name: "BGP mode missing PeerASN",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerAddress: "10.0.0.1", ASN: 65000}},
			expectErr:   true,
			errContains: []string{".bgpConfig.peerASN: peer ASN must be specified for BGP mode"},
		},
		{
			name: "BGP mode missing PeerAddress",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerASN: 65001, ASN: 65000}},
			expectErr:   true,
			errContains: []string{".bgpConfig.peerAddress: peer address must be specified for BGP mode"},
		},
		{
			name: "BGP mode invalid PeerAddress",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerAddress: "invalid-ip", PeerASN: 65001, ASN: 65000}},
			expectErr:   true,
			errContains: []string{".bgpConfig.peerAddress: invalid IP 'invalid-ip'"},
		},
		{
			name: "BGP mode invalid SourceAddress",
			input: &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1", PeerAddress: "10.0.0.1", PeerASN: 65001, ASN: 65000, SourceAddress: "not-an-ip"}},
			expectErr:   true,
			errContains: []string{".bgpConfig.sourceAddress: invalid IP address format 'not-an-ip'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != nil { // Avoid panic on nil input
				SetDefaults_KubeVIPConfig(tt.input)
			}

			verrs := &ValidationErrors{}
			Validate_KubeVIPConfig(tt.input, verrs, "spec.kubevip")

			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
