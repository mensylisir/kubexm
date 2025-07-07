package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/common" // Import common
	"github.com/mensylisir/kubexm/pkg/util"   // Import util
	"github.com/mensylisir/kubexm/pkg/util/validation"
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
				Mode:                 util.StrPtr(KubeVIPModeARP),
				EnableControlPlaneLB: util.BoolPtr(true),
				EnableServicesLB:     util.BoolPtr(false),
				Image:                util.StrPtr(common.DefaultKubeVIPImage), // Added default image
				ExtraArgs:            []string{},
			},
		},
		{
			name:  "mode BGP, BGPConfig nil",
			input: &KubeVIPConfig{Mode: util.StrPtr(KubeVIPModeBGP)},
			expected: &KubeVIPConfig{
				Mode:                 util.StrPtr(KubeVIPModeBGP),
				EnableControlPlaneLB: util.BoolPtr(true),
				EnableServicesLB:     util.BoolPtr(false),
				Image:                util.StrPtr(common.DefaultKubeVIPImage), // Added default image
				ExtraArgs:            []string{},
				BGPConfig:            &KubeVIPBGPConfig{},
			},
		},
		{
			name:  "mode BGP, BGPConfig present",
			input: &KubeVIPConfig{Mode: util.StrPtr(KubeVIPModeBGP), BGPConfig: &KubeVIPBGPConfig{RouterID: "1.1.1.1"}},
			expected: &KubeVIPConfig{
				Mode:                 util.StrPtr(KubeVIPModeBGP),
				EnableControlPlaneLB: util.BoolPtr(true),
				EnableServicesLB:     util.BoolPtr(false),
				Image:                util.StrPtr(common.DefaultKubeVIPImage), // Added default image
				ExtraArgs:            []string{},
				BGPConfig:            &KubeVIPBGPConfig{RouterID: "1.1.1.1"},
			},
		},
		{
			name:  "custom settings",
			input: &KubeVIPConfig{EnableControlPlaneLB: util.BoolPtr(false), EnableServicesLB: util.BoolPtr(true), Image: util.StrPtr("myimage")},
			expected: &KubeVIPConfig{
				Mode:                 util.StrPtr(KubeVIPModeARP),
				EnableControlPlaneLB: util.BoolPtr(false),
				EnableServicesLB:     util.BoolPtr(true),
				Image:                util.StrPtr("myimage"), // User specified image preserved
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
			input:       nil,
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
			input:       &KubeVIPConfig{Mode: stringPtr(KubeVIPModeBGP), VIP: stringPtr(validVIP)},
			expectErr:   true,
			errContains: []string{".bgpConfig.routerID: router ID must be specified"},
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
			if tt.input != nil {
				SetDefaults_KubeVIPConfig(tt.input)
			}

			verrs := &validation.ValidationErrors{}
			Validate_KubeVIPConfig(tt.input, verrs, "spec.kubevip")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
