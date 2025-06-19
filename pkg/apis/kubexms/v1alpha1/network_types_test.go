package v1alpha1

import (
	"strings"
	"testing"
)

// Helper function to get pointers for basic types in tests
func pboolNetworkTest(b bool) *bool { return &b }
func pint32NetworkTest(i int32) *int32 { return &i }
func pstrNetworkTest(s string) *string { return &s } // Added
func pintNetworkTest(i int) *int { v := i; return &v } // Added for Calico IPPool BlockSize

// --- Test SetDefaults_NetworkConfig & Sub-configs ---
func TestSetDefaults_NetworkConfig_Overall(t *testing.T) {
	cfg := &NetworkConfig{}
	SetDefaults_NetworkConfig(cfg)
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
	cfgCalico := &NetworkConfig{Plugin: "calico"}
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
	cfgKubeOvn := &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: pboolNetworkTest(true)}}
	SetDefaults_NetworkConfig(cfgKubeOvn) // Should call SetDefaults_KubeOvnConfig
	if cfgKubeOvn.KubeOvn == nil { t.Fatal("KubeOvn config should be initialized for plugin kubeovn") }
	if cfgKubeOvn.KubeOvn.Label == nil || *cfgKubeOvn.KubeOvn.Label != "kube-ovn/role" { t.Errorf("KubeOvn Label default failed: %v", cfgKubeOvn.KubeOvn.Label) }
	if cfgKubeOvn.KubeOvn.TunnelType == nil || *cfgKubeOvn.KubeOvn.TunnelType != "geneve" { t.Errorf("KubeOvn TunnelType default failed: %v", cfgKubeOvn.KubeOvn.TunnelType) }
	if cfgKubeOvn.KubeOvn.EnableSSL == nil || *cfgKubeOvn.KubeOvn.EnableSSL != false { t.Errorf("KubeOvn EnableSSL default failed: %v", cfgKubeOvn.KubeOvn.EnableSSL) }

	// Test Hybridnet defaults when enabled
	cfgHybridnet := &NetworkConfig{Plugin: "hybridnet", Hybridnet: &HybridnetConfig{Enabled: pboolNetworkTest(true)}}
	SetDefaults_NetworkConfig(cfgHybridnet) // Should call SetDefaults_HybridnetConfig
	if cfgHybridnet.Hybridnet == nil { t.Fatal("Hybridnet config should be initialized for plugin hybridnet") }
	if cfgHybridnet.Hybridnet.DefaultNetworkType == nil || *cfgHybridnet.Hybridnet.DefaultNetworkType != "Overlay" { t.Errorf("Hybridnet DefaultNetworkType default failed: %v", cfgHybridnet.Hybridnet.DefaultNetworkType) }
	if cfgHybridnet.Hybridnet.EnableNetworkPolicy == nil || !*cfgHybridnet.Hybridnet.EnableNetworkPolicy { t.Errorf("Hybridnet EnableNetworkPolicy default failed: %v", cfgHybridnet.Hybridnet.EnableNetworkPolicy) }
	if cfgHybridnet.Hybridnet.InitDefaultNetwork == nil || !*cfgHybridnet.Hybridnet.InitDefaultNetwork { t.Errorf("Hybridnet InitDefaultNetwork default failed: %v", cfgHybridnet.Hybridnet.InitDefaultNetwork) }

	// Test Calico Default IPPool creation and LogSeverityScreen
	cfgCalicoWithPool := &NetworkConfig{Plugin: "calico", KubePodsCIDR: "192.168.0.0/16", Calico: &CalicoConfig{DefaultIPPOOL: pboolNetworkTest(true), IPPools: []CalicoIPPool{}}}
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
	k8sCfg := &KubernetesConfig{PodSubnet: "10.244.0.0/16", ServiceSubnet: "10.96.0.0/12"}
	cfg := &NetworkConfig{
		Plugin:          "calico",
		KubePodsCIDR:    "10.244.0.0/16", // Can be same as k8sCfg or different if overriding
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
	k8sCfg := &KubernetesConfig{PodSubnet: "10.244.0.0/16", ServiceSubnet: "10.96.0.0/12"}
	tests := []struct {
		name        string
		cfg         *NetworkConfig
		k8sForTest  *KubernetesConfig // Optional k8s config for specific test cases
		wantErrMsg  string
	}{
		{"nil_config", nil, k8sCfg, "network configuration section cannot be nil"},
		{"empty_pod_cidr_if_k8s_also_empty", &NetworkConfig{Plugin: "calico", KubePodsCIDR: ""}, &KubernetesConfig{PodSubnet:""}, ".kubePodsCIDR: (or kubernetes.podSubnet) cannot be empty"},
		{"invalid_pod_cidr", &NetworkConfig{Plugin: "calico", KubePodsCIDR: "invalid"}, k8sCfg, ".kubePodsCIDR: invalid CIDR format"},
		{"calico_nil_if_plugin_calico", &NetworkConfig{Plugin: "calico", Calico: nil}, k8sCfg, ".calico: calico configuration section cannot be nil"},
		{"calico_invalid_ipip", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPIPMode: "bad"}}, k8sCfg, ".calico.ipipMode: invalid mode 'bad'"},
		{"flannel_nil_if_plugin_flannel", &NetworkConfig{Plugin: "flannel", Flannel: nil}, k8sCfg, ".flannel: flannel configuration section cannot be nil"},
		{"flannel_invalid_backend", &NetworkConfig{Plugin: "flannel", Flannel: &FlannelConfig{BackendMode: "bad"}}, k8sCfg, ".flannel.backendMode: invalid mode 'bad'"},
		{"kubeovn_invalid_tunneltype", &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: pboolNetworkTest(true), TunnelType: pstrNetworkTest("bad")}}, k8sCfg, ".kubeovn.tunnelType: invalid type 'bad'"},
		{"kubeovn_invalid_joincidr", &NetworkConfig{Plugin: "kubeovn", KubeOvn: &KubeOvnConfig{Enabled: pboolNetworkTest(true), JoinCIDR: pstrNetworkTest("invalid")}}, k8sCfg, ".kubeovn.joinCIDR: invalid CIDR format"},
		{"hybridnet_invalid_networktype", &NetworkConfig{Plugin: "hybridnet", Hybridnet: &HybridnetConfig{Enabled: pboolNetworkTest(true), DefaultNetworkType: pstrNetworkTest("bad")}}, k8sCfg, ".hybridnet.defaultNetworkType: invalid type 'bad'"},
		{"calico_invalid_logseverity", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{LogSeverityScreen: pstrNetworkTest("trace")}}, k8sCfg, ".calico.logSeverityScreen: invalid: 'trace'"},
		{"calico_ippool_empty_cidr", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: ""}}}}, k8sCfg, ".calico.ipPools[0:p1].cidr: cannot be empty"},
		{"calico_ippool_invalid_cidr", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name: "p1", CIDR: "invalid"}}}}, k8sCfg, ".calico.ipPools[0:p1].cidr: invalid CIDR 'invalid'"},
		{"calico_ippool_bad_blocksize_low", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: pintNetworkTest(19)}}}}, k8sCfg, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_blocksize_high", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",BlockSize: pintNetworkTest(33)}}}}, k8sCfg, ".calico.ipPools[0:p1].blockSize: must be between 20 and 32"},
		{"calico_ippool_bad_encap", &NetworkConfig{Plugin: "calico", Calico: &CalicoConfig{IPPools: []CalicoIPPool{{Name:"p1",CIDR:"1.1.1.0/24",Encapsulation: "bad"}}}}, k8sCfg, ".calico.ipPools[0:p1].encapsulation: invalid: 'bad'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentK8sCfg := tt.k8sForTest
			if currentK8sCfg == nil { // Use default if not specified by test case
			   currentK8sCfg = k8sCfg
			}
			if tt.cfg != nil { SetDefaults_NetworkConfig(tt.cfg) }

			verrs := &ValidationErrors{}
			Validate_NetworkConfig(tt.cfg, verrs, "spec.network", currentK8sCfg)
			if verrs.IsEmpty() {
				t.Fatalf("Validate_NetworkConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_NetworkConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}

// --- Test NetworkConfig Helper Methods ---
func TestNetworkConfig_EnableMultusCNI(t *testing.T) {
   cfg := &NetworkConfig{}
   SetDefaults_NetworkConfig(cfg) // Defaults Multus.Enabled to false
   if cfg.EnableMultusCNI() != false {t.Error("EnableMultusCNI default failed")}

   cfg.Multus.Enabled = pboolNetworkTest(true)
   if cfg.EnableMultusCNI() != true {t.Error("EnableMultusCNI true failed")}
}

func TestCalicoConfig_TyphaHelpers(t *testing.T) {
   cfg := &CalicoConfig{}
   SetDefaults_CalicoConfig(cfg) // Defaults EnableTypha to false
   if cfg.IsTyphaEnabled() != false {t.Error("IsTyphaEnabled default failed")}
   if cfg.GetTyphaReplicas() != 0 {t.Error("GetTyphaReplicas default for disabled Typha failed")}

   cfg.EnableTypha = pboolNetworkTest(true)
   SetDefaults_CalicoConfig(cfg) // Re-default with Typha enabled to set default replicas
   if cfg.IsTyphaEnabled() != true {t.Error("IsTyphaEnabled true failed")}
   if cfg.GetTyphaReplicas() != 2 {t.Errorf("GetTyphaReplicas default for enabled Typha failed, got %d", cfg.GetTyphaReplicas())}

   cfg.TyphaReplicas = pint32NetworkTest(5)
   if cfg.GetTyphaReplicas() != 5 {t.Errorf("GetTyphaReplicas custom failed, got %d", cfg.GetTyphaReplicas())}
}
