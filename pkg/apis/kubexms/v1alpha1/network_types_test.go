package v1alpha1

import (
	"strings"
	"testing"
)

// Helper functions (pboolNetworkTest, etc.) removed in favor of global helpers
// from zz_helpers.go (e.g., boolPtr, int32Ptr, stringPtr, intPtr)

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
	// VethMTU defaults to 0 in SetDefaults_CalicoConfig, which means Calico auto-detects.
	// If a specific default like 1440 was intended, SetDefaults_CalicoConfig would need to set it.
	// Assuming 0 is the intended "let Calico decide" default from our code.
	if cfgCalico.Calico.VethMTU == nil || *cfgCalico.Calico.VethMTU != 0 {t.Errorf("Calico VethMTU default failed, got %v, want 0", cfgCalico.Calico.VethMTU)}


	// Test Flannel defaults when plugin is Flannel
	cfgFlannel := &NetworkConfig{Plugin: "flannel"}
	SetDefaults_NetworkConfig(cfgFlannel)
	if cfgFlannel.Flannel == nil { t.Fatal("Flannel config should be initialized when plugin is flannel") }
	if cfgFlannel.Flannel.BackendMode != "vxlan" { t.Errorf("Flannel BackendMode default failed") }

	// Test KubeOvn defaults when enabled
	cfgKubeOvn := &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgKubeOvn) // Should call SetDefaults_KubeOvnConfig
	if cfgKubeOvn.KubeOvn == nil { t.Fatal("KubeOvn config should be initialized for plugin kubeovn") }
	if cfgKubeOvn.KubeOvn.Label == nil || *cfgKubeOvn.KubeOvn.Label != "kube-ovn/role" { t.Errorf("KubeOvn Label default failed: %v", cfgKubeOvn.KubeOvn.Label) }
	if cfgKubeOvn.KubeOvn.TunnelType == nil || *cfgKubeOvn.KubeOvn.TunnelType != "geneve" { t.Errorf("KubeOvn TunnelType default failed: %v", cfgKubeOvn.KubeOvn.TunnelType) }
	if cfgKubeOvn.KubeOvn.EnableSSL == nil || *cfgKubeOvn.KubeOvn.EnableSSL != false { t.Errorf("KubeOvn EnableSSL default failed: %v", cfgKubeOvn.KubeOvn.EnableSSL) }

	// Test Hybridnet defaults when enabled
	cfgHybridnet := &NetworkConfig{Plugin: "hybridnet", Hybridnet: &HybridnetConfig{Enabled: boolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgHybridnet) // Should call SetDefaults_HybridnetConfig
	if cfgHybridnet.Hybridnet == nil { t.Fatal("Hybridnet config should be initialized for plugin hybridnet") }
	if cfgHybridnet.Hybridnet.DefaultNetworkType == nil || *cfgHybridnet.Hybridnet.DefaultNetworkType != "Overlay" { t.Errorf("Hybridnet DefaultNetworkType default failed: %v", cfgHybridnet.Hybridnet.DefaultNetworkType) }
	if cfgHybridnet.Hybridnet.EnableNetworkPolicy == nil || !*cfgHybridnet.Hybridnet.EnableNetworkPolicy { t.Errorf("Hybridnet EnableNetworkPolicy default failed: %v", cfgHybridnet.Hybridnet.EnableNetworkPolicy) }
	if cfgHybridnet.Hybridnet.InitDefaultNetwork == nil || !*cfgHybridnet.Hybridnet.InitDefaultNetwork { t.Errorf("Hybridnet InitDefaultNetwork default failed: %v", cfgHybridnet.Hybridnet.InitDefaultNetwork) }

	// Test Calico Default IPPool creation and LogSeverityScreen
	cfgCalicoWithPool := &NetworkConfig{Plugin: "calico", KubePodsCIDR: "192.168.0.0/16", Calico: &CalicoConfig{DefaultIPPOOL: boolPtr(true), IPPools: []CalicoIPPool{}}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgCalicoWithPool)
	if cfgCalicoWithPool.Calico == nil {t.Fatal("Calico config should be initialized")}
	if cfgCalicoWithPool.Calico.LogSeverityScreen == nil || *cfgCalicoWithPool.Calico.LogSeverityScreen != "Info" {t.Errorf("Calico LogSeverityScreen default failed: %v", cfgCalicoWithPool.Calico.LogSeverityScreen)}
	if len(cfgCalicoWithPool.Calico.IPPools) != 1 {t.Fatalf("Expected 1 default Calico IPPool, got %d", len(cfgCalicoWithPool.Calico.IPPools))}
	defaultPool := cfgCalicoWithPool.Calico.IPPools[0]
	if defaultPool.CIDR != "192.168.0.0/16" {t.Errorf("Default IPPool CIDR mismatch: %s", defaultPool.CIDR)}
	if defaultPool.BlockSize == nil || *defaultPool.BlockSize != 26 {t.Errorf("Default IPPool BlockSize mismatch: %v", defaultPool.BlockSize)}
	if cfgCalicoWithPool.Calico.TyphaNodeSelector == nil {t.Error("Calico TyphaNodeSelector should be initialized")}
}

