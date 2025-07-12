package v1alpha1

import (
	"strings"
	"testing"
	// "net" // No longer needed here as TestNetworksOverlap was moved
	"github.com/stretchr/testify/assert" // Ensure testify is imported
	"github.com/mensylisir/kubexm/pkg/util" // For pointer helpers
)

// Helper functions (pboolNetworkTest, etc.) removed in favor of global helpers
// from zz_helpers.go (e.g., util.BoolPtr, int32Ptr, util.StrPtr, util.IntPtr)

// --- Test SetDefaults_NetworkConfig & Sub-configs ---
func TestSetDefaults_NetworkConfig_Overall(t *testing.T) {
	cfg := &NetworkConfig{}
	SetDefaults_NetworkConfig(cfg)

	if cfg.Plugin != "calico" { // Check the new default for Plugin
		t.Errorf("Default Plugin = %s, want calico", cfg.Plugin)
	}

	// Check general defaults
	if cfg.Multus == nil || cfg.Multus.Enabled == nil || *cfg.Multus.Enabled != false {
		t.Errorf("Multus default = %v, want enabled=false", cfg.Multus)
	}
	// Check KubeOvn and Hybridnet default enabled state
	if cfg.KubeOvn == nil || cfg.KubeOvn.Enabled == nil || *cfg.KubeOvn.Enabled != false {
	   t.Errorf("KubeOvn default = %v, want enabled=false", cfg.KubeOvn)
	}
	if cfg.Hybridnet == nil || cfg.Hybridnet.Enabled == nil || *cfg.Hybridnet.Enabled != false {
	   t.Errorf("Hybridnet default = %v, want enabled=false", cfg.Hybridnet)
	}

	// Test Calico defaults when plugin is Calico
	cfgCalico := &NetworkConfig{Plugin: "calico", IPPool: &IPPoolConfig{}} // Ensure IPPool is not nil for Calico defaults
	SetDefaults_NetworkConfig(cfgCalico)
	if cfgCalico.Calico == nil { t.Fatal("Calico config should be initialized when plugin is calico") }
	if cfgCalico.Calico.IPIPMode != "Always" { t.Errorf("Calico IPIPMode default failed") }
	if cfgCalico.Calico.VXLANMode != "Never" { t.Errorf("Calico VXLANMode default failed") }
	if cfgCalico.Calico.IPv4NatOutgoing == nil || !*cfgCalico.Calico.IPv4NatOutgoing {t.Error("Calico IPv4NatOutgoing default failed")}
	if cfgCalico.Calico.VethMTU == nil || *cfgCalico.Calico.VethMTU != 0 {t.Errorf("Calico VethMTU default failed, got %v, want 0", cfgCalico.Calico.VethMTU)}


	// Test Flannel defaults when plugin is Flannel
	cfgFlannel := &NetworkConfig{Plugin: "flannel"}
	SetDefaults_NetworkConfig(cfgFlannel)
	if cfgFlannel.Flannel == nil { t.Fatal("Flannel config should be initialized when plugin is flannel") }
	if cfgFlannel.Flannel.BackendMode != "vxlan" { t.Errorf("Flannel BackendMode default failed") }

	// Test KubeOvn defaults when enabled
	cfgKubeOvn := &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: util.BoolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgKubeOvn)
	if cfgKubeOvn.KubeOvn == nil { t.Fatal("KubeOvn config should be initialized for plugin kubeovn") }
	if cfgKubeOvn.KubeOvn.Label == nil || *cfgKubeOvn.KubeOvn.Label != "kube-ovn/role" { t.Errorf("KubeOvn Label default failed: %v", cfgKubeOvn.KubeOvn.Label) }
	if cfgKubeOvn.KubeOvn.TunnelType == nil || *cfgKubeOvn.KubeOvn.TunnelType != "geneve" { t.Errorf("KubeOvn TunnelType default failed: %v", cfgKubeOvn.KubeOvn.TunnelType) }
	if cfgKubeOvn.KubeOvn.EnableSSL == nil || *cfgKubeOvn.KubeOvn.EnableSSL != false { t.Errorf("KubeOvn EnableSSL default failed: %v", cfgKubeOvn.KubeOvn.EnableSSL) }

	// Test Hybridnet defaults when enabled
	cfgHybridnet := &NetworkConfig{Plugin: "hybridnet", Hybridnet: &HybridnetConfig{Enabled: util.BoolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgHybridnet)
	if cfgHybridnet.Hybridnet == nil { t.Fatal("Hybridnet config should be initialized for plugin hybridnet") }
	if cfgHybridnet.Hybridnet.DefaultNetworkType == nil || *cfgHybridnet.Hybridnet.DefaultNetworkType != "Overlay" { t.Errorf("Hybridnet DefaultNetworkType default failed: %v", cfgHybridnet.Hybridnet.DefaultNetworkType) }
	if cfgHybridnet.Hybridnet.EnableNetworkPolicy == nil || !*cfgHybridnet.Hybridnet.EnableNetworkPolicy { t.Errorf("Hybridnet EnableNetworkPolicy default failed: %v", cfgHybridnet.Hybridnet.EnableNetworkPolicy) }
	if cfgHybridnet.Hybridnet.InitDefaultNetwork == nil || !*cfgHybridnet.Hybridnet.InitDefaultNetwork { t.Errorf("Hybridnet InitDefaultNetwork default failed: %v", cfgHybridnet.Hybridnet.InitDefaultNetwork) }

	cfgCalicoWithPool := &NetworkConfig{Plugin: "calico", KubePodsCIDR: "192.168.0.0/16", Calico: &CalicoConfig{DefaultIPPOOL: util.BoolPtr(true), IPPools: []CalicoIPPool{}}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgCalicoWithPool)
	if cfgCalicoWithPool.Calico == nil {t.Fatal("Calico config should be initialized")}
	if cfgCalicoWithPool.Calico.LogSeverityScreen == nil || *cfgCalicoWithPool.Calico.LogSeverityScreen != "Info" {t.Errorf("Calico LogSeverityScreen default failed: %v", cfgCalicoWithPool.Calico.LogSeverityScreen)}
	if len(cfgCalicoWithPool.Calico.IPPools) != 1 {t.Fatalf("Expected 1 default Calico IPPool, got %d", len(cfgCalicoWithPool.Calico.IPPools))}
	defaultPool := cfgCalicoWithPool.Calico.IPPools[0]
	if defaultPool.CIDR != "192.168.0.0/16" {t.Errorf("Default IPPool CIDR mismatch: %s", defaultPool.CIDR)}
	if defaultPool.BlockSize == nil || *defaultPool.BlockSize != 26 {t.Errorf("Default IPPool BlockSize mismatch: %v", defaultPool.BlockSize)}
	if defaultPool.Encapsulation != "IPIP" {t.Errorf("Default IPPool Encapsulation mismatch: got %s, want IPIP (due to default IPIPMode=Always)", defaultPool.Encapsulation)}
	if cfgCalicoWithPool.Calico.TyphaNodeSelector == nil {t.Error("Calico TyphaNodeSelector should be initialized")}

	testCases := []struct {
		name             string
		ipipMode         string
		vxlanMode        string
		expectedEncap    string
	}{
		{"ipip_always", "Always", "Never", "IPIP"},
		{"ipip_crosssubnet", "CrossSubnet", "Never", "IPIP"},
		{"vxlan_always_ipip_never", "Never", "Always", "VXLAN"},
		{"vxlan_crosssubnet_ipip_never", "Never", "CrossSubnet", "VXLAN"},
		{"both_never", "Never", "Never", "None"},
		{"ipip_always_vxlan_always", "Always", "Always", "IPIP"},
	}

	for _, tc := range testCases {
		t.Run("DefaultPoolEncap_"+tc.name, func(t *testing.T) {
			cfg := &NetworkConfig{
				Plugin:       "calico",
				KubePodsCIDR: "192.168.1.0/24",
				Calico: &CalicoConfig{
					IPIPMode:      tc.ipipMode,
					VXLANMode:     tc.vxlanMode,
					DefaultIPPOOL: util.BoolPtr(true), // Ensure default pool is created to test its encap
					IPPools:       []CalicoIPPool{}, // Start with no custom pools
				},
				IPPool: &IPPoolConfig{BlockSize: util.IntPtr(26)},
			}
			SetDefaults_NetworkConfig(cfg)
			if len(cfg.Calico.IPPools) < 1 { // Should be at least 1 (the default one)
				t.Fatalf("Expected at least 1 default IPPool, got %d for case %s", len(cfg.Calico.IPPools), tc.name)
			}
			assert.Equal(t, tc.expectedEncap, cfg.Calico.IPPools[0].Encapsulation, "Case %s: Default IPPool Encapsulation mismatch", tc.name)

			// Now test custom pool with empty encapsulation
			cfgWithCustomPool := &NetworkConfig{
				Plugin:       "calico",
				KubePodsCIDR: "192.168.1.0/24",
				Calico: &CalicoConfig{
					IPIPMode:      tc.ipipMode,
					VXLANMode:     tc.vxlanMode,
					DefaultIPPOOL: util.BoolPtr(false), // Disable default pool to isolate custom pool test
					IPPools: []CalicoIPPool{
						{Name: "custom", CIDR: "10.10.0.0/16", Encapsulation: ""}, // Empty encap
					},
				},
				IPPool: &IPPoolConfig{BlockSize: util.IntPtr(26)},
			}
			SetDefaults_NetworkConfig(cfgWithCustomPool)
			if assert.Len(t, cfgWithCustomPool.Calico.IPPools, 1, "Should have 1 custom IPPool for case %s", tc.name) {
				assert.Equal(t, tc.expectedEncap, cfgWithCustomPool.Calico.IPPools[0].Encapsulation, "Case %s: Custom IPPool with empty Encapsulation, default mismatch", tc.name)
			}

			// Test custom pool with non-empty encapsulation (should not be changed)
			userDefinedEncap := "VXLAN"
			if tc.expectedEncap == "VXLAN" { // Avoid testing with the same as expected default
				userDefinedEncap = "IPIP"
			}
			if tc.expectedEncap == "None" && userDefinedEncap == "IPIP" { // if default is None, test with IPIP
                 // no change needed
			} else if tc.expectedEncap == "None" { // if default is None, test with VXLAN
				userDefinedEncap = "VXLAN"
			}


			cfgWithUserEncap := &NetworkConfig{
				Plugin:       "calico",
				KubePodsCIDR: "192.168.1.0/24",
				Calico: &CalicoConfig{
					IPIPMode:      tc.ipipMode,
					VXLANMode:     tc.vxlanMode,
					DefaultIPPOOL: util.BoolPtr(false),
					IPPools: []CalicoIPPool{
						{Name: "custom-user", CIDR: "10.10.10.0/16", Encapsulation: userDefinedEncap},
					},
				},
				IPPool: &IPPoolConfig{BlockSize: util.IntPtr(26)},
			}
			SetDefaults_NetworkConfig(cfgWithUserEncap)
			if assert.Len(t, cfgWithUserEncap.Calico.IPPools, 1, "Should have 1 custom IPPool with user encap for case %s", tc.name) {
				assert.Equal(t, userDefinedEncap, cfgWithUserEncap.Calico.IPPools[0].Encapsulation, "Case %s: Custom IPPool with user-defined Encapsulation should not change", tc.name)
			}
		})
	}

	// Test that default IPPool is not created if KubePodsCIDR is empty
	cfgNoPodsCIDR := &NetworkConfig{
		Plugin: "calico",
		KubePodsCIDR: "", // Empty PodsCIDR
		Calico: &CalicoConfig{DefaultIPPOOL: util.BoolPtr(true)},
		IPPool: &IPPoolConfig{},
	}
	SetDefaults_NetworkConfig(cfgNoPodsCIDR)
	assert.NotNil(t, cfgNoPodsCIDR.Calico, "Calico should be initialized even with no PodsCIDR")
	assert.Empty(t, cfgNoPodsCIDR.Calico.IPPools, "Default Calico IPPool should not be created if KubePodsCIDR is empty")

}

