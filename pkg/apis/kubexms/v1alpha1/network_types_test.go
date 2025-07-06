package v1alpha1

import (
	"strings"
	"testing"
	// "net" // No longer needed here as TestNetworksOverlap was moved
	"github.com/mensylisir/kubexm/pkg/util/validation"
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
	if cfgCalico.Calico.VethMTU == nil || *cfgCalico.Calico.VethMTU != 0 {t.Errorf("Calico VethMTU default failed, got %v, want 0", cfgCalico.Calico.VethMTU)}


	// Test Flannel defaults when plugin is Flannel
	cfgFlannel := &NetworkConfig{Plugin: "flannel"}
	SetDefaults_NetworkConfig(cfgFlannel)
	if cfgFlannel.Flannel == nil { t.Fatal("Flannel config should be initialized when plugin is flannel") }
	if cfgFlannel.Flannel.BackendMode != "vxlan" { t.Errorf("Flannel BackendMode default failed") }

	// Test KubeOvn defaults when enabled
	cfgKubeOvn := &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgKubeOvn)
	if cfgKubeOvn.KubeOvn == nil { t.Fatal("KubeOvn config should be initialized for plugin kubeovn") }
	if cfgKubeOvn.KubeOvn.Label == nil || *cfgKubeOvn.KubeOvn.Label != "kube-ovn/role" { t.Errorf("KubeOvn Label default failed: %v", cfgKubeOvn.KubeOvn.Label) }
	if cfgKubeOvn.KubeOvn.TunnelType == nil || *cfgKubeOvn.KubeOvn.TunnelType != "geneve" { t.Errorf("KubeOvn TunnelType default failed: %v", cfgKubeOvn.KubeOvn.TunnelType) }
	if cfgKubeOvn.KubeOvn.EnableSSL == nil || *cfgKubeOvn.KubeOvn.EnableSSL != false { t.Errorf("KubeOvn EnableSSL default failed: %v", cfgKubeOvn.KubeOvn.EnableSSL) }

	// Test Hybridnet defaults when enabled
	cfgHybridnet := &NetworkConfig{Plugin: "hybridnet", Hybridnet: &HybridnetConfig{Enabled: boolPtr(true)}, IPPool: &IPPoolConfig{}}
	SetDefaults_NetworkConfig(cfgHybridnet)
	if cfgHybridnet.Hybridnet == nil { t.Fatal("Hybridnet config should be initialized for plugin hybridnet") }
	if cfgHybridnet.Hybridnet.DefaultNetworkType == nil || *cfgHybridnet.Hybridnet.DefaultNetworkType != "Overlay" { t.Errorf("Hybridnet DefaultNetworkType default failed: %v", cfgHybridnet.Hybridnet.DefaultNetworkType) }
	if cfgHybridnet.Hybridnet.EnableNetworkPolicy == nil || !*cfgHybridnet.Hybridnet.EnableNetworkPolicy { t.Errorf("Hybridnet EnableNetworkPolicy default failed: %v", cfgHybridnet.Hybridnet.EnableNetworkPolicy) }
	if cfgHybridnet.Hybridnet.InitDefaultNetwork == nil || !*cfgHybridnet.Hybridnet.InitDefaultNetwork { t.Errorf("Hybridnet InitDefaultNetwork default failed: %v", cfgHybridnet.Hybridnet.InitDefaultNetwork) }

	cfgCalicoWithPool := &NetworkConfig{Plugin: "calico", KubePodsCIDR: "192.168.0.0/16", Calico: &CalicoConfig{DefaultIPPOOL: boolPtr(true), IPPools: []CalicoIPPool{}}, IPPool: &IPPoolConfig{}}
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
					DefaultIPPOOL: boolPtr(true),
					IPPools:       []CalicoIPPool{},
				},
				IPPool: &IPPoolConfig{BlockSize: intPtr(26)},
			}
			SetDefaults_NetworkConfig(cfg)
			if len(cfg.Calico.IPPools) != 1 {
				t.Fatalf("Expected 1 default IPPool, got %d for case %s", len(cfg.Calico.IPPools), tc.name)
			}
			if cfg.Calico.IPPools[0].Encapsulation != tc.expectedEncap {
				t.Errorf("Case %s: Default IPPool Encapsulation = %s, want %s", tc.name, cfg.Calico.IPPools[0].Encapsulation, tc.expectedEncap)
			}
		})
	}
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
	verrs := &validation.ValidationErrors{}
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
		{"kubeovn_invalid_tunneltype", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true), TunnelType: stringPtr("bad")}}, ".kubeovn.tunnelType: invalid type 'bad'"},
		{"kubeovn_invalid_joincidr", &NetworkConfig{Plugin: "kubeovn", KubePodsCIDR: "10.244.0.0/16", KubeOvn: &KubeOvnConfig{Enabled: boolPtr(true), JoinCIDR: stringPtr("invalid")}}, ".kubeovn.joinCIDR: invalid CIDR format"},
		{"hybridnet_invalid_networktype", &NetworkConfig{Plugin: "hybridnet", KubePodsCIDR: "10.244.0.0/16", Hybridnet: &HybridnetConfig{Enabled: boolPtr(true), DefaultNetworkType: stringPtr("bad")}}, ".hybridnet.defaultNetworkType: invalid type 'bad'"},
		{"calico_invalid_logseverity", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{LogSeverityScreen: stringPtr("trace")}}, ".calico.logSeverityScreen: invalid: 'trace'"},
		{"calico_ippool_empty_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: ""}}}}, ".calico.ipPools[0:p1].cidr: cannot be empty"},
		{"calico_ippool_invalid_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: "invalid"}}}}, ".calico.ipPools[0:p1].cidr: invalid CIDR 'invalid'"},
		{"calico_ippool_bad_blocksize_low", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: intPtr(19)}}}}, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_blocksize_high", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: intPtr(33)}}}}, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_encap", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",Encapsulation: "bad"}}}}, ".calico.ipPools[0:p1].encapsulation: invalid: 'bad'"},
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

			verrs := &validation.ValidationErrors{}
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