// --- Test Validate_NetworkConfig & Sub-configs ---
func TestValidate_NetworkConfig_Valid(t *testing.T) {
	// k8sCfg is passed to Validate_NetworkConfig but no longer used for Pod/Service CIDR derivation by it.
	// Can be an empty struct or nil if Validate_NetworkConfig signature changes to remove it.
	k8sCfg := &KubernetesConfig{}
	cfg := &NetworkConfig{
		Plugin:          "calico",
		KubePodsCIDR:    "10.244.0.0/16",
		KubeServiceCIDR: "10.96.0.0/12",
		Calico: &CalicoConfig{IPIPMode: "Always", VXLANMode: "Never"},
	}
	SetDefaults_NetworkConfig(cfg) // Apply defaults
	verrs := &ValidationErrors{}
	Validate_NetworkConfig(cfg, verrs, "spec.network", k8sCfg)
	if !verrs.IsEmpty() {
		t.Errorf("Validate_NetworkConfig for valid Calico config failed: %v", verrs)
	}
}

func TestValidate_NetworkConfig_Invalid(t *testing.T) {
	// k8sCfg is passed to Validate_NetworkConfig but its PodSubnet/ServiceSubnet are not used by it anymore.
	// Providing a default empty one for the signature.
	defaultK8sCfg := &KubernetesConfig{}
	tests := []struct {
		name        string
		cfg         *NetworkConfig
		k8sForTest  *KubernetesConfig // Optional k8s config for specific test cases, mostly can be defaultK8sCfg
		wantErrMsg  string
	}{
		{"nil_config", nil, defaultK8sCfg, "network configuration section cannot be nil"},
		{"empty_pod_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: ""}, defaultK8sCfg, ".kubePodsCIDR: cannot be empty"},
		{"invalid_pod_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "invalid"}, defaultK8sCfg, ".kubePodsCIDR: invalid CIDR format"},
		// {"calico_nil_if_plugin_calico", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: nil}, defaultK8sCfg, ".calico: config cannot be nil if plugin is 'calico'"}, // Defaulting ensures Calico is non-nil
		{"calico_invalid_ipip", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPIPMode: "bad"}}, defaultK8sCfg, ".calico.ipipMode: invalid: 'bad'"},
		// {"flannel_nil_if_plugin_flannel", &NetworkConfig{Plugin: "flannel", KubePodsCIDR: "10.244.0.0/16", Flannel: nil}, defaultK8sCfg, ".flannel: config cannot be nil if plugin is 'flannel'"}, // Defaulting ensures Flannel is non-nil
		{"flannel_invalid_backend", &NetworkConfig{Plugin: "flannel", KubePodsCIDR: "10.244.0.0/16", Flannel: &FlannelConfig{BackendMode: "bad"}}, defaultK8sCfg, "spec.network.flannel.backendMode: invalid: 'bad'"}, // Exact full message
		{"kubeovn_invalid_tunneltype", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true), TunnelType: stringPtr("bad")}}, defaultK8sCfg, ".kubeovn.tunnelType: invalid type 'bad'"},
		{"kubeovn_invalid_joincidr", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true), JoinCIDR: stringPtr("invalid")}}, defaultK8sCfg, ".kubeovn.joinCIDR: invalid CIDR format"},
		{"hybridnet_invalid_networktype", &NetworkConfig{Plugin: "hybridnet", KubePodsCIDR: "10.244.0.0/16", Hybridnet: &HybridnetConfig{Enabled: boolPtr(true), DefaultNetworkType: stringPtr("bad")}}, defaultK8sCfg, ".hybridnet.defaultNetworkType: invalid type 'bad'"},
		{"calico_invalid_logseverity", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{LogSeverityScreen: stringPtr("trace")}}, defaultK8sCfg, ".calico.logSeverityScreen: invalid: 'trace'"},
		{"calico_ippool_empty_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: ""}}}}, defaultK8sCfg, ".calico.ipPools[0:p1].cidr: cannot be empty"},
		{"calico_ippool_invalid_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: "invalid"}}}}, defaultK8sCfg, ".calico.ipPools[0:p1].cidr: invalid CIDR 'invalid'"},
		{"calico_ippool_bad_blocksize_low", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: intPtr(19)}}}}, defaultK8sCfg, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_blocksize_high", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: intPtr(33)}}}}, defaultK8sCfg, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_encap", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",Encapsulation: "bad"}}}}, defaultK8sCfg, ".calico.ipPools[0:p1].encapsulation: invalid: 'bad'"},
		{
			"cidrs_overlap_pods_in_service",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.96.0.0/16", KubeServiceCIDR: "10.96.0.0/12"},
			defaultK8sCfg,
			"kubePodsCIDR (10.96.0.0/16) and kubeServiceCIDR (10.96.0.0/12) overlap",
		},
		{
			"cidrs_overlap_service_in_pods",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/12", KubeServiceCIDR: "10.244.1.0/24"},
			defaultK8sCfg,
			"kubePodsCIDR (10.244.0.0/12) and kubeServiceCIDR (10.244.1.0/24) overlap",
		},
		{
			"cidrs_valid_no_overlap",
			&NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", KubeServiceCIDR: "10.96.0.0/12"},
			defaultK8sCfg,
			"", // Expect no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentK8sCfgToUse := tt.k8sForTest // Use the one from test case if specified
			if currentK8sCfgToUse == nil {
			   currentK8sCfgToUse = defaultK8sCfg // Fallback to the default empty k8sCfg
			}
			if tt.cfg != nil { SetDefaults_NetworkConfig(tt.cfg) }

			verrs := &ValidationErrors{}
			Validate_NetworkConfig(tt.cfg, verrs, "spec.network", currentK8sCfgToUse)

			if tt.wantErrMsg == "" { // This case expects no error
				if !verrs.IsEmpty() {
					t.Errorf("Validate_NetworkConfig for %s expected no error, got %v", tt.name, verrs.Errors)
				}
			} else { // This case expects an error
				if verrs.IsEmpty() {
					t.Fatalf("Validate_NetworkConfig expected error for %s, got none", tt.name)
				}
				found := false
				for _, errStr := range verrs.Errors {
					if strings.Contains(errStr, tt.wantErrMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Validate_NetworkConfig error for %s. Expected to contain '%s', got errors: %v", tt.name, tt.wantErrMsg, verrs.Errors)
				}
			}
		})
	}
}