func TestValidate_NetworkConfig_Valid(t *testing.T) {
	k8sCfgMinimal := &KubernetesConfig{Version: "v1.25.0"}
	SetDefaults_KubernetesConfig(k8sCfgMinimal, "test-cluster")


	cfg := &NetworkConfig{
		Plugin:          "calico",
		KubePodsCIDR:    "10.244.0.0/16",
		KubeServiceCIDR: "10.96.0.0/12",
		Calico: &CalicoConfig{IPIPMode: "Always", VXLANMode: "Never"},
	}
	SetDefaults_NetworkConfig(cfg)
	verrs := &ValidationErrors{}
	Validate_NetworkConfig(cfg, verrs, "spec.network", k8sCfgMinimal) // Pass k8sCfgMinimal
	if verrs.HasErrors() { // Updated
		t.Errorf("Validate_NetworkConfig for valid Calico config failed: %v", verrs.Error()) // Updated
	}
}

func TestValidate_NetworkConfig_Invalid(t *testing.T) {
	k8sCfgMinimal := &KubernetesConfig{Version: "v1.25.0"}
	SetDefaults_KubernetesConfig(k8sCfgMinimal, "test-cluster")

	tests := []struct {
		name        string
		cfg         *NetworkConfig
		wantErrMsg  string
	}{
		{"nil_config", nil, "network configuration section cannot be nil"},
		{"empty_pod_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: ""}, ".kubePodsCIDR: cannot be empty"},
		{"invalid_pod_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "invalid"}, ".kubePodsCIDR: invalid CIDR format"},
		{"calico_invalid_ipip", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPIPMode: "bad"}}, ".calico.ipipMode: invalid: 'bad'"},
		{"flannel_invalid_backend", &NetworkConfig{Plugin: "flannel", KubePodsCIDR: "10.244.0.0/16", Flannel: &FlannelConfig{BackendMode: "bad"}}, "spec.network.flannel.backendMode: invalid: 'bad'"},
		{"kubeovn_invalid_tunneltype", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: util.BoolPtr(true), TunnelType: util.StrPtr("bad")}}, ".kubeovn.tunnelType: invalid type 'bad'"},
		{"kubeovn_invalid_joincidr", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: util.BoolPtr(true), JoinCIDR: util.StrPtr("invalid")}}, ".kubeovn.joinCIDR: invalid CIDR format"},
		{"hybridnet_invalid_networktype", &NetworkConfig{Plugin: "hybridnet", KubePodsCIDR: "10.244.0.0/16", Hybridnet: &HybridnetConfig{Enabled: util.BoolPtr(true), DefaultNetworkType: util.StrPtr("bad")}}, ".hybridnet.defaultNetworkType: invalid type 'bad'"},
		{"calico_invalid_logseverity", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{LogSeverityScreen: util.StrPtr("trace")}}, ".calico.logSeverityScreen: invalid: 'trace'"},
		// Calico IPPool specific validations are now in TestValidate_CalicoConfig
		// {"calico_ippool_empty_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: ""}}}}, ".calico.ipPools[0:p1].cidr: cannot be empty"},
		// {"calico_ippool_invalid_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: "invalid"}}}}, ".calico.ipPools[0:p1].cidr: invalid CIDR 'invalid'"},
		// {"calico_ippool_bad_blocksize_low", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: util.IntPtr(19)}}}}, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		// {"calico_ippool_bad_blocksize_high", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: util.IntPtr(33)}}}}, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		// {"calico_ippool_bad_encap", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",Encapsulation: "bad"}}}}, ".calico.ipPools[0:p1].encapsulation: invalid: 'bad'"},
		{
			"cidrs_overlap_pods_in_service",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.96.0.0/16", KubeServiceCIDR: "10.96.0.0/12"},
			"kubePodsCIDR (10.96.0.0/16) and kubeServiceCIDR (10.96.0.0/12) overlap",
		},
		{
			"cidrs_overlap_service_in_pods",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/12", KubeServiceCIDR: "10.244.1.0/24"},
			"kubePodsCIDR (10.244.0.0/12) and kubeServiceCIDR (10.244.1.0/24) overlap",
		},
		{
			"cidrs_valid_no_overlap",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", KubeServiceCIDR: "10.96.0.0/12"},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg != nil { SetDefaults_NetworkConfig(tt.cfg) }

			verrs := &ValidationErrors{}
			Validate_NetworkConfig(tt.cfg, verrs, "spec.network", k8sCfgMinimal) // Pass k8sCfgMinimal

			if tt.wantErrMsg == "" {
				if verrs.HasErrors() { // Updated
					t.Errorf("Validate_NetworkConfig for %s expected no error, got %v", tt.name, verrs.Error()) // Updated
				}
			} else {
				if !verrs.HasErrors() { // Updated
					t.Fatalf("Validate_NetworkConfig expected error for %s, got none", tt.name)
				}
				found := false
				// Error() returns a single string with errors separated by \n
				for _, errStrSingle := range strings.Split(verrs.Error(), "\n") { // Updated
					if strings.Contains(errStrSingle, tt.wantErrMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Validate_NetworkConfig error for %s. Expected to contain '%s', got errors: %v", tt.name, tt.wantErrMsg, verrs.Error()) // Updated
				}
			}
		})
	}
}

