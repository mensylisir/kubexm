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
		if assert.NotNil(t, cfg.Kubelet.HairpinMode) {
			assert.Equal(t, "promiscuous-bridge", *cfg.Kubelet.HairpinMode, "Kubelet.HairpinMode default mismatch")
		}
	}

	// Test Kubelet HairpinMode with user-defined value (should not be overridden by default)
	userDefinedHairpin := "hairpin-veth"
	cfgUserHairpin := &KubernetesConfig{Kubelet: &KubeletConfig{HairpinMode: &userDefinedHairpin}}
	SetDefaults_KubernetesConfig(cfgUserHairpin, "test-hairpin")
	if assert.NotNil(t, cfgUserHairpin.Kubelet) && assert.NotNil(t, cfgUserHairpin.Kubelet.HairpinMode) {
		assert.Equal(t, userDefinedHairpin, *cfgUserHairpin.Kubelet.HairpinMode, "Kubelet.HairpinMode should not be overridden if user-defined")
	}


	// Test APIServer AdmissionPlugins defaults
	cfgAdmission := &KubernetesConfig{}
	SetDefaults_KubernetesConfig(cfgAdmission, "test-admission")
	expectedDefaultPlugins := []string{
		"NodeRestriction", "NamespaceLifecycle", "LimitRanger", "ServiceAccount",
		"DefaultStorageClass", "DefaultTolerationSeconds", "MutatingAdmissionWebhook",
		"ValidatingAdmissionWebhook", "ResourceQuota",
	}
	if assert.NotNil(t, cfgAdmission.APIServer) {
		assert.ElementsMatch(t, expectedDefaultPlugins, cfgAdmission.APIServer.AdmissionPlugins, "APIServer.AdmissionPlugins default mismatch")
	}

	// Test APIServer AdmissionPlugins with user-provided empty list (should be filled with defaults)
	cfgAdmissionUserEmpty := &KubernetesConfig{APIServer: &APIServerConfig{AdmissionPlugins: []string{}}}
	SetDefaults_KubernetesConfig(cfgAdmissionUserEmpty, "test-admission-user-empty")
	if assert.NotNil(t, cfgAdmissionUserEmpty.APIServer) {
		assert.ElementsMatch(t, expectedDefaultPlugins, cfgAdmissionUserEmpty.APIServer.AdmissionPlugins, "APIServer.AdmissionPlugins should be default if user provided empty list")
	}

	// Test APIServer AdmissionPlugins with some user-provided plugins (defaults should be appended if not present)
	userPlugins := []string{"MyCustomPlugin", "NodeRestriction"} // NodeRestriction is a default one
	cfgAdmissionUserPartial := &KubernetesConfig{APIServer: &APIServerConfig{AdmissionPlugins: userPlugins}}
	SetDefaults_KubernetesConfig(cfgAdmissionUserPartial, "test-admission-user-partial")
	expectedMergedPlugins := []string{
		"MyCustomPlugin", "NodeRestriction", "NamespaceLifecycle", "LimitRanger", "ServiceAccount",
		"DefaultStorageClass", "DefaultTolerationSeconds", "MutatingAdmissionWebhook",
		"ValidatingAdmissionWebhook", "ResourceQuota",
	}
	if assert.NotNil(t, cfgAdmissionUserPartial.APIServer) {
		assert.ElementsMatch(t, expectedMergedPlugins, cfgAdmissionUserPartial.APIServer.AdmissionPlugins, "APIServer.AdmissionPlugins merging logic failed")
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

func TestValidate_KubeletConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *KubeletConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""}, // nil config should be handled by caller
		{"valid_empty", &KubeletConfig{}, ""}, // Defaults will be applied before validation typically
		{"valid_full", &KubeletConfig{
			CgroupDriver: stringPtr("systemd"),
			HairpinMode:  stringPtr("promiscuous-bridge"),
			PodPidsLimit: int64Ptr(20000),
			EvictionHard: map[string]string{"memory.available": "50Mi"},
		}, ""},
		{"invalid_cgroupdriver", &KubeletConfig{CgroupDriver: stringPtr("docker")}, ".cgroupDriver: must be one of [systemd cgroupfs] if specified"},
		{"valid_cgroupdriver_empty_for_default", &KubeletConfig{CgroupDriver: stringPtr("")}, ""}, // Empty string is not in validKubeletCgroupDrivers, but if default is applied it would be fine
		{"invalid_hairpin", &KubeletConfig{HairpinMode: stringPtr("bad-mode")}, ".hairpinMode: invalid mode 'bad-mode'"},
		{"valid_hairpin_empty_for_default", &KubeletConfig{HairpinMode: stringPtr("")}, ""}, // Empty string is allowed for default by validKubeletHairpinModes
		{"invalid_podPidsLimit_zero", &KubeletConfig{PodPidsLimit: int64Ptr(0)}, ".podPidsLimit: must be positive or -1 (unlimited)"},
		{"invalid_podPidsLimit_negative_not_-1", &KubeletConfig{PodPidsLimit: int64Ptr(-10)}, ".podPidsLimit: must be positive or -1 (unlimited)"},
		{"valid_podPidsLimit_-1", &KubeletConfig{PodPidsLimit: int64Ptr(-1)}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults for cases where an empty struct is passed,
			// as validation often happens after defaults.
			if tt.cfg != nil && tt.name == "valid_empty" { // Specific case for empty struct
				SetDefaults_KubeletConfig(tt.cfg, "systemd") // Provide a typical containerManager for defaults
			}
			// For "valid_cgroupdriver_empty_for_default" and "valid_hairpin_empty_for_default",
			// the validation itself allows empty string if it's part of the valid values list (which it is for hairpin).
            // CgroupDriver's default is applied from KubernetesConfig.ContainerManager if Kubelet.CgroupDriver is nil.
            // If it's an empty string explicitly, it might fail unless "" is in `validKubeletCgroupDrivers`.
            // Current `validKubeletCgroupDrivers` = []string{common.CgroupDriverSystemd, common.CgroupDriverCgroupfs}
            // So, an explicit empty string for CgroupDriver in KubeletConfig *will* fail validation if not nil.
            // Let's adjust "valid_cgroupdriver_empty_for_default" to expect failure or make it nil to test defaulting.
            if tt.name == "valid_cgroupdriver_empty_for_default" {
                 // This test case as originally conceived for an empty string *value* is actually invalid
                 // because "" is not in `validKubeletCgroupDrivers`.
                 // If the intention was to test `nil` CgroupDriver field leading to default, that's a SetDefaults test.
                 // Let's assume it's testing an *explicit* but invalid empty string.
                 tt.wantErrMsg = ".cgroupDriver: must be one of [systemd cgroupfs] if specified"
            }


			verrs := &validation.ValidationErrors{}
			Validate_KubeletConfig(tt.cfg, verrs, "spec.kubernetes.kubelet")

			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Validate_KubeletConfig expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Validate_KubeletConfig expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Validate_KubeletConfig error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
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
		// APIServer specific errors are now in TestValidate_APIServerConfig
		// Kubelet specific errors are now in TestValidate_KubeletConfig
		{"kubeproxy_iptables_bad_masq_bit", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{MasqueradeBit: int32Ptr(32)}}}, ".kubeProxy.ipTables.masqueradeBit: must be between 0 and 31"},
		{"kubeproxy_iptables_bad_sync", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{SyncPeriod: "bad"}}}, ".kubeProxy.ipTables.syncPeriod: invalid duration format"},
		{"kubeproxy_ipvs_bad_sync", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{MinSyncPeriod: "bad"}}}, ".kubeProxy.ipvs.minSyncPeriod: invalid duration format"},
		{"kubeproxy_ipvs_bad_exclude_cidr", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{ExcludeCIDRs: []string{"invalid"}}}}, ".kubeProxy.ipvs.excludeCIDRs[0]: invalid CIDR format"},
		{"kubeproxy_mode_mismatch_iptables_has_ipvs", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "iptables", KubeProxy: &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{}}}, ".kubeProxy.ipvs: should not be set if proxyMode is 'iptables'"},
		{"kubeproxy_mode_mismatch_ipvs_has_iptables", &KubernetesConfig{Version: "v1.20.0", ProxyMode: "ipvs", KubeProxy: &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{}}}, ".kubeProxy.ipTables: should not be set if proxyMode is 'ipvs'"},
		{"kubernetes_version_invalid_format", &KubernetesConfig{Version: "v1.bad.0"}, ".version: 'v1.bad.0' is not a recognized version format"},
		{"apiserverCertExtraSans_empty_entry", &KubernetesConfig{Version: "v1.20.0", ApiserverCertExtraSans: []string{" "}}, ".apiserverCertExtraSans[0]: SAN entry cannot be empty"},
		{"apiserverCertExtraSans_invalid_dns", &KubernetesConfig{Version: "v1.20.0", ApiserverCertExtraSans: []string{"-invalid.dns-"}}, ".apiserverCertExtraSans[0]: invalid SAN entry '-invalid.dns-'"},
		{"apiserverCertExtraSans_invalid_ip", &KubernetesConfig{Version: "v1.20.0", ApiserverCertExtraSans: []string{"999.999.999.999"}}, ".apiserverCertExtraSans[0]: invalid SAN entry '999.999.999.999'"},
		{"apiserverCertExtraSans_valid_and_invalid", &KubernetesConfig{Version: "v1.20.0", ApiserverCertExtraSans: []string{"example.com", "1.2.3.4", "-invalid-"}}, ".apiserverCertExtraSans[2]: invalid SAN entry '-invalid-'"},
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