// --- Test NetworkConfig Helper Methods ---
func TestNetworkConfig_EnableMultusCNI(t *testing.T) {
   cfg := &NetworkConfig{IPPool: &IPPoolConfig{}} // Ensure IPPool is not nil for SetDefaults_NetworkConfig
   SetDefaults_NetworkConfig(cfg) // Defaults Multus.Enabled to false
   if cfg.EnableMultusCNI() != false {t.Error("EnableMultusCNI default failed")}

   cfg.Multus.Enabled = boolPtr(true)
   if cfg.EnableMultusCNI() != true {t.Error("EnableMultusCNI true failed")}
}

func TestCalicoConfig_TyphaHelpers(t *testing.T) {
   cfg := &CalicoConfig{}
   SetDefaults_CalicoConfig(cfg, "", nil) // Pass nil for globalDefaultBlockSize
   if cfg.IsTyphaEnabled() != false {t.Error("IsTyphaEnabled default failed")}
   if cfg.GetTyphaReplicas() != 0 {t.Error("GetTyphaReplicas default for disabled Typha failed")}

   cfg.EnableTypha = boolPtr(true)
   SetDefaults_CalicoConfig(cfg, "", nil) // Pass nil for globalDefaultBlockSize
   if cfg.IsTyphaEnabled() != true {t.Error("IsTyphaEnabled true failed")}
   if cfg.GetTyphaReplicas() != 2 {t.Errorf("GetTyphaReplicas default for enabled Typha failed, got %d", cfg.GetTyphaReplicas())}

   cfg.TyphaReplicas = intPtr(5)
   if cfg.GetTyphaReplicas() != 5 {t.Errorf("GetTyphaReplicas custom failed, got %d", cfg.GetTyphaReplicas())}
}