func TestValidate_FlannelConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *FlannelConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"valid_empty", &FlannelConfig{}, ""}, // Defaults applied by parent
		{"valid_vxlan", &FlannelConfig{BackendMode: "vxlan"}, ""},
		{"valid_host-gw", &FlannelConfig{BackendMode: "host-gw", DirectRouting: util.BoolPtr(true)}, ""},
		{"invalid_backendMode", &FlannelConfig{BackendMode: "bad-backend"}, ".backendMode: invalid: 'bad-backend'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			if tt.cfg != nil && tt.name == "valid_empty" {
				SetDefaults_FlannelConfig(tt.cfg)
			}
			Validate_FlannelConfig(tt.cfg, verrs, "")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidate_KubeOvnConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *KubeOvnConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"disabled", &KubeOvnConfig{Enabled: util.BoolPtr(false)}, ""},
		{"valid_enabled_defaults", &KubeOvnConfig{Enabled: util.BoolPtr(true)}, ""}, // Defaults applied by parent
		{"valid_full", &KubeOvnConfig{
			Enabled:    util.BoolPtr(true),
			JoinCIDR:   util.StrPtr("100.64.0.0/16"),
			Label:      util.StrPtr("custom/label"),
			TunnelType: util.StrPtr("vxlan"),
			EnableSSL:  util.BoolPtr(true),
		}, ""},
		{"enabled_invalid_joinCIDR", &KubeOvnConfig{Enabled: util.BoolPtr(true), JoinCIDR: util.StrPtr("not-a-cidr")}, ".joinCIDR: invalid CIDR format"},
		{"enabled_empty_label", &KubeOvnConfig{Enabled: util.BoolPtr(true), Label: util.StrPtr(" ")}, ".label: cannot be empty if specified"},
		{"enabled_invalid_tunnelType", &KubeOvnConfig{Enabled: util.BoolPtr(true), TunnelType: util.StrPtr("gre")}, ".tunnelType: invalid type 'gre'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			if tt.cfg != nil && tt.name == "valid_enabled_defaults" {
				SetDefaults_KubeOvnConfig(tt.cfg)
			}
			Validate_KubeOvnConfig(tt.cfg, verrs, "")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidate_HybridnetConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *HybridnetConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"disabled", &HybridnetConfig{Enabled: util.BoolPtr(false)}, ""},
		{"valid_enabled_defaults", &HybridnetConfig{Enabled: util.BoolPtr(true)}, ""}, // Defaults applied by parent
		{"valid_full", &HybridnetConfig{
			Enabled:             util.BoolPtr(true),
			DefaultNetworkType:  util.StrPtr("Underlay"),
			EnableNetworkPolicy: util.BoolPtr(false),
			InitDefaultNetwork:  util.BoolPtr(false),
		}, ""},
		{"enabled_invalid_networkType", &HybridnetConfig{Enabled: util.BoolPtr(true), DefaultNetworkType: util.StrPtr("Mixed")}, ".defaultNetworkType: invalid type 'Mixed'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			if tt.cfg != nil && tt.name == "valid_enabled_defaults" {
				SetDefaults_HybridnetConfig(tt.cfg)
			}
			Validate_HybridnetConfig(tt.cfg, verrs, "")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidate_IPPoolConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *IPPoolConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"valid_empty", &IPPoolConfig{}, ""}, // No validation on empty struct itself
		{"valid_blockSize", &IPPoolConfig{BlockSize: util.IntPtr(24)}, ""},
		{"invalid_blockSize_low", &IPPoolConfig{BlockSize: util.IntPtr(19)}, ".blockSize: must be between 20 and 32 if specified"},
		{"invalid_blockSize_high", &IPPoolConfig{BlockSize: util.IntPtr(33)}, ".blockSize: must be between 20 and 32 if specified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			Validate_IPPoolConfig(tt.cfg, verrs, "")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestNetworkConfig_EnableMultusCNI(t *testing.T) {
   cfg := &NetworkConfig{IPPool: &IPPoolConfig{}}
   SetDefaults_NetworkConfig(cfg)
   if cfg.EnableMultusCNI() != false {t.Error("EnableMultusCNI default failed")}

   cfg.Multus.Enabled = util.BoolPtr(true)
   if cfg.EnableMultusCNI() != true {t.Error("EnableMultusCNI true failed")}
}

