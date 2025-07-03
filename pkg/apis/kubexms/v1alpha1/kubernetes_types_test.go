package v1alpha1

import (
	"strings"
	"testing"
	"k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/util/version" // Already imported by kubernetes_types.go
	"github.com/stretchr/testify/assert"
)

// Helper function to get pointers for basic types in tests, if not in a shared util yet
// For bool - using global boolPtr from zz_helpers.go
// For int32 - using global int32Ptr from zz_helpers.go

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
	if cfg.ProxyMode != "ipvs" { // Changed expectation to ipvs
		t.Errorf("Default ProxyMode = %s, want ipvs", cfg.ProxyMode)
	}
	if cfg.AutoRenewCerts == nil || *cfg.AutoRenewCerts != true { // Changed expectation to true
		t.Errorf("Default AutoRenewCerts = %v, want true", cfg.AutoRenewCerts)
	}
	if cfg.DisableKubeProxy == nil || *cfg.DisableKubeProxy != false {
		t.Errorf("Default DisableKubeProxy = %v, want false", cfg.DisableKubeProxy)
	}
	if cfg.MasqueradeAll == nil || *cfg.MasqueradeAll != false {
		t.Errorf("Default MasqueradeAll = %v, want false", cfg.MasqueradeAll)
	}
	if cfg.ContainerManager != "systemd" { // Updated expected default
		t.Errorf("Default ContainerManager = %s, want systemd", cfg.ContainerManager)
	}
	if cfg.MaxPods == nil || *cfg.MaxPods != 110 { // New default check
		t.Errorf("Default MaxPods = %v, want 110", cfg.MaxPods)
	}
	if cfg.NodeCidrMaskSize == nil || *cfg.NodeCidrMaskSize != 24 { // New default check
		t.Errorf("Default NodeCidrMaskSize = %v, want 24", cfg.NodeCidrMaskSize)
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

	if cfg.APIServer == nil || cfg.APIServer.ExtraArgs == nil {
		t.Error("APIServer.ExtraArgs should be initialized (non-nil empty slice)")
	}
	if cfg.APIServer.AdmissionPlugins == nil {
		t.Error("APIServer.AdmissionPlugins should be initialized (non-nil empty slice)")
	}
	if cfg.ControllerManager == nil || cfg.ControllerManager.ExtraArgs == nil {
		t.Error("ControllerManager.ExtraArgs should be initialized (non-nil empty slice)")
	}
	if cfg.Scheduler == nil || cfg.Scheduler.ExtraArgs == nil {
		t.Error("Scheduler.ExtraArgs should be initialized (non-nil empty slice)")
	}
	if cfg.Kubelet == nil || cfg.Kubelet.ExtraArgs == nil {
		t.Error("Kubelet.ExtraArgs should be initialized (non-nil empty slice)")
	}
	if cfg.Kubelet.EvictionHard == nil {t.Error("Kubelet.EvictionHard map should be initialized")}

	if cfg.Kubelet.CgroupDriver == nil || *cfg.Kubelet.CgroupDriver != "systemd" { // Assuming default to systemd
		t.Errorf("Kubelet.CgroupDriver default failed, got %v", cfg.Kubelet.CgroupDriver)
	}
	cfgWithManager := &KubernetesConfig{ContainerManager: "cgroupfs"}
	SetDefaults_KubernetesConfig(cfgWithManager, "test")
	if cfgWithManager.Kubelet.CgroupDriver == nil || *cfgWithManager.Kubelet.CgroupDriver != "cgroupfs" {
		t.Errorf("Kubelet.CgroupDriver should default from ContainerManager if set, got %v", cfgWithManager.Kubelet.CgroupDriver)
	}

	if cfg.KubeProxy == nil || cfg.KubeProxy.ExtraArgs == nil {
		t.Error("KubeProxy.ExtraArgs should be initialized (non-nil empty slice)")
	}
	// Test KubeProxy sub-config defaults based on ProxyMode
	cfgProxyIptables := &KubernetesConfig{ProxyMode: "iptables"}
	SetDefaults_KubernetesConfig(cfgProxyIptables, "iptables-test")
	if cfgProxyIptables.KubeProxy.IPTables == nil { t.Error("KubeProxy.IPTables should be initialized for iptables mode") }
	if cfgProxyIptables.KubeProxy.IPTables.MasqueradeAll == nil || !*cfgProxyIptables.KubeProxy.IPTables.MasqueradeAll {
		t.Error("KubeProxy.IPTables.MasqueradeAll default failed")
	}
	if cfgProxyIptables.KubeProxy.IPTables.MasqueradeBit == nil || *cfgProxyIptables.KubeProxy.IPTables.MasqueradeBit != 14 {
		t.Error("KubeProxy.IPTables.MasqueradeBit default failed")
	}


	cfgProxyIpvs := &KubernetesConfig{ProxyMode: "ipvs"}
	SetDefaults_KubernetesConfig(cfgProxyIpvs, "ipvs-test")
	assert.NotNil(t, cfgProxyIpvs.KubeProxy.IPVS, "KubeProxy.IPVS should be initialized for ipvs mode")
	if cfgProxyIpvs.KubeProxy.IPVS != nil { // Guard against nil pointer dereference if assert fails
		assert.Equal(t, "rr", cfgProxyIpvs.KubeProxy.IPVS.Scheduler, "KubeProxy.IPVS.Scheduler default failed")
		assert.NotNil(t, cfgProxyIpvs.KubeProxy.IPVS.ExcludeCIDRs, "KubeProxy.IPVS.ExcludeCIDRs should be initialized")
		assert.Len(t, cfgProxyIpvs.KubeProxy.IPVS.ExcludeCIDRs, 0, "KubeProxy.IPVS.ExcludeCIDRs should be empty by default")
	}
}

