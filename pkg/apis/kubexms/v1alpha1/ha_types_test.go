package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	cfg := &HighAvailabilityConfig{}
	SetDefaults_HighAvailabilityConfig(cfg)

	t.Run("default with external ManagedKeepalivedHAProxy type", func(t *testing.T) {
		haEnabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedHAProxy",
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		assert.NotNil(t, cfgExt.External, "External config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived, "External.Keepalived config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived.AuthType, "External.Keepalived.AuthType should have a default")
		assert.Equal(t, "PASS", *cfgExt.External.Keepalived.AuthType, "External.Keepalived.AuthType default mismatch")
		assert.NotNil(t, cfgExt.External.HAProxy, "External.HAProxy config should be initialized")
		assert.NotNil(t, cfgExt.External.HAProxy.Mode, "External.HAProxy.Mode should have a default")
		assert.Equal(t, "tcp", *cfgExt.External.HAProxy.Mode, "External.HAProxy.Mode default mismatch")
	})

	t.Run("default with external ManagedKeepalivedNginxLB type", func(t *testing.T) {
		haEnabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedNginxLB",
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		assert.NotNil(t, cfgExt.External, "External config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived, "External.Keepalived config should be initialized")
		assert.NotNil(t, cfgExt.External.NginxLB, "External.NginxLB config should be initialized")
		assert.NotNil(t, cfgExt.External.NginxLB.Mode, "External.NginxLB.Mode should have a default")
		assert.Equal(t, "tcp", *cfgExt.External.NginxLB.Mode, "External.NginxLB.Mode default mismatch")
	})

	t.Run("default with internal KubeVIP type", func(t *testing.T) {
		haEnabled := true
		cfgInt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			Internal: &InternalLoadBalancerConfig{
				Type: common.InternalLBTypeKubeVIP, // Use constant
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgInt)
		assert.NotNil(t, cfgInt.Internal, "Internal config should be initialized")
		if assert.NotNil(t, cfgInt.Internal.KubeVIP, "Internal.KubeVIP config should be initialized for Type KubeVIP") {
			assert.NotNil(t, cfgInt.Internal.KubeVIP.Mode, "KubeVIP.Mode should have a default")
			assert.Equal(t, KubeVIPModeARP, *cfgInt.Internal.KubeVIP.Mode, "KubeVIP.Mode default mismatch")
		}
	})
}

func TestValidate_HighAvailabilityConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *HighAvailabilityConfig
		wantErrMsg string
		expectErr  bool
	}{
		{
		name: "valid external ManagedKeepalivedHAProxy with endpoint IP (VIP removed, CPE moved)",
		cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedHAProxy", // This type implies managed, so LoadBalancerHostGroupName is needed
				LoadBalancerHostGroupName: stringPtr("lb-group"), // Added
				Keepalived: &KeepalivedConfig{
					VRID:      intPtr(1),
					Priority:  intPtr(101),
					Interface: stringPtr("eth0"),
					AuthPass:  stringPtr("secret"),
				},
				HAProxy: &HAProxyConfig{
					FrontendPort:   intPtr(6443),
					// Ensure Port is explicitly set if Address doesn't solely define it for validation logic
					BackendServers: []HAProxyBackendServer{{Name: "cp1", Address: "192.168.0.10:6443", Port: 6443}},
				},
			}},
		expectErr: false,
		},
		{
		name: "valid external UserProvided (CPE validation is at Cluster level)",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided"}},
			expectErr: false,
		},
		{
			name:      "valid HA disabled",
			cfg:       &HighAvailabilityConfig{Enabled: boolPtr(false)},
			expectErr: false,
		},
		{
			name:      "valid empty config (HA disabled by default)",
			cfg:       &HighAvailabilityConfig{},
			expectErr: false,
		},
		{
			name: "invalid external LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "unknownExternalLB"}},
			wantErrMsg: "spec.highAvailability.external.type: unknown external LB type 'unknownExternalLB'",
			expectErr:  true,
		},
		{
			name: "keepalived_config_present_external_type_mismatch",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Keepalived: &KeepalivedConfig{VRID: intPtr(1)}}},
			wantErrMsg: "spec.highAvailability.external.keepalived: should not be set for 'UserProvided' external LB type", // Corrected path
			expectErr:  true,
		},
		{
			name: "valid internal KubeVIP",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", KubeVIP: &KubeVIPConfig{
					VIP:       stringPtr("192.168.1.100"),
					Interface: stringPtr("eth0"),
				}}},
			expectErr: false,
		},
		{
			name: "invalid internal LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "unknownInternalLB"}},
			wantErrMsg: "spec.highAvailability.internal.type: unknown internal LB type 'unknownInternalLB'",
			expectErr:  true,
		},
		{
			name: "KubeVIP internal LB missing KubeVIP section",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: nil}}, // Uses constant
			wantErrMsg: ".internal.kubevip.vip: virtual IP address must be specified",
			expectErr:  true,
		},
		{
			name: "External and Internal LB simultaneously enabled",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: common.ExternalLBTypeExternal},
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: &KubeVIPConfig{VIP: stringPtr("1.1.1.1"), Interface: stringPtr("eth0")}}},
			wantErrMsg: "external load balancer and internal load balancer cannot be enabled simultaneously",
			expectErr:  true,
		},
		{
			name: "Managed External LB (kubexm-kh) missing LoadBalancerHostGroupName",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type: common.ExternalLBTypeKubexmKH,
					Keepalived: &KeepalivedConfig{VRID: intPtr(1), Priority: intPtr(100), Interface: stringPtr("eth0"), AuthPass: stringPtr("pass")},
					HAProxy:    &HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "s1", Address: "1.2.3.4:6443"}}},
					// LoadBalancerHostGroupName is nil
				}},
			wantErrMsg: ".external.loadBalancerHostGroupName: must be specified for managed external LB type 'kubexm-kh'",
			expectErr:  true,
		},
		{
			name: "Managed External LB (kubexm-kn) with empty LoadBalancerHostGroupName",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type: common.ExternalLBTypeKubexmKN,
					Keepalived:                &KeepalivedConfig{VRID: intPtr(1), Priority: intPtr(100), Interface: stringPtr("eth0"), AuthPass: stringPtr("pass")},
					NginxLB:                   &NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "1.2.3.4:6443"}}},
					LoadBalancerHostGroupName: stringPtr("   "),
				}},
			wantErrMsg: ".external.loadBalancerHostGroupName: must be specified for managed external LB type 'kubexm-kn'",
			expectErr:  true,
		},
		{
			name: "Valid Managed External LB (kubexm-kh) with LoadBalancerHostGroupName",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type:                      common.ExternalLBTypeKubexmKH,
					LoadBalancerHostGroupName: stringPtr("lb-group"),
					Keepalived:                &KeepalivedConfig{VRID: intPtr(1), Priority: intPtr(100), Interface: stringPtr("eth0"), AuthPass: stringPtr("pass")},
					HAProxy:                   &HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "s1", Address: "1.2.3.4:6443", Port: 6443}}}, // Explicitly set Port
				}},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_HighAvailabilityConfig(tt.cfg)
			verrs := &validation.ValidationErrors{}
			Validate_HighAvailabilityConfig(tt.cfg, verrs, "spec.highAvailability")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Validate_HighAvailabilityConfig expected error for %s, got none", tt.name)
				if tt.wantErrMsg != "" {
					assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Validate_HighAvailabilityConfig error for %s", tt.name)
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Validate_HighAvailabilityConfig for valid case %s failed: %v", tt.name, verrs.Error())
			}
		})
	}
}