func TestCalicoConfig_TyphaHelpers(t *testing.T) {
   cfg := &CalicoConfig{}
   SetDefaults_CalicoConfig(cfg, "", nil)
   if cfg.IsTyphaEnabled() != false {t.Error("IsTyphaEnabled default failed")}
   if cfg.GetTyphaReplicas() != 0 {t.Error("GetTyphaReplicas default for disabled Typha failed")}

   cfg.EnableTypha = util.BoolPtr(true)
   SetDefaults_CalicoConfig(cfg, "", nil)
   if cfg.IsTyphaEnabled() != true {t.Error("IsTyphaEnabled true failed")}
   if cfg.GetTyphaReplicas() != 2 {t.Errorf("GetTyphaReplicas default for enabled Typha failed, got %d", cfg.GetTyphaReplicas())}

   cfg.TyphaReplicas = util.IntPtr(5)
   if cfg.GetTyphaReplicas() != 5 {t.Errorf("GetTyphaReplicas custom failed, got %d", cfg.GetTyphaReplicas())}
}

func TestValidate_NetworkConfig_Calls_Validate_CiliumConfig(t *testing.T) {
	k8sCfgMinimal := &KubernetesConfig{Version: "v1.25.0"}
	SetDefaults_KubernetesConfig(k8sCfgMinimal, "test-cluster")

	cfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{
			TunnelingMode: "invalid-mode", // This will be validated by Validate_CiliumConfig
		},
		KubePodsCIDR:    "10.244.0.0/16",
		KubeServiceCIDR: "10.96.0.0/12",
	}
	// Apply defaults to ensure CiliumConfig is processed if it was nil initially
	SetDefaults_NetworkConfig(cfg)


	verrs := &ValidationErrors{}
	Validate_NetworkConfig(cfg, verrs, "spec.network", k8sCfgMinimal) // Pass k8sCfgMinimal

	if !verrs.HasErrors() {
		t.Fatal("Expected validation errors from CiliumConfig via NetworkConfig, but got none.")
	}

	expectedErrorSubstring := "spec.network.cilium.tunnelingMode: invalid mode 'invalid-mode'"
	found := false
	for _, errStr := range strings.Split(verrs.Error(), "\n") {
		if strings.Contains(errStr, expectedErrorSubstring) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error substring '%s' not found in errors: %v", expectedErrorSubstring, verrs.Error())
	}
}

