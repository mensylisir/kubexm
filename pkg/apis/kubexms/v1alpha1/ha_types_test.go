package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *HighAvailabilityConfig
		expected *HighAvailabilityConfig
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "HA disabled by default",
			input: &HighAvailabilityConfig{},
			expected: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(false),
				External: nil,
				Internal: nil,
			},
		},
		{
			name: "HA enabled, external and internal nil",
			input: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(true),
			},
			expected: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: nil,
				Internal: nil,
			},
		},
		{
			name: "default with external ManagedKeepalivedHAProxy type",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedHAProxy"},
			},
			expected: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type: "ManagedKeepalivedHAProxy",
					Keepalived: &KeepalivedConfig{
						VRID:         nil,
						Priority:     nil,
						Interface:    nil,
						AuthType:     util.StrPtr(common.KeepalivedAuthTypePASS),
						AuthPass:     util.StrPtr(common.DefaultKeepalivedAuthPass),
						SkipInstall:  util.BoolPtr(false),
						ExtraConfig:  []string{},
						Preempt:      util.BoolPtr(common.DefaultKeepalivedPreempt),
						CheckScript:  util.StrPtr(common.DefaultKeepalivedCheckScript),
						Interval:     util.IntPtr(common.DefaultKeepalivedInterval),
						Rise:         util.IntPtr(common.DefaultKeepalivedRise),
						Fall:         util.IntPtr(common.DefaultKeepalivedFall),
						AdvertInt:    util.IntPtr(common.DefaultKeepalivedAdvertInt),
						LVScheduler:  util.StrPtr(common.DefaultKeepalivedLVScheduler),
					},
					HAProxy: &HAProxyConfig{
						FrontendBindAddress: util.StrPtr("0.0.0.0"),
						FrontendPort:        util.IntPtr(common.HAProxyDefaultFrontendPort),
						Mode:                util.StrPtr(common.DefaultHAProxyMode),
						BalanceAlgorithm:    util.StrPtr(common.DefaultHAProxyAlgorithm),
						BackendServers:      []HAProxyBackendServer{},
						ExtraGlobalConfig:   []string{},
						ExtraDefaultsConfig: []string{},
						ExtraFrontendConfig: []string{},
						ExtraBackendConfig:  []string{},
						SkipInstall:         util.BoolPtr(false),
					},
				},
			},
		},
		{
			name: "default with external ManagedKeepalivedNginxLB type",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedNginxLB"},
			},
			expected: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type: "ManagedKeepalivedNginxLB",
					Keepalived: &KeepalivedConfig{
						VRID:         nil,
						Priority:     nil,
						Interface:    nil,
						AuthType:     util.StrPtr(common.KeepalivedAuthTypePASS),
						AuthPass:     util.StrPtr(common.DefaultKeepalivedAuthPass),
						SkipInstall:  util.BoolPtr(false),
						ExtraConfig:  []string{},
						Preempt:      util.BoolPtr(common.DefaultKeepalivedPreempt),
						CheckScript:  util.StrPtr(common.DefaultKeepalivedCheckScript),
						Interval:     util.IntPtr(common.DefaultKeepalivedInterval),
						Rise:         util.IntPtr(common.DefaultKeepalivedRise),
						Fall:         util.IntPtr(common.DefaultKeepalivedFall),
						AdvertInt:    util.IntPtr(common.DefaultKeepalivedAdvertInt),
						LVScheduler:  util.StrPtr(common.DefaultKeepalivedLVScheduler),
					},
					NginxLB: &NginxLBConfig{
						ListenAddress:     util.StrPtr("0.0.0.0"),
						ListenPort:        util.IntPtr(common.DefaultNginxListenPort),
						Mode:              util.StrPtr(common.DefaultNginxMode),
						BalanceAlgorithm:  util.StrPtr(common.DefaultNginxAlgorithm),
						UpstreamServers:   []NginxLBUpstreamServer{},
						ExtraHTTPConfig:   []string{},
						ExtraStreamConfig: []string{},
						ExtraServerConfig: []string{},
						ConfigFilePath:    util.StrPtr(common.DefaultNginxConfigFilePath),
						SkipInstall:       util.BoolPtr(false),
					},
				},
			},
		},
		{
			name: "default with internal KubeVIP type",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP},
			},
			expected: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{
					Type: common.InternalLBTypeKubeVIP,
					KubeVIP: &KubeVIPConfig{
						Mode:                 util.StrPtr(common.DefaultKubeVIPMode), // common.DefaultKubeVIPMode is "ARP"
						Image:                util.StrPtr(common.DefaultKubeVIPImage),
						ExtraArgs:            []string{},
						EnableControlPlaneLB: util.BoolPtr(true),
						EnableServicesLB:     util.BoolPtr(false),
						BGPConfig:            nil,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_HighAvailabilityConfig(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_HighAvailabilityConfig(t *testing.T) {
	validKeepalived := &KeepalivedConfig{
		VRID:         util.IntPtr(common.DefaultKeepalivedVRID),
		Priority:     util.IntPtr(common.DefaultKeepalivedPriorityMaster),
		Interface:    util.StrPtr("eth0"),
		AuthType:     util.StrPtr(common.KeepalivedAuthTypePASS),
		AuthPass:     util.StrPtr(common.DefaultKeepalivedAuthPass),
	}
	SetDefaults_KeepalivedConfig(validKeepalived)


	validHAProxy := &HAProxyConfig{}
	SetDefaults_HAProxyConfig(validHAProxy)
	validHAProxy.BackendServers = []HAProxyBackendServer{{Name: "s1", Address: "1.1.1.1", Port: 6443}}

	validNginxLB := &NginxLBConfig{}
	SetDefaults_NginxLBConfig(validNginxLB)
	validNginxLB.UpstreamServers = []NginxLBUpstreamServer{{Address: "1.1.1.1:6443"}}

	validKubeVIP := &KubeVIPConfig{}
	SetDefaults_KubeVIPConfig(validKubeVIP)
	validKubeVIP.VIP = util.StrPtr("192.168.1.200")
	validKubeVIP.Interface = util.StrPtr("eth0")

	tests := []struct {
		name         string
		input        *HighAvailabilityConfig
		expectError  bool
		errorMsgs    []string
	}{
		{
			name: "valid external ManagedKeepalivedHAProxy with endpoint IP (VIP removed, CPE moved)",
			input: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{
					Type:                      common.ExternalLBTypeKubexmKH,
					Keepalived:                validKeepalived,
					HAProxy:                   validHAProxy,
					LoadBalancerHostGroupName: util.StrPtr("lb-group"),
				},
			},
			expectError: false,
		},
		{
			name: "valid external UserProvided (CPE validation is at Cluster level)",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided"},
			},
			expectError: false,
		},
		{
			name: "valid HA disabled",
			input: &HighAvailabilityConfig{
				Enabled: util.BoolPtr(false),
			},
			expectError: false,
		},
		{
			name: "valid empty config (HA disabled by default)",
			input: &HighAvailabilityConfig{},
			expectError: false,
		},
		{
			name: "invalid external LB type",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "InvalidType"},
			},
			expectError: true,
			errorMsgs:    []string{".external.type: unknown external LB type 'InvalidType'"},
		},
		{
			name: "keepalived config present external type mismatch",
			input: &HighAvailabilityConfig{
				Enabled:    util.BoolPtr(true),
				External:   &ExternalLoadBalancerConfig{Type: "UserProvided", Keepalived: &KeepalivedConfig{}},
			},
			expectError: true,
			errorMsgs:    []string{".external.keepalived: should not be set for 'UserProvided' external LB type"},
		},
		{
			name: "valid internal KubeVIP",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: validKubeVIP},
			},
			expectError: false,
		},
		{
			name: "invalid internal LB type",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "InvalidInternalType"},
			},
			expectError: true,
			errorMsgs:    []string{".internal.type: unknown internal LB type 'InvalidInternalType'"},
		},
		{
			name: "KubeVIP internal LB missing KubeVIP section (will be defaulted, then validated for fields)",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: nil},
			},
			expectError: true,
			errorMsgs:    []string{
				"spec.highAvailability.internal.kubevip.vip: virtual IP address must be specified",
				"spec.highAvailability.internal.kubevip.interface: network interface must be specified for ARP mode",
			},
		},
		{
			name: "KubeVIP internal LB with nil KubeVIP but defaulted, expecting VIP/Interface errors",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: &KubeVIPConfig{}},
			},
			expectError: true,
			errorMsgs:    []string{"spec.highAvailability.internal.kubevip.vip: virtual IP address must be specified", "spec.highAvailability.internal.kubevip.interface: network interface must be specified for ARP mode"},
		},
		{
			name: "External and Internal LB simultaneously enabled",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided"},
				Internal: &InternalLoadBalancerConfig{Type: common.InternalLBTypeKubeVIP, KubeVIP: validKubeVIP},
			},
			expectError: true,
			errorMsgs:    []string{"external load balancer and internal load balancer cannot be enabled simultaneously"},
		},
		{
			name: "Managed External LB (kubexm-kh) missing LoadBalancerHostGroupName",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: common.ExternalLBTypeKubexmKH, Keepalived: validKeepalived, HAProxy: validHAProxy},
			},
			expectError: true,
			errorMsgs:    []string{".external.loadBalancerHostGroupName: must be specified for managed external LB type"},
		},
		{
			name: "Managed External LB (kubexm-kn) with empty LoadBalancerHostGroupName",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: common.ExternalLBTypeKubexmKN, Keepalived: validKeepalived, NginxLB: validNginxLB, LoadBalancerHostGroupName: util.StrPtr(" ")},
			},
			expectError: true,
			errorMsgs:    []string{".external.loadBalancerHostGroupName: must be specified for managed external LB type"},
		},
		{
			name: "Valid Managed External LB (kubexm-kh) with LoadBalancerHostGroupName",
			input: &HighAvailabilityConfig{
				Enabled:  util.BoolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: common.ExternalLBTypeKubexmKH, Keepalived: validKeepalived, HAProxy: validHAProxy, LoadBalancerHostGroupName: util.StrPtr("lb-nodes")},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputToTest := tt.input
			if inputToTest != nil {
				copiedInput := *inputToTest
				if copiedInput.External != nil {
					externalCopy := *copiedInput.External
					if externalCopy.Keepalived != nil {
						kpCopy := *externalCopy.Keepalived
						externalCopy.Keepalived = &kpCopy
					}
					if externalCopy.HAProxy != nil {
						hpCopy := *externalCopy.HAProxy
						externalCopy.HAProxy = &hpCopy
					}
					if externalCopy.NginxLB != nil {
						nginxCopy := *externalCopy.NginxLB
						externalCopy.NginxLB = &nginxCopy
					}
					copiedInput.External = &externalCopy
				}
				if copiedInput.Internal != nil {
					internalCopy := *copiedInput.Internal
					if internalCopy.KubeVIP != nil {
						kvCopy := *internalCopy.KubeVIP
						internalCopy.KubeVIP = &kvCopy
					}
					if internalCopy.WorkerNodeHAProxy != nil {
						hpCopy := *internalCopy.WorkerNodeHAProxy
						internalCopy.WorkerNodeHAProxy = &hpCopy
					}
					copiedInput.Internal = &internalCopy
				}
				inputToTest = &copiedInput
				SetDefaults_HighAvailabilityConfig(inputToTest)
			}

			verrs := &validation.ValidationErrors{}
			Validate_HighAvailabilityConfig(inputToTest, verrs, "spec.highAvailability")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v, DefaultedOrOriginal: %+v", tt.name, tt.input, inputToTest)
				if len(tt.errorMsgs) > 0 {
					fullErrorMsg := verrs.Error()
					for _, subMsg := range tt.errorMsgs {
						assert.Contains(t, fullErrorMsg, subMsg, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, subMsg, fullErrorMsg)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v, DefaultedOrOriginal: %+v", tt.name, verrs.Error(), tt.input, inputToTest)
			}
		})
	}
}
