package v1alpha1

import (
	"strings"
	"testing"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// Helper function to get pointers for basic types in tests, if not in a shared util yet
// For bool - using global boolPtr from zz_helpers.go
// For int32 - using global int32Ptr from zz_helpers.go

// --- Test SetDefaults_KubernetesConfig ---
func TestSetDefaults_KubernetesConfig(t *testing.T) {
	cfg := &KubernetesConfig{}
	SetDefaults_KubernetesConfig(cfg, "test-cluster-name")

	assert.Equal(t, "test-cluster-name", cfg.ClusterName, "Default ClusterName mismatch")
	assert.Equal(t, "cluster.local", cfg.DNSDomain, "Default DNSDomain mismatch")
	assert.Equal(t, "ipvs", cfg.ProxyMode, "Default ProxyMode mismatch")
	if assert.NotNil(t, cfg.AutoRenewCerts) {
		assert.True(t, *cfg.AutoRenewCerts, "Default AutoRenewCerts mismatch")
	}
	if assert.NotNil(t, cfg.DisableKubeProxy) {
		assert.False(t, *cfg.DisableKubeProxy, "Default DisableKubeProxy mismatch")
	}
	if assert.NotNil(t, cfg.MasqueradeAll) {
		assert.False(t, *cfg.MasqueradeAll, "Default MasqueradeAll mismatch")
	}
	assert.Equal(t, "systemd", cfg.ContainerManager, "Default ContainerManager mismatch")
	if assert.NotNil(t, cfg.MaxPods) {
		assert.Equal(t, int32(110), *cfg.MaxPods, "Default MaxPods mismatch")
	}
	if assert.NotNil(t, cfg.NodeCidrMaskSize) {
		assert.Equal(t, int32(24), *cfg.NodeCidrMaskSize, "Default NodeCidrMaskSize mismatch")
	}

	if assert.NotNil(t, cfg.Nodelocaldns) && assert.NotNil(t, cfg.Nodelocaldns.Enabled) {
		assert.True(t, *cfg.Nodelocaldns.Enabled, "Nodelocaldns default mismatch")
	}
	if assert.NotNil(t, cfg.Audit) && assert.NotNil(t, cfg.Audit.Enabled) {
		assert.False(t, *cfg.Audit.Enabled, "Audit default mismatch")
	}
	if assert.NotNil(t, cfg.Kata) && assert.NotNil(t, cfg.Kata.Enabled) {
		assert.False(t, *cfg.Kata.Enabled, "Kata default mismatch")
	}
	if assert.NotNil(t, cfg.NodeFeatureDiscovery) && assert.NotNil(t, cfg.NodeFeatureDiscovery.Enabled) {
		assert.False(t, *cfg.NodeFeatureDiscovery.Enabled, "NodeFeatureDiscovery default mismatch")
	}

	assert.NotNil(t, cfg.FeatureGates, "FeatureGates map should be initialized")

	if assert.NotNil(t, cfg.APIServer) {
		assert.NotNil(t, cfg.APIServer.ExtraArgs, "APIServer.ExtraArgs should be initialized")
		assert.Contains(t, cfg.APIServer.ExtraArgs, "--profiling=false")
		assert.Contains(t, cfg.APIServer.ExtraArgs, "--anonymous-auth=false")
		assert.Equal(t, "30000-32767", cfg.APIServer.ServiceNodePortRange, "APIServer.ServiceNodePortRange default mismatch")
	}

	assert.NotNil(t, cfg.ControllerManager, "ControllerManager should be initialized")
	if cfg.ControllerManager != nil {
		assert.NotNil(t, cfg.ControllerManager.ExtraArgs, "ControllerManager.ExtraArgs should be initialized")
		assert.Contains(t, cfg.ControllerManager.ExtraArgs, "--profiling=false")
		assert.Contains(t, cfg.ControllerManager.ExtraArgs, "--bind-address=127.0.0.1")
	}

	assert.NotNil(t, cfg.Scheduler, "Scheduler should be initialized")
	if cfg.Scheduler != nil {
		assert.NotNil(t, cfg.Scheduler.ExtraArgs, "Scheduler.ExtraArgs should be initialized")
		assert.Contains(t, cfg.Scheduler.ExtraArgs, "--profiling=false")
		assert.Contains(t, cfg.Scheduler.ExtraArgs, "--bind-address=127.0.0.1")
	}

	if assert.NotNil(t, cfg.Kubelet) {
		assert.NotNil(t, cfg.Kubelet.ExtraArgs, "Kubelet.ExtraArgs should be initialized")
		assert.Contains(t, cfg.Kubelet.ExtraArgs, "--anonymous-auth=false")
		assert.Contains(t, cfg.Kubelet.ExtraArgs, "--read-only-port=0")
		assert.NotNil(t, cfg.Kubelet.EvictionHard, "Kubelet.EvictionHard map should be initialized")
		expectedEvictionHard := map[string]string{
			"memory.available":  "100Mi",
			"nodefs.available":  "10%",
			"imagefs.available": "15%",
			"nodefs.inodesFree": "5%",
		}
		assert.Equal(t, expectedEvictionHard, cfg.Kubelet.EvictionHard, "Kubelet.EvictionHard defaults mismatch")
		if assert.NotNil(t, cfg.Kubelet.CgroupDriver) {
			assert.Equal(t, "systemd", *cfg.Kubelet.CgroupDriver, "Kubelet.CgroupDriver default mismatch")
		}
	}

	cfgWithManager := &KubernetesConfig{ContainerManager: "cgroupfs"}
	SetDefaults_KubernetesConfig(cfgWithManager, "test")
	if assert.NotNil(t, cfgWithManager.Kubelet) && assert.NotNil(t, cfgWithManager.Kubelet.CgroupDriver) {
		assert.Equal(t, "cgroupfs", *cfgWithManager.Kubelet.CgroupDriver, "Kubelet.CgroupDriver should default from ContainerManager")
	}

	if assert.NotNil(t, cfg.KubeProxy) {
		assert.NotNil(t, cfg.KubeProxy.ExtraArgs, "KubeProxy.ExtraArgs should be initialized")
	}

	cfgProxyIptables := &KubernetesConfig{ProxyMode: "iptables"}
	SetDefaults_KubernetesConfig(cfgProxyIptables, "iptables-test")
	if assert.NotNil(t, cfgProxyIptables.KubeProxy) && assert.NotNil(t, cfgProxyIptables.KubeProxy.IPTables) {
		iptablesCfg := cfgProxyIptables.KubeProxy.IPTables
		if assert.NotNil(t, iptablesCfg.MasqueradeAll) {
			assert.False(t, *iptablesCfg.MasqueradeAll, "KubeProxy.IPTables.MasqueradeAll default failed")
		}
		if assert.NotNil(t, iptablesCfg.MasqueradeBit) {
			assert.Equal(t, int32(14), *iptablesCfg.MasqueradeBit, "KubeProxy.IPTables.MasqueradeBit default failed")
		}
		assert.Equal(t, "30s", iptablesCfg.SyncPeriod, "KubeProxy.IPTables.SyncPeriod default failed")
		assert.Equal(t, "15s", iptablesCfg.MinSyncPeriod, "KubeProxy.IPTables.MinSyncPeriod default failed")
	}

	cfgProxyIpvs := &KubernetesConfig{ProxyMode: "ipvs"}
	SetDefaults_KubernetesConfig(cfgProxyIpvs, "ipvs-test")
	if assert.NotNil(t, cfgProxyIpvs.KubeProxy) && assert.NotNil(t, cfgProxyIpvs.KubeProxy.IPVS) {
		ipvsCfg := cfgProxyIpvs.KubeProxy.IPVS
		assert.Equal(t, "rr", ipvsCfg.Scheduler, "KubeProxy.IPVS.Scheduler default failed")
		assert.NotNil(t, ipvsCfg.ExcludeCIDRs, "KubeProxy.IPVS.ExcludeCIDRs should be initialized")
		assert.Len(t, ipvsCfg.ExcludeCIDRs, 0, "KubeProxy.IPVS.ExcludeCIDRs should be empty by default")
		assert.Equal(t, "30s", ipvsCfg.SyncPeriod, "KubeProxy.IPVS.SyncPeriod default failed")
		assert.Equal(t, "15s", ipvsCfg.MinSyncPeriod, "KubeProxy.IPVS.MinSyncPeriod default failed")
	}
}

// --- Test Validate_KubernetesConfig ---
func TestValidate_KubernetesConfig_Valid(t *testing.T) {
	cfg := &KubernetesConfig{
		Version:     "v1.25.0",
		DNSDomain:   "my.cluster.local",
	}
	SetDefaults_KubernetesConfig(cfg, "valid-k8s-cluster")
	verrs := &validation.ValidationErrors{}
	Validate_KubernetesConfig(cfg, verrs, "spec.kubernetes")
	if verrs.HasErrors() {
		t.Errorf("Validate_KubernetesConfig for valid config failed: %v", verrs.Error())
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
		{"invalid_proxymode", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "foo"}, ".proxyMode: invalid mode 'foo'"},
		{"invalid_containerManager", &KubernetesConfig{Version: "v1.20.0", ContainerManager: "rkt"}, ".containerManager: must be one of [systemd cgroupfs]"},
		{"empty_kubeletConfiguration_raw", &KubernetesConfig{Version: "v1.20.0", KubeletConfiguration: &runtime.RawExtension{Raw: []byte("")}}, ".kubeletConfiguration: raw data cannot be empty"},
		{"empty_kubeProxyConfiguration_raw", &KubernetesConfig{Version: "v1.20.0", KubeProxyConfiguration: &runtime.RawExtension{Raw: []byte("")}}, ".kubeProxyConfiguration: raw data cannot be empty"},
		{"apiserver_invalid_port_range_format", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "invalid"}}, ".apiServer.serviceNodePortRange: invalid format"},
		{"apiserver_invalid_port_range_low_min", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "0-30000"}}, ".apiServer.serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"apiserver_invalid_port_range_high_max", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "30000-70000"}}, ".apiServer.serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"apiserver_invalid_port_range_min_gte_max", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "30000-30000"}}, ".apiServer.serviceNodePortRange: min port 30000 must be less than max port 30000"},
		{"apiserver_invalid_port_range_not_numbers", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{ServiceNodePortRange: "abc-def"}}, ".apiServer.serviceNodePortRange: ports must be numbers"},
		{"apiserver_empty_admission_plugin", &KubernetesConfig{Version: "v1.20.0", APIServer: &APIServerConfig{AdmissionPlugins: []string{"ValidPlugin", " "}}}, ".apiServer.admissionPlugins[1]: admission plugin name cannot be empty"},
		{"kubelet_invalid_cgroupdriver", &KubernetesConfig{Version: "v1.20.0", Kubelet: &KubeletConfig{CgroupDriver: stringPtr("docker")}}, ".kubelet.cgroupDriver: must be one of [systemd cgroupfs] if specified"},
		{"kubelet_invalid_hairpin", &KubernetesConfig{Version: "v1.20.0", Kubelet: &KubeletConfig{HairpinMode: stringPtr("bad")}}, ".kubelet.hairpinMode: invalid mode 'bad'"},
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
			if tt.cfg != nil {
			   SetDefaults_KubernetesConfig(tt.cfg, "test-cluster")
			}
			verrs := &validation.ValidationErrors{}
			Validate_KubernetesConfig(tt.cfg, verrs, "spec.kubernetes")
			if !verrs.HasErrors() {
				t.Fatalf("Validate_KubernetesConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_KubernetesConfig error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestKubernetesConfig_Helpers(t *testing.T) {
	cfg := &KubernetesConfig{Version: "v1.24.5"}
	SetDefaults_KubernetesConfig(cfg, "test")

	if cfg.IsKubeProxyDisabled() != false { t.Error("IsKubeProxyDisabled default failed") }
	cfg.DisableKubeProxy = boolPtr(true)
	if cfg.IsKubeProxyDisabled() != true { t.Error("IsKubeProxyDisabled true failed") }

	if cfg.IsNodelocaldnsEnabled() != true { t.Error("IsNodelocaldnsEnabled default failed") }
	cfg.Nodelocaldns.Enabled = boolPtr(false)
	if cfg.IsNodelocaldnsEnabled() != false { t.Error("IsNodelocaldnsEnabled false failed") }

	if cfg.IsAuditEnabled() != false {t.Error("IsAuditEnabled default failed")}
	cfg.Audit.Enabled = boolPtr(true)
	if !cfg.IsAuditEnabled() {t.Error("IsAuditEnabled true failed")}

	if cfg.IsKataEnabled() != false {t.Error("IsKataEnabled default failed")}
	cfg.Kata.Enabled = boolPtr(true)
	if !cfg.IsKataEnabled() {t.Error("IsKataEnabled true failed")}

	if cfg.IsNodeFeatureDiscoveryEnabled() != false {t.Error("IsNodeFeatureDiscoveryEnabled default failed")}
	cfg.NodeFeatureDiscovery.Enabled = boolPtr(true)
	if !cfg.IsNodeFeatureDiscoveryEnabled() {t.Error("IsNodeFeatureDiscoveryEnabled true failed")}

	if cfg.IsAutoRenewCertsEnabled() != true {t.Error("IsAutoRenewCertsEnabled default failed, expected true")}
	cfg.AutoRenewCerts = boolPtr(false)
	if cfg.IsAutoRenewCertsEnabled() != false {t.Error("IsAutoRenewCertsEnabled set to false failed")}

	if cfg.GetMaxPods() != 110 { t.Errorf("GetMaxPods default failed, got %d", cfg.GetMaxPods()) }
	cfg.MaxPods = int32Ptr(200)
	if cfg.GetMaxPods() != 200 { t.Errorf("GetMaxPods custom failed, got %d", cfg.GetMaxPods()) }

	if !cfg.IsAtLeastVersion("v1.24.0") { t.Error("IsAtLeastVersion('v1.24.0') failed for v1.24.5") }
	if cfg.IsAtLeastVersion("v1.25.0") { t.Error("IsAtLeastVersion('v1.25.0') should have failed for v1.24.5") }
	if !cfg.IsAtLeastVersion("v1.23") { t.Error("IsAtLeastVersion('v1.23') failed for v1.24.5") }

	cfgNilVersion := &KubernetesConfig{}
	if cfgNilVersion.IsAtLeastVersion("v1.0.0") {t.Error("IsAtLeastVersion should be false for nil version string")}
}

func TestValidate_ControllerManagerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *ControllerManagerConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"valid_empty", &ControllerManagerConfig{}, ""},
		{"valid_with_path", &ControllerManagerConfig{ServiceAccountPrivateKeyFile: "/path/to/sa.key"}, ""},
		{"invalid_empty_path", &ControllerManagerConfig{ServiceAccountPrivateKeyFile: "   "}, "serviceAccountPrivateKeyFile: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			Validate_ControllerManagerConfig(tt.cfg, verrs, "spec.kubernetes.controllerManager")
			if tt.wantErrMsg == "" {
				if verrs.HasErrors() {
					t.Errorf("Validate_ControllerManagerConfig expected no error for %s, got %v", tt.name, verrs.Error())
				}
			} else {
				if !verrs.HasErrors() {
					t.Fatalf("Validate_ControllerManagerConfig expected error for %s, got none", tt.name)
				}
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_ControllerManagerConfig error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
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
		{"nil_config", nil, ""},
		{"valid_empty", &SchedulerConfig{}, ""},
		{"valid_with_path", &SchedulerConfig{PolicyConfigFile: "/path/to/policy.yaml"}, ""},
		{"invalid_empty_path", &SchedulerConfig{PolicyConfigFile: "   "}, "policyConfigFile: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			Validate_SchedulerConfig(tt.cfg, verrs, "spec.kubernetes.scheduler")
			if tt.wantErrMsg == "" {
				if verrs.HasErrors() {
					t.Errorf("Validate_SchedulerConfig expected no error for %s, got %v", tt.name, verrs.Error())
				}
			} else {
				if !verrs.HasErrors() {
					t.Fatalf("Validate_SchedulerConfig expected error for %s, got none", tt.name)
				}
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_SchedulerConfig error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
				}
			}
		})
	}
}
