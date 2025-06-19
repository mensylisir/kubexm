package v1alpha1

import (
	"strings"
	"testing"
	// "k8s.io/apimachinery/pkg/util/version" // Already imported by kubernetes_types.go
)

// Helper function to get pointers for basic types in tests, if not in a shared util yet
// For bool
func pboolKubernetesTest(b bool) *bool { return &b }
// For int32
func pint32KubernetesTest(i int32) *int32 { return &i }


// --- Test SetDefaults_KubernetesConfig ---
func TestSetDefaults_KubernetesConfig(t *testing.T) {
	cfg := &KubernetesConfig{}
	SetDefaults_KubernetesConfig(cfg, "test-cluster-name")

	if cfg.ClusterName != "test-cluster-name" {
		t.Errorf("Default ClusterName = %s, want test-cluster-name", cfg.ClusterName)
	}
	if cfg.DNSDomain != "cluster.local" {
		t.Errorf("Default DNSDomain = %s, want cluster.local", cfg.DNSDomain)
	}
	if cfg.ProxyMode != "iptables" {
		t.Errorf("Default ProxyMode = %s, want iptables", cfg.ProxyMode)
	}
	if cfg.AutoRenewCerts == nil || *cfg.AutoRenewCerts != false {
		t.Errorf("Default AutoRenewCerts = %v, want false", cfg.AutoRenewCerts)
	}
	if cfg.DisableKubeProxy == nil || *cfg.DisableKubeProxy != false {
		t.Errorf("Default DisableKubeProxy = %v, want false", cfg.DisableKubeProxy)
	}
	if cfg.MasqueradeAll == nil || *cfg.MasqueradeAll != false {
		t.Errorf("Default MasqueradeAll = %v, want false", cfg.MasqueradeAll)
	}

	// Check sub-configs are initialized and defaulted
	if cfg.Nodelocaldns == nil || cfg.Nodelocaldns.Enabled == nil || *cfg.Nodelocaldns.Enabled != true {
		t.Errorf("Nodelocaldns default = %v, want enabled=true", cfg.Nodelocaldns)
	}
	if cfg.Audit == nil || cfg.Audit.Enabled == nil || *cfg.Audit.Enabled != false {
		t.Errorf("Audit default = %v, want enabled=false", cfg.Audit)
	}
	if cfg.Kata == nil || cfg.Kata.Enabled == nil || *cfg.Kata.Enabled != false {
		t.Errorf("Kata default = %v, want enabled=false", cfg.Kata)
	}
	if cfg.NodeFeatureDiscovery == nil || cfg.NodeFeatureDiscovery.Enabled == nil || *cfg.NodeFeatureDiscovery.Enabled != false {
		t.Errorf("NodeFeatureDiscovery default = %v, want enabled=false", cfg.NodeFeatureDiscovery)
	}

	if cfg.FeatureGates == nil { t.Error("FeatureGates map should be initialized") }
	if cfg.APIServer == nil || cfg.APIServer.ExtraArgs == nil { t.Error("APIServer or ExtraArgs map should be initialized") }
	// ... similar checks for ControllerManager, Scheduler, Kubelet, KubeProxy ExtraArgs ...
}

// --- Test Validate_KubernetesConfig ---
func TestValidate_KubernetesConfig_Valid(t *testing.T) {
	cfg := &KubernetesConfig{
		Version:     "v1.25.0",
		DNSDomain:   "my.cluster.local",
		PodSubnet:   "10.244.0.0/16",
		ServiceSubnet: "10.96.0.0/12",
	}
	SetDefaults_KubernetesConfig(cfg, "valid-k8s-cluster") // Apply defaults
	verrs := &ValidationErrors{}
	Validate_KubernetesConfig(cfg, verrs, "spec.kubernetes")
	if !verrs.IsEmpty() {
		t.Errorf("Validate_KubernetesConfig for valid config failed: %v", verrs)
	}
}