func TestSetDefaults_NetworkConfig_Calls_SetDefaults_CiliumConfig(t *testing.T) {
	cfg := &NetworkConfig{
		Plugin: "cilium",
		// Cilium: &CiliumConfig{}, // Intentionally leave nil to test initialization by parent
		IPPool: &IPPoolConfig{},
	}
	SetDefaults_NetworkConfig(cfg)

	if cfg.Cilium == nil {
		t.Fatal("CiliumConfig should have been initialized by SetDefaults_NetworkConfig.")
	}
	// Defaults for Cilium are tested in cilium_types_test.go, here we just check it's called.
	// We can check one key default to be sure.
	if cfg.Cilium.TunnelingMode != "vxlan" { // Assuming "vxlan" is a default in CiliumConfig
		t.Errorf("CiliumConfig TunnelingMode default was not applied as expected: got %s", cfg.Cilium.TunnelingMode)
	}
}


func TestValidate_CalicoConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *CalicoConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""}, // Should not panic
		{"valid_empty", &CalicoConfig{}, ""}, // Defaults will be applied by parent
		{"valid_full", &CalicoConfig{
			IPIPMode:          "Always",
			VXLANMode:         "Never",
			VethMTU:           util.IntPtr(1440),
			IPv4NatOutgoing:   util.BoolPtr(true),
			DefaultIPPOOL:     util.BoolPtr(true),
			EnableTypha:       util.BoolPtr(true),
			TyphaReplicas:     util.IntPtr(3),
			LogSeverityScreen: util.StrPtr("Debug"),
			IPPools: []CalicoIPPool{
				{Name: "pool1", CIDR: "10.0.1.0/24", Encapsulation: "IPIP", BlockSize: util.IntPtr(26)},
				{Name: "pool2", CIDR: "10.0.2.0/24", Encapsulation: "VXLAN", NatOutgoing: util.BoolPtr(false)},
			},
		}, ""},
		{"invalid_ipipMode", &CalicoConfig{IPIPMode: "Sometimes"}, ".ipipMode: invalid: 'Sometimes'"},
		{"invalid_vxlanMode", &CalicoConfig{VXLANMode: "Maybe"}, ".vxlanMode: invalid: 'Maybe'"},
		{"invalid_vethMTU_negative", &CalicoConfig{VethMTU: util.IntPtr(-100)}, ".vethMTU: invalid: -100"},
		{"typha_enabled_invalid_replicas_zero", &CalicoConfig{EnableTypha: util.BoolPtr(true), TyphaReplicas: util.IntPtr(0)}, ".typhaReplicas: must be positive if Typha is enabled"},
		{"typha_enabled_invalid_replicas_negative", &CalicoConfig{EnableTypha: util.BoolPtr(true), TyphaReplicas: util.IntPtr(-1)}, ".typhaReplicas: must be positive if Typha is enabled"},
		{"typha_enabled_nil_replicas", &CalicoConfig{EnableTypha: util.BoolPtr(true), TyphaReplicas: nil}, ".typhaReplicas: must be positive if Typha is enabled"}, // After defaults, this would be 2. Test assumes direct validation.
		{"invalid_logSeverityScreen", &CalicoConfig{LogSeverityScreen: util.StrPtr("Verbose")}, ".logSeverityScreen: invalid: 'Verbose'"},
		{"ippool_empty_cidr", &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: ""}}}, ".ipPools[0:p1].cidr: cannot be empty"},
		{"ippool_invalid_cidr", &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: "not-a-cidr"}}}, ".ipPools[0:p1].cidr: invalid CIDR 'not-a-cidr'"},
		{"ippool_invalid_blockSize_low", &CalicoConfig{IPPools: []CalicoIPPool{{CIDR: "1.1.1.0/24", BlockSize: util.IntPtr(19)}}}, ".ipPools[0:].blockSize: must be between 20 and 32"},
		{"ippool_invalid_blockSize_high", &CalicoConfig{IPPools: []CalicoIPPool{{CIDR: "1.1.1.0/24", BlockSize: util.IntPtr(33)}}}, ".ipPools[0:].blockSize: must be between 20 and 32"},
		{"ippool_invalid_encapsulation", &CalicoConfig{IPPools: []CalicoIPPool{{CIDR: "1.1.1.0/24", Encapsulation: "GRE"}}}, ".ipPools[0:].encapsulation: invalid: 'GRE'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			// Simulate defaults being applied if cfg is empty, for standalone validation
			if tt.cfg != nil && tt.name == "valid_empty" {
				SetDefaults_CalicoConfig(tt.cfg, "10.244.0.0/16", util.IntPtr(26))
			}
			// For TyphaReplicas with nil, default would set it. If testing direct validation after external defaulting:
			if tt.name == "typha_enabled_nil_replicas" && tt.cfg != nil {
				// SetDefaults_CalicoConfig would set TyphaReplicas to 2 if EnableTypha is true and TyphaReplicas is nil.
				// So, to test the validation rule "must be positive if Typha is enabled" directly
				// without the default kicking in, we'd need a scenario where default is NOT applied OR replicas is explicitly 0/negative.
				// The existing "typha_enabled_invalid_replicas_zero" and "negative" cover this better if default is not run before this validation.
				// Let's assume for this unit test, defaults are not run unless specified (like for valid_empty).
			}

			Validate_CalicoConfig(tt.cfg, verrs, "") // pathPrefix is empty for direct validation
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}