func TestNetworkConfig_EnableMultusCNI(t *testing.T) {
   cfg := &NetworkConfig{IPPool: &IPPoolConfig{}}
   SetDefaults_NetworkConfig(cfg)
   if cfg.EnableMultusCNI() != false {t.Error("EnableMultusCNI default failed")}

   cfg.Multus.Enabled = boolPtr(true)
   if cfg.EnableMultusCNI() != true {t.Error("EnableMultusCNI true failed")}
}

func TestCalicoConfig_TyphaHelpers(t *testing.T) {
   cfg := &CalicoConfig{}
   SetDefaults_CalicoConfig(cfg, "", nil)
   if cfg.IsTyphaEnabled() != false {t.Error("IsTyphaEnabled default failed")}
   if cfg.GetTyphaReplicas() != 0 {t.Error("GetTyphaReplicas default for disabled Typha failed")}

   cfg.EnableTypha = boolPtr(true)
   SetDefaults_CalicoConfig(cfg, "", nil)
   if cfg.IsTyphaEnabled() != true {t.Error("IsTyphaEnabled true failed")}
   if cfg.GetTyphaReplicas() != 2 {t.Errorf("GetTyphaReplicas default for enabled Typha failed, got %d", cfg.GetTyphaReplicas())}

   cfg.TyphaReplicas = intPtr(5)
   if cfg.GetTyphaReplicas() != 5 {t.Errorf("GetTyphaReplicas custom failed, got %d", cfg.GetTyphaReplicas())}
}

func TestValidate_NetworkConfig_Calls_Validate_CiliumConfig(t *testing.T) {
	k8sCfgMinimal := &KubernetesConfig{Version: "v1.25.0"}
	SetDefaults_KubernetesConfig(k8sCfgMinimal, "test-cluster")

	cfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{
			TunnelingMode: "invalid-mode",
		},
		KubePodsCIDR:    "10.244.0.0/16",
		KubeServiceCIDR: "10.96.0.0/12",
	}

	verrs := &validation.ValidationErrors{}
	Validate_NetworkConfig(cfg, verrs, "spec.network", k8sCfgMinimal) // Pass k8sCfgMinimal

	if !verrs.HasErrors() { // Updated
		t.Fatal("Expected validation errors from CiliumConfig via NetworkConfig, but got none.")
	}

	expectedErrorSubstring := "spec.network.cilium.tunnelingMode: invalid mode 'invalid-mode'"
	found := false
	// Error() returns a single string with errors separated by \n
	for _, errStr := range strings.Split(verrs.Error(), "\n") { // Updated
		if strings.Contains(errStr, expectedErrorSubstring) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error substring '%s' not found in errors: %v", expectedErrorSubstring, verrs.Error()) // Updated
	}
}

func TestSetDefaults_NetworkConfig_Calls_SetDefaults_CiliumConfig(t *testing.T) {
	cfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{},
		IPPool: &IPPoolConfig{},
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