func TestValidate_APIServerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *APIServerConfig
		wantErrMsg string // Expect this substring in the error message
	}{
		{"nil_config", nil, ""}, // nil config should be handled by caller, Validate_APIServerConfig should not panic
		{"valid_empty", &APIServerConfig{}, ""},
		{"valid_full", &APIServerConfig{
			EtcdServers:          []string{"http://etcd1:2379"},
			EtcdCAFile:           "/etc/kubernetes/pki/etcd/ca.crt",
			EtcdCertFile:         "/etc/kubernetes/pki/apiserver-etcd-client.crt",
			EtcdKeyFile:          "/etc/kubernetes/pki/apiserver-etcd-client.key",
			AdmissionPlugins:     []string{"NodeRestriction", "ResourceQuota"},
			ServiceNodePortRange: "30000-32000",
		}, ""},
		{"invalid_serviceNodePortRange_format", &APIServerConfig{ServiceNodePortRange: "invalid"}, ".serviceNodePortRange: invalid format 'invalid', expected 'min-max'"},
		{"invalid_serviceNodePortRange_low_min", &APIServerConfig{ServiceNodePortRange: "0-30000"}, ".serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"invalid_serviceNodePortRange_high_max", &APIServerConfig{ServiceNodePortRange: "30000-70000"}, ".serviceNodePortRange: port numbers must be between 1 and 65535"},
		{"invalid_serviceNodePortRange_min_gte_max", &APIServerConfig{ServiceNodePortRange: "30000-30000"}, ".serviceNodePortRange: min port 30000 must be less than max port 30000"},
		{"invalid_serviceNodePortRange_not_numbers", &APIServerConfig{ServiceNodePortRange: "abc-def"}, ".serviceNodePortRange: ports must be numbers"},
		{"empty_admission_plugin", &APIServerConfig{AdmissionPlugins: []string{"ValidPlugin", " "}}, ".admissionPlugins[1]: admission plugin name cannot be empty"},
		{"etcdServers_empty_entry", &APIServerConfig{EtcdServers: []string{"http://etcd1:2379", " "}}, ".etcdServers[1]: etcd server entry cannot be empty"},
		{"etcdCAFile_whitespace", &APIServerConfig{EtcdCAFile: "   "}, ".etcdCAFile: cannot be only whitespace if specified"},
		{"etcdCertFile_whitespace", &APIServerConfig{EtcdCertFile: "   "}, ".etcdCertFile: cannot be only whitespace if specified"},
		{"etcdKeyFile_whitespace", &APIServerConfig{EtcdKeyFile: "   "}, ".etcdKeyFile: cannot be only whitespace if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			// Note: ApiserverCertExtraSans is part of KubernetesConfig, so it's tested in TestValidate_KubernetesConfig_Invalid
			Validate_APIServerConfig(tt.cfg, verrs, "spec.kubernetes.apiServer")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Validate_APIServerConfig expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Validate_APIServerConfig expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Validate_APIServerConfig error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
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

func TestValidate_KubeProxyIPTablesConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *KubeProxyIPTablesConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"valid_empty", &KubeProxyIPTablesConfig{}, ""}, // Defaults are applied before validation
		{"valid_full", &KubeProxyIPTablesConfig{MasqueradeAll: boolPtr(true), MasqueradeBit: int32Ptr(16), SyncPeriod: "60s", MinSyncPeriod: "30s"}, ""},
		{"invalid_masqueradeBit_low", &KubeProxyIPTablesConfig{MasqueradeBit: int32Ptr(-1)}, ".masqueradeBit: must be between 0 and 31"},
		{"invalid_masqueradeBit_high", &KubeProxyIPTablesConfig{MasqueradeBit: int32Ptr(32)}, ".masqueradeBit: must be between 0 and 31"},
		{"invalid_syncPeriod", &KubeProxyIPTablesConfig{SyncPeriod: "bad-duration"}, ".syncPeriod: invalid duration format"},
		{"invalid_minSyncPeriod", &KubeProxyIPTablesConfig{MinSyncPeriod: "1minute"}, ".minSyncPeriod: invalid duration format"}, // Example of a valid unit but not parsed by time.ParseDuration without number
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg != nil && tt.name == "valid_empty" {
				SetDefaults_KubeProxyIPTablesConfig(tt.cfg)
			}
			verrs := &validation.ValidationErrors{}
			Validate_KubeProxyIPTablesConfig(tt.cfg, verrs, "iptables")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidate_KubeProxyIPVSConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *KubeProxyIPVSConfig
		wantErrMsg string
	}{
		{"nil_config", nil, ""},
		{"valid_empty", &KubeProxyIPVSConfig{}, ""}, // Defaults are applied
		{"valid_full", &KubeProxyIPVSConfig{Scheduler: "wlc", SyncPeriod: "1m", MinSyncPeriod: "30s", ExcludeCIDRs: []string{"10.0.0.0/24"}}, ""},
		{"invalid_syncPeriod", &KubeProxyIPVSConfig{SyncPeriod: "still-bad"}, ".syncPeriod: invalid duration format"},
		{"invalid_minSyncPeriod", &KubeProxyIPVSConfig{MinSyncPeriod: "1hour"}, ".minSyncPeriod: invalid duration format"},
		{"invalid_excludeCIDR", &KubeProxyIPVSConfig{ExcludeCIDRs: []string{"not-a-cidr"}}, ".excludeCIDRs[0]: invalid CIDR format"},
		{"valid_multiple_excludeCIDRs", &KubeProxyIPVSConfig{ExcludeCIDRs: []string{"192.168.1.0/24", "10.10.0.0/16"}}, ""},
		{"invalid_multiple_excludeCIDRs_one_bad", &KubeProxyIPVSConfig{ExcludeCIDRs: []string{"192.168.1.0/24", "bad"}}, ".excludeCIDRs[1]: invalid CIDR format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg != nil && tt.name == "valid_empty" {
				SetDefaults_KubeProxyIPVSConfig(tt.cfg)
			}
			verrs := &validation.ValidationErrors{}
			Validate_KubeProxyIPVSConfig(tt.cfg, verrs, "ipvs")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestValidate_KubeProxyConfig(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *KubeProxyConfig
		parentProxyMode string
		wantErrMsg      string
	}{
		{"nil_config", nil, "iptables", ""},
		{"valid_iptables_mode_with_iptables_config", &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{}}, "iptables", ""},
		{"valid_ipvs_mode_with_ipvs_config", &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{}}, "ipvs", ""},
		{"invalid_iptables_mode_with_ipvs_config", &KubeProxyConfig{IPVS: &KubeProxyIPVSConfig{}}, "iptables", ".ipvs: should not be set if proxyMode is 'iptables'"},
		{"invalid_ipvs_mode_with_iptables_config", &KubeProxyConfig{IPTables: &KubeProxyIPTablesConfig{}}, "ipvs", ".ipTables: should not be set if proxyMode is 'ipvs'"},
		{"valid_iptables_mode_with_nil_iptables_config", &KubeProxyConfig{IPTables: nil}, "iptables", ""}, // Defaulting might create it
		{"valid_ipvs_mode_with_nil_ipvs_config", &KubeProxyConfig{IPVS: nil}, "ipvs", ""}, // Defaulting might create it
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate top-level defaulting if cfg is not nil
			// KubeProxyConfig itself doesn't have SetDefaults, it's handled by KubernetesConfig parent
			if tt.cfg != nil {
				if tt.parentProxyMode == "iptables" && tt.cfg.IPTables == nil && tt.cfg.IPVS == nil { // Only if both are nil to simulate initial state
					// If we test a scenario where IPTables should be defaulted, it would be done by parent.
					// For this isolated test, we assume if it's nil, it means it wasn't set by user.
				}
				if tt.parentProxyMode == "ipvs" && tt.cfg.IPVS == nil && tt.cfg.IPTables == nil {
					// Similar to above for IPVS
				}
			}

			verrs := &validation.ValidationErrors{}
			Validate_KubeProxyConfig(tt.cfg, verrs, "kubeproxy", tt.parentProxyMode)
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got %v", tt.name, verrs.Error())
			} else {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
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