// --- Tests for CiliumConfig Defaulting and Validation ---

func TestSetDefaults_CiliumConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *CiliumConfig
		expected *CiliumConfig
	}{
		{
			name:  "nil input",
			input: nil,
			expected: nil,
		},
		{
			name:  "empty input",
			input: &CiliumConfig{},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				EnableHubble:           false,
				HubbleUI:               false,
				EnableBPFMasquerade:    false,
				IdentityAllocationMode: "crd",
			},
		},
		{
			name: "HubbleUI true, EnableHubble false",
			input: &CiliumConfig{
				HubbleUI:     true,
				EnableHubble: false, // This should trigger EnableHubble to true
			},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				EnableHubble:           true, // Expected to be true
				HubbleUI:               true,
				EnableBPFMasquerade:    false,
				IdentityAllocationMode: "crd",
			},
		},
		{
			name: "HubbleUI true, EnableHubble true",
			input: &CiliumConfig{
				HubbleUI:     true,
				EnableHubble: true,
			},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    false,
				IdentityAllocationMode: "crd",
			},
		},
		{
			name: "partial input, e.g. only TunnelingMode set",
			input: &CiliumConfig{
				TunnelingMode: "geneve",
			},
			expected: &CiliumConfig{
				TunnelingMode:          "geneve",
				KubeProxyReplacement:   "strict",
				EnableHubble:           false,
				HubbleUI:               false,
				EnableBPFMasquerade:    false,
				IdentityAllocationMode: "crd",
			},
		},
		{
			name: "all fields explicitly set by user",
			input: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    true,
				IdentityAllocationMode: "kvstore",
			},
			expected: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    true,
				IdentityAllocationMode: "kvstore",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_CiliumConfig(tt.input)
			if tt.expected == nil {
				if tt.input != nil {
					t.Errorf("Expected nil, got %v", tt.input)
				}
			} else {
				if tt.input == nil {
					t.Errorf("Expected %v, got nil", tt.expected)
				} else {
					if tt.input.TunnelingMode != tt.expected.TunnelingMode {
						t.Errorf("TunnelingMode: got %s, want %s", tt.input.TunnelingMode, tt.expected.TunnelingMode)
					}
					if tt.input.KubeProxyReplacement != tt.expected.KubeProxyReplacement {
						t.Errorf("KubeProxyReplacement: got %s, want %s", tt.input.KubeProxyReplacement, tt.expected.KubeProxyReplacement)
					}
					if tt.input.EnableHubble != tt.expected.EnableHubble {
						t.Errorf("EnableHubble: got %v, want %v", tt.input.EnableHubble, tt.expected.EnableHubble)
					}
					if tt.input.HubbleUI != tt.expected.HubbleUI {
						t.Errorf("HubbleUI: got %v, want %v", tt.input.HubbleUI, tt.expected.HubbleUI)
					}
					if tt.input.EnableBPFMasquerade != tt.expected.EnableBPFMasquerade {
						t.Errorf("EnableBPFMasquerade: got %v, want %v", tt.input.EnableBPFMasquerade, tt.expected.EnableBPFMasquerade)
					}
					if tt.input.IdentityAllocationMode != tt.expected.IdentityAllocationMode {
						t.Errorf("IdentityAllocationMode: got %s, want %s", tt.input.IdentityAllocationMode, tt.expected.IdentityAllocationMode)
					}
				}
			}
		})
	}
}

