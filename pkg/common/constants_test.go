package common

import (
	"testing"
	"time" // Import time package
)

func TestDefaultConstants(t *testing.T) {
	if DefaultAddonNamespace != "kube-system" {
		t.Errorf("DefaultAddonNamespace constant is incorrect: got %s, want kube-system", DefaultAddonNamespace)
	}
}

func TestRegexConstants(t *testing.T) {
	expectedChartRegex := `^v?([0-9]+)(\.[0-9]+){0,2}$`
	if ValidChartVersionRegexString != expectedChartRegex {
		t.Errorf("ValidChartVersionRegexString constant is incorrect: got %s, want %s", ValidChartVersionRegexString, expectedChartRegex)
	}

	expectedDomainRegex := `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`
	if DomainValidationRegexString != expectedDomainRegex {
		t.Errorf("DomainValidationRegexString constant is incorrect: got %s, want %s", DomainValidationRegexString, expectedDomainRegex)
	}
}

func TestKubernetesConstants(t *testing.T) {
	if CgroupDriverSystemd != "systemd" {
		t.Errorf("CgroupDriverSystemd constant is incorrect: got %s, want systemd", CgroupDriverSystemd)
	}
	if CgroupDriverCgroupfs != "cgroupfs" {
		t.Errorf("CgroupDriverCgroupfs constant is incorrect: got %s, want cgroupfs", CgroupDriverCgroupfs)
	}
	if KubeProxyModeIPTables != "iptables" {
		t.Errorf("KubeProxyModeIPTables constant is incorrect: got %s, want iptables", KubeProxyModeIPTables)
	}
	if KubeProxyModeIPVS != "ipvs" {
		t.Errorf("KubeProxyModeIPVS constant is incorrect: got %s, want ipvs", KubeProxyModeIPVS)
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusPending != "Pending" {
		t.Errorf("StatusPending constant is incorrect: got %s, want Pending", StatusPending)
	}
	if StatusProcessing != "Processing" {
		t.Errorf("StatusProcessing constant is incorrect: got %s, want Processing", StatusProcessing)
	}
	if StatusSuccess != "Success" {
		t.Errorf("StatusSuccess constant is incorrect: got %s, want Success", StatusSuccess)
	}
	if StatusFailed != "Failed" {
		t.Errorf("StatusFailed constant is incorrect: got %s, want Failed", StatusFailed)
	}
}

func TestNodeConditionConstants(t *testing.T) {
	if NodeConditionReady != "Ready" {
		t.Errorf("NodeConditionReady constant is incorrect: got %s, want Ready", NodeConditionReady)
	}
}

func TestCNIPluginNames(t *testing.T) {
	if CNICalico != "calico" {
		t.Errorf("CNICalico constant is incorrect: got %s, want calico", CNICalico)
	}
	if CNIFlannel != "flannel" {
		t.Errorf("CNIFlannel constant is incorrect: got %s, want flannel", CNIFlannel)
	}
	if CNICilium != "cilium" {
		t.Errorf("CNICilium constant is incorrect: got %s, want cilium", CNICilium)
	}
	if CNIMultus != "multus" {
		t.Errorf("CNIMultus constant is incorrect: got %s, want multus", CNIMultus)
	}
}

func TestKernelModulesConstants(t *testing.T) {
	if KernelModuleBrNetfilter != "br_netfilter" {
		t.Errorf("KernelModuleBrNetfilter constant is incorrect: got %s, want br_netfilter", KernelModuleBrNetfilter)
	}
	if KernelModuleIpvs != "ip_vs" {
		t.Errorf("KernelModuleIpvs constant is incorrect: got %s, want ip_vs", KernelModuleIpvs)
	}
}

func TestPreflightDefaultsConstants(t *testing.T) {
	if DefaultMinCPUCores != 2 {
		t.Errorf("DefaultMinCPUCores constant is incorrect: got %d, want 2", DefaultMinCPUCores)
	}
	if DefaultMinMemoryMB != 2048 {
		t.Errorf("DefaultMinMemoryMB constant is incorrect: got %d, want 2048", DefaultMinMemoryMB)
	}
}

func TestTimeoutRetryConstants(t *testing.T) {
	if DefaultKubeAPIServerReadyTimeout != 5*time.Minute {
		t.Errorf("DefaultKubeAPIServerReadyTimeout is incorrect")
	}
	if DefaultKubeletReadyTimeout != 3*time.Minute {
		t.Errorf("DefaultKubeletReadyTimeout is incorrect")
	}
	if DefaultEtcdReadyTimeout != 5*time.Minute {
		t.Errorf("DefaultEtcdReadyTimeout is incorrect")
	}
	if DefaultPodReadyTimeout != 5*time.Minute {
		t.Errorf("DefaultPodReadyTimeout is incorrect")
	}
	if DefaultResourceOperationTimeout != 2*time.Minute {
		t.Errorf("DefaultResourceOperationTimeout is incorrect")
	}
	if DefaultTaskRetryAttempts != 3 {
		t.Errorf("DefaultTaskRetryAttempts is incorrect")
	}
	if DefaultTaskRetryDelaySeconds != 10 {
		t.Errorf("DefaultTaskRetryDelaySeconds is incorrect")
	}
}

func TestFilePermissionConstants(t *testing.T) {
	if DefaultDirPermission != 0755 {
		t.Errorf("DefaultDirPermission is incorrect: got %o, want 0755", DefaultDirPermission)
	}
	if DefaultFilePermission != 0644 {
		t.Errorf("DefaultFilePermission is incorrect: got %o, want 0644", DefaultFilePermission)
	}
	if DefaultKubeconfigPermission != 0600 {
		t.Errorf("DefaultKubeconfigPermission is incorrect: got %o, want 0600", DefaultKubeconfigPermission)
	}
	if DefaultPrivateKeyPermission != 0600 {
		t.Errorf("DefaultPrivateKeyPermission is incorrect: got %o, want 0600", DefaultPrivateKeyPermission)
	}
}

func TestIPProtocolConstants(t *testing.T) {
	if IPProtocolIPv4 != "IPv4" {
		t.Errorf("IPProtocolIPv4 is incorrect")
	}
	if IPProtocolIPv6 != "IPv6" {
		t.Errorf("IPProtocolIPv6 is incorrect")
	}
	if IPProtocolDualStack != "DualStack" {
		t.Errorf("IPProtocolDualStack is incorrect")
	}
}

func TestPlaceholderValueConstants(t *testing.T) {
	if ValueAuto != "auto" {
		t.Errorf("ValueAuto is incorrect")
	}
	if ValueDefault != "default" {
		t.Errorf("ValueDefault is incorrect")
	}
}

func TestCacheKeyConstants(t *testing.T) { // Added test for cache keys
	if CacheKeyHostFactsPrefix != "facts.host." {
		t.Errorf("CacheKeyHostFactsPrefix constant is incorrect: got %s", CacheKeyHostFactsPrefix)
	}
	if CacheKeyClusterCACert != "pki.ca.cert" {
		t.Errorf("CacheKeyClusterCACert constant is incorrect: got %s", CacheKeyClusterCACert)
	}
	if CacheKeyClusterCAKey != "pki.ca.key" {
		t.Errorf("CacheKeyClusterCAKey constant is incorrect: got %s", CacheKeyClusterCAKey)
	}
}