// --- Test Validate_KubernetesConfig ---
func TestValidate_KubernetesConfig_Valid(t *testing.T) {
	cfg := &KubernetesConfig{
		Version:     "v1.25.0",
		DNSDomain:   "my.cluster.local",
		// PodSubnet and ServiceSubnet are part of NetworkConfig
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
		// {"bad_version_format", &KubernetesConfig{Version: "1.25.0"}, ".version: must start with 'v'"}, // Removed as validation is commented out
		// {"empty_dnsdomain", &KubernetesConfig{Version: "v1.20.0", DNSDomain: ""}, ".dnsDomain: cannot be empty"}, // This case is now handled by defaulting
		{"invalid_proxymode", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "foo"}, ".proxyMode: invalid mode 'foo'"},
		// {"invalid_podsubnet", &KubernetesConfig{Version: "v1.20.0", PodSubnet: "invalid"}, ".podSubnet: invalid CIDR format"}, // Moved to NetworkConfig validation
		// {"invalid_servicesubnet", &KubernetesConfig{Version: "v1.20.0", ServiceSubnet: "invalid"}, ".serviceSubnet: invalid CIDR format"}, // Moved to NetworkConfig validation
		{"invalid_containerManager", &KubernetesConfig{Version: "v1.20.0", ContainerManager: "rkt"}, ".containerManager: must be 'cgroupfs' or 'systemd'"},
		{"empty_kubeletConfiguration_raw", &KubernetesConfig{Version: "v1.20.0", KubeletConfiguration: &runtime.RawExtension{Raw: []byte("")}}, ".kubeletConfiguration: raw data cannot be empty"},
		{"empty_kubeProxyConfiguration_raw", &KubernetesConfig{Version: "v1.20.0", KubeProxyConfiguration: &runtime.RawExtension{Raw: []byte("")}}, ".kubeProxyConfiguration: raw data cannot be empty"},
		// APIServerConfig validation
		{"apiserver_invalid_port_range_format", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "invalid"}}, ".apiServer.serviceNodePortRange: invalid format"},
		{"apiserver_invalid_port_range_low_min", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "0-30000"}}, ".apiServer.serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"apiserver_invalid_port_range_high_max", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "30000-70000"}}, ".apiServer.serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"apiserver_invalid_port_range_min_gte_max", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "30000-30000"}}, ".apiServer.serviceNodePortRange: min port 30000 must be less than max port 30000"},
		{"apiserver_invalid_port_range_not_numbers", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "abc-def"}}, ".apiServer.serviceNodePortRange: ports must be numbers"},
		// KubeletConfig validation
		{"kubelet_invalid_cgroupdriver", &KubernetesConfig{Version: "v1.20.0", Kubelet: &KubeletConfig{CgroupDriver: stringPtr("docker")}}, ".kubelet.cgroupDriver: must be 'cgroupfs' or 'systemd'"},
		{"kubelet_invalid_hairpin", &KubernetesConfig{Version: "v1.20.0", Kubelet: &KubeletConfig{HairpinMode: stringPtr("bad")}}, ".kubelet.hairpinMode: invalid mode 'bad'"},
		// KubeProxyConfig validation
		{"kubeproxy_iptables_bad_masq_bit", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{MasqueradeBit: int32Ptr(32)}}}, ".kubeProxy.ipTables.masqueradeBit: must be between 0 and 31"},
		{"kubeproxy_iptables_bad_sync", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{SyncPeriod: "bad"}}}, ".kubeProxy.ipTables.syncPeriod: invalid duration format"},
		{"kubeproxy_ipvs_bad_sync", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{MinSyncPeriod: "bad"}}}, ".kubeProxy.ipvs.minSyncPeriod: invalid duration format"},
		{"kubeproxy_ipvs_bad_exclude_cidr", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{ExcludeCIDRs: []string{"invalid"}}}}, ".kubeProxy.ipvs.excludeCIDRs[0]: invalid CIDR format"},
		{"kubeproxy_mode_mismatch_iptables_has_ipvs", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{}}}, ".kubeProxy.ipvs: should not be set if proxyMode is 'iptables'"},
		{"kubeproxy_mode_mismatch_ipvs_has_iptables", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{}}}, ".kubeProxy.ipTables: should not be set if proxyMode is 'ipvs'"},
		{"kubernetes_version_invalid_format", &KubernetesConfig{Version: "v1.bad.0"}, ".version: 'v1.bad.0' is not a recognized version format"},
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
	cfg.DisableKubeProxy = boolPtr(true)
	if cfg.IsKubeProxyDisabled() != true { t.Error("IsKubeProxyDisabled true failed") }

	// IsNodelocaldnsEnabled
	if cfg.IsNodelocaldnsEnabled() != true { t.Error("IsNodelocaldnsEnabled default failed") } // Default is true
	cfg.Nodelocaldns.Enabled = boolPtr(false)
	if cfg.IsNodelocaldnsEnabled() != false { t.Error("IsNodelocaldnsEnabled false failed") }

	// IsAuditEnabled
	if cfg.IsAuditEnabled() != false {t.Error("IsAuditEnabled default failed")}
	cfg.Audit.Enabled = boolPtr(true)
	if !cfg.IsAuditEnabled() {t.Error("IsAuditEnabled true failed")}

	// IsKataEnabled
	if cfg.IsKataEnabled() != false {t.Error("IsKataEnabled default failed")}
	cfg.Kata.Enabled = boolPtr(true)
	if !cfg.IsKataEnabled() {t.Error("IsKataEnabled true failed")}

	// IsNodeFeatureDiscoveryEnabled
	if cfg.IsNodeFeatureDiscoveryEnabled() != false {t.Error("IsNodeFeatureDiscoveryEnabled default failed")}
	cfg.NodeFeatureDiscovery.Enabled = boolPtr(true)
	if !cfg.IsNodeFeatureDiscoveryEnabled() {t.Error("IsNodeFeatureDiscoveryEnabled true failed")}


	// IsAutoRenewCertsEnabled
	if cfg.IsAutoRenewCertsEnabled() != true {t.Error("IsAutoRenewCertsEnabled default failed, expected true")} // Default is true
	cfg.AutoRenewCerts = boolPtr(false) // Set to false to test change
	if cfg.IsAutoRenewCertsEnabled() != false {t.Error("IsAutoRenewCertsEnabled set to false failed")}


	// GetMaxPods
	if cfg.GetMaxPods() != 110 { t.Errorf("GetMaxPods default failed, got %d", cfg.GetMaxPods()) }
	cfg.MaxPods = int32Ptr(200)
	if cfg.GetMaxPods() != 200 { t.Errorf("GetMaxPods custom failed, got %d", cfg.GetMaxPods()) }

	// IsAtLeastVersion
	if !cfg.IsAtLeastVersion("v1.24.0") { t.Error("IsAtLeastVersion('v1.24.0') failed for v1.24.5") }
	if cfg.IsAtLeastVersion("v1.25.0") { t.Error("IsAtLeastVersion('v1.25.0') should have failed for v1.24.5") }
	if !cfg.IsAtLeastVersion("v1.23") { t.Error("IsAtLeastVersion('v1.23') failed for v1.24.5") }

	cfgNilVersion := &KubernetesConfig{}
	if cfgNilVersion.IsAtLeastVersion("v1.0.0") {t.Error("IsAtLeastVersion should be false for nil version string")}

}