func TestValidate_CiliumConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       *CiliumConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil input",
			input:       nil,
			expectError: false,
		},
		{
			name: "valid empty (after defaults)",
			input: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
			},
			expectError: false,
		},
		{
			name: "invalid TunnelingMode",
			input: &CiliumConfig{TunnelingMode: "invalid"},
			expectError: true,
			errorMsg:    "cilium.tunnelingMode: invalid mode 'invalid'",
		},
		{
			name: "invalid KubeProxyReplacement",
			input: &CiliumConfig{KubeProxyReplacement: "invalid"},
			expectError: true,
			errorMsg:    "cilium.kubeProxyReplacement: invalid mode 'invalid'",
		},
		{
			name: "HubbleUI true, EnableHubble false",
			input: &CiliumConfig{HubbleUI: true, EnableHubble: false},
			expectError: true,
			errorMsg:    "cilium.hubbleUI: inconsistent state: hubbleUI is true but enableHubble is false. Defaulting should ensure enableHubble is true when hubbleUI is true.",
		},
		{
			name: "valid HubbleUI true, EnableHubble true",
			input: &CiliumConfig{EnableHubble: true, HubbleUI: true},
			expectError: false,
		},
		{
			name: "invalid IdentityAllocationMode",
			input: &CiliumConfig{IdentityAllocationMode: "invalid"},
			expectError: true,
			errorMsg:    "cilium.identityAllocationMode: invalid mode 'invalid'",
		},
		{
			name: "all fields valid",
			input: &CiliumConfig{
				TunnelingMode:          "geneve",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    true,
				IdentityAllocationMode: "kvstore",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			Validate_CiliumConfig(tt.input, verrs, "cilium")
			if tt.expectError {
				if verrs.IsEmpty() {
					t.Errorf("Expected validation errors but got none")
				}
				if tt.errorMsg != "" {
					found := false
					for _, errStr := range verrs.Errors {
						if strings.Contains(errStr, tt.errorMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message substring '%s' not found in errors: %v", tt.errorMsg, verrs.Errors)
					}
				}
			} else {
				if !verrs.IsEmpty() {
					t.Errorf("Expected no validation errors, but got: %v", verrs.Errors)
				}
			}
		})
	}
}

func TestValidate_NetworkConfig_Calls_Validate_CiliumConfig(t *testing.T) {
	cfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{
			TunnelingMode: "invalid-mode", // Invalid value
		},
		KubePodsCIDR:    "10.244.0.0/16", // Valid KubePodsCIDR
		KubeServiceCIDR: "10.96.0.0/12",  // Valid KubeServiceCIDR
	}
	// No need to call SetDefaults_NetworkConfig here if we are testing validation logic path
	// and CiliumConfig is explicitly provided.

	verrs := &ValidationErrors{}
	Validate_NetworkConfig(cfg, verrs, "spec.network", nil) // k8sSpec can be nil

	if verrs.IsEmpty() {
		t.Fatal("Expected validation errors from CiliumConfig via NetworkConfig, but got none.")
	}

	expectedErrorSubstring := "spec.network.cilium.tunnelingMode: invalid mode 'invalid-mode'"
	found := false
	for _, errStr := range verrs.Errors {
		if strings.Contains(errStr, expectedErrorSubstring) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error substring '%s' not found in errors: %v", expectedErrorSubstring, verrs.Errors)
	}
}

func TestSetDefaults_NetworkConfig_Calls_SetDefaults_CiliumConfig(t *testing.T) {
	cfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{}, // Empty CiliumConfig
		IPPool: &IPPoolConfig{}, // Ensure IPPool is not nil, as SetDefaults_NetworkConfig accesses it
	}
	SetDefaults_NetworkConfig(cfg)

	if cfg.Cilium == nil {
		t.Fatal("CiliumConfig should have been initialized by SetDefaults_NetworkConfig.")
	}
	if cfg.Cilium.TunnelingMode != "vxlan" {
		t.Errorf("CiliumConfig TunnelingMode default was not applied: got %s, want vxlan", cfg.Cilium.TunnelingMode)
	}
	if cfg.Cilium.KubeProxyReplacement != "strict" {
		t.Errorf("CiliumConfig KubeProxyReplacement default was not applied: got %s, want strict", cfg.Cilium.KubeProxyReplacement)
	}
}
