package v1alpha1

import (
	"strings"
	"testing"
)

// Helper function to get pointers for basic types in tests
func pboolNetworkTest(b bool) *bool { return &b }
func pint32NetworkTest(i int32) *int32 { return &i }
// func pstrNetworkTest(s string) *string { return &s } // If needed

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
	if cfgCalico.Calico.VethMTU == nil || *cfgCalico.Calico.VethMTU != 1440 {t.Error("Calico VethMTU default failed")}


	// Test Flannel defaults when plugin is Flannel
	cfgFlannel := &NetworkConfig{Plugin: "flannel"}
	SetDefaults_NetworkConfig(cfgFlannel)
	if cfgFlannel.Flannel == nil { t.Fatal("Flannel config should be initialized when plugin is flannel") }
	if cfgFlannel.Flannel.BackendMode != "vxlan" { t.Errorf("Flannel BackendMode default failed") }
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