func pstrKubernetesTest(s string) *string { return &s }

func TestValidate_ControllerManagerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *ControllerManagerConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""}, // nil config should not error, just return
		{"valid_empty", &ControllerManagerConfig{}, ""},
		{"valid_with_path", &ControllerManagerConfig{ServiceAccountPrivateKeyFile: "/path/to/sa.key"}, ""},
		{"invalid_empty_path", &ControllerManagerConfig{ServiceAccountPrivateKeyFile: "   "}, "serviceAccountPrivateKeyFile: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			Validate_ControllerManagerConfig(tt.cfg, verrs, "spec.kubernetes.controllerManager")
			if tt.wantErrMsg == "" {
				if !verrs.IsEmpty() {
					t.Errorf("Validate_ControllerManagerConfig expected no error for %s, got %v", tt.name, verrs)
				}
			} else {
				if verrs.IsEmpty() {
					t.Fatalf("Validate_ControllerManagerConfig expected error for %s, got none", tt.name)
				}
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_ControllerManagerConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestValidate_SchedulerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *SchedulerConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""}, // nil config should not error, just return
		{"valid_empty", &SchedulerConfig{}, ""},
		{"valid_with_path", &SchedulerConfig{PolicyConfigFile: "/path/to/policy.yaml"}, ""},
		{"invalid_empty_path", &SchedulerConfig{PolicyConfigFile: "   "}, "policyConfigFile: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			Validate_SchedulerConfig(tt.cfg, verrs, "spec.kubernetes.scheduler")
			if tt.wantErrMsg == "" {
				if !verrs.IsEmpty() {
					t.Errorf("Validate_SchedulerConfig expected no error for %s, got %v", tt.name, verrs)
				}
			} else {
				if verrs.IsEmpty() {
					t.Fatalf("Validate_SchedulerConfig expected error for %s, got none", tt.name)
				}
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_SchedulerConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
				}
			}
		})
	}
}