func TestValidate_KubernetesConfig_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *KubernetesConfig
		wantErrMsg string
	}{
		{"nil_config", nil, "kubernetes configuration section cannot be nil"},
		{"empty_version", &KubernetesConfig{Version: ""}, ".version: cannot be empty"},
		{"bad_version_format", &KubernetesConfig{Version: "1.25.0"}, ".version: must start with 'v'"},
		{"empty_dnsdomain", &KubernetesConfig{Version: "v1.20.0", DNSDomain: ""}, ".dnsDomain: cannot be empty"},
		{"invalid_proxymode", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "foo"}, ".proxyMode: invalid mode 'foo'"},
		{"invalid_podsubnet", &KubernetesConfig{Version: "v1.20.0", PodSubnet: "invalid"}, ".podSubnet: invalid CIDR format"},
		{"invalid_servicesubnet", &KubernetesConfig{Version: "v1.20.0", ServiceSubnet: "invalid"}, ".serviceSubnet: invalid CIDR format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// SetDefaults might be called or not depending on whether the field being tested is affected by defaults.
			// For nil_config, SetDefaults won't be called.
			if tt.cfg != nil {
			   SetDefaults_KubernetesConfig(tt.cfg, "test-cluster")
			}
			verrs := &ValidationErrors{}
			Validate_KubernetesConfig(tt.cfg, verrs, "spec.kubernetes")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_KubernetesConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_KubernetesConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}

// --- Test KubernetesConfig Helper Methods ---
func TestKubernetesConfig_Helpers(t *testing.T) {
	cfg := &KubernetesConfig{Version: "v1.24.5"}
	SetDefaults_KubernetesConfig(cfg, "test") // Apply defaults

	// IsKubeProxyDisabled
	if cfg.IsKubeProxyDisabled() != false { t.Error("IsKubeProxyDisabled default failed") }
	cfg.DisableKubeProxy = pboolKubernetesTest(true)
	if cfg.IsKubeProxyDisabled() != true { t.Error("IsKubeProxyDisabled true failed") }

	// IsNodelocaldnsEnabled
	if cfg.IsNodelocaldnsEnabled() != true { t.Error("IsNodelocaldnsEnabled default failed") } // Default is true
	cfg.Nodelocaldns.Enabled = pboolKubernetesTest(false)
	if cfg.IsNodelocaldnsEnabled() != false { t.Error("IsNodelocaldnsEnabled false failed") }

	// IsAuditEnabled
	if cfg.IsAuditEnabled() != false {t.Error("IsAuditEnabled default failed")}
	cfg.Audit.Enabled = pboolKubernetesTest(true)
	if !cfg.IsAuditEnabled() {t.Error("IsAuditEnabled true failed")}

	// IsKataEnabled
	if cfg.IsKataEnabled() != false {t.Error("IsKataEnabled default failed")}
	cfg.Kata.Enabled = pboolKubernetesTest(true)
	if !cfg.IsKataEnabled() {t.Error("IsKataEnabled true failed")}

	// IsNodeFeatureDiscoveryEnabled
	if cfg.IsNodeFeatureDiscoveryEnabled() != false {t.Error("IsNodeFeatureDiscoveryEnabled default failed")}
	cfg.NodeFeatureDiscovery.Enabled = pboolKubernetesTest(true)
	if !cfg.IsNodeFeatureDiscoveryEnabled() {t.Error("IsNodeFeatureDiscoveryEnabled true failed")}


	// IsAutoRenewCertsEnabled
	if cfg.IsAutoRenewCertsEnabled() != false {t.Error("IsAutoRenewCertsEnabled default failed")}
	cfg.AutoRenewCerts = pboolKubernetesTest(true)
	if !cfg.IsAutoRenewCertsEnabled() {t.Error("IsAutoRenewCertsEnabled true failed")}

	// GetMaxPods
	if cfg.GetMaxPods() != 110 { t.Errorf("GetMaxPods default failed, got %d", cfg.GetMaxPods()) }
	cfg.MaxPods = pint32KubernetesTest(200)
	if cfg.GetMaxPods() != 200 { t.Errorf("GetMaxPods custom failed, got %d", cfg.GetMaxPods()) }

	// IsAtLeastVersion
	if !cfg.IsAtLeastVersion("v1.24.0") { t.Error("IsAtLeastVersion('v1.24.0') failed for v1.24.5") }
	if cfg.IsAtLeastVersion("v1.25.0") { t.Error("IsAtLeastVersion('v1.25.0') should have failed for v1.24.5") }
	if !cfg.IsAtLeastVersion("v1.23") { t.Error("IsAtLeastVersion('v1.23') failed for v1.24.5") }

	cfgNilVersion := &KubernetesConfig{}
	if cfgNilVersion.IsAtLeastVersion("v1.0.0") {t.Error("IsAtLeastVersion should be false for nil version string")}

}
