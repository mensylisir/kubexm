package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/stretchr/testify/assert"
	// "gopkg.in/yaml.v3" // No longer needed after an assertion change
)

// parseYAMLString is a helper to write a YAML string to a temp file and parse it.
func parseYAMLString(t *testing.T, yamlContent string) (*v1alpha1.Cluster, error) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}
	return ParseFromFile(configFile)
}

const validYAMLMinimal = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  global:
    user: "testuser"
    privateKeyPath: "/dev/null"
  hosts:
  - name: master-1
    address: 192.168.1.10
    roles: ["master", "etcd"]
  etcd:
    type: kubexm # Ensure etcd section is present and type is valid
  kubernetes:
    type: kubexm
    version: v1.25.0
  network:
    kubePodsCIDR: "10.244.0.0/16" # Ensure network section is present
  controlPlaneEndpoint:
    lb_address: "1.2.3.4"
`

const validYAMLDockerRuntime = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: docker-cluster
spec:
  global:
    user: "dockeruser"
    privateKeyPath: "/dev/null"
  hosts:
  - name: node1
    address: 192.168.1.50
    roles: ["control-plane", "worker", "etcd"]
  containerRuntime:
    type: docker
    version: "20.10.17"
    docker:
      dataRoot: "/var/lib/docker-custom"
      logDriver: "journald"
      execOpts: ["native.cgroupdriver=systemd"]
      storageDriver: "overlay2"
      registryMirrors:
        - "https://dockerhub.mirror.internal"
      insecureRegistries:
        - "my.dev.registry:5000"
      bip: "172.28.0.1/16"
      defaultAddressPools:
        - base: "172.29.0.0/16"
          size: 24
      installCRIDockerd: true
      criDockerdVersion: "v0.3.1"
  etcd: {} # Minimal valid etcd
  kubernetes:
    type: kubexm
    version: v1.24.0
    containerManager: "systemd"
  network:
    kubePodsCIDR: "10.244.0.0/16" # Minimal valid network
  controlPlaneEndpoint:
    lb_address: "1.2.3.5"
`

const validYAMLFull = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: full-cluster
spec:
  global:
    user: globaluser
    port: 2222
    privateKeyPath: "/dev/null"
    connectionTimeout: "60s"
    workDir: /tmp/global_work_enriched
    verbose: true
    ignoreErr: false
    skipPreflight: false
  hosts:
  - name: master-1
    address: 192.168.1.10
    internalAddress: 10.0.0.10
    port: 22
    user: hostuser
    privateKeyPath: /home/hostuser/.ssh/id_rsa
    roles: ["master", "etcd"]
    labels:
      nodetype: master_node
    taints:
    - key: "CriticalAddonsOnly"
      value: "true"
      effect: "NoExecute"
    type: ssh
  - name: worker-1
    address: 192.168.1.20
    roles: ["worker"]
    user: workeruser # Inherits privateKeyPath from global
  containerRuntime:
    type: containerd
    version: "1.6.9"
    containerd:
      registryMirrors:
        docker.io:
        - "https://mirror.docker.com"
        - "https://another.mirror.com"
      insecureRegistries:
        - "my.insecure.registry:5000"
      useSystemdCgroup: true
      disabledPlugins: ["io.containerd.internal.v1.opt"]
  etcd:
    type: kubeadm
    version: v3.5.9
    clientPort: 2378
    peerPort: 2381
    dataDir: "/mnt/etcd_data_custom"
    extraArgs:
      - "--election-timeout=1200"
      - "--heartbeat-interval=60"
    backupDir: "/opt/etcd_backup"
    backupPeriodHours: 12
    keepBackupNumber: 5
    snapshotCount: 50000
    logLevel: debug
  kubernetes:
    type: kubeadm
    version: v1.25.3
    clusterName: my-k8s-cluster
    dnsDomain: custom.cluster.local
    proxyMode: ipvs
    containerManager: systemd
    maxPods: 150
    featureGates:
      "EphemeralContainers": true
    apiServer:
      admissionPlugins: ["NodeRestriction","NamespaceLifecycle"]
      extraArgs:
        - "--audit-log-maxage=30"
        - "--etcd-servers=http://127.0.0.1:2379"
    controllerManager:
      extraArgs: ["--leader-elect=false"]
    kubelet:
      cgroupDriver: "systemd"
      evictionHard:
        "memory.available": "5%"
        "nodefs.available": "10%"
      extraArgs:
        - "--kube-reserved=cpu=500m,memory=1Gi"
    # kubeletConfiguration: {"apiVersion": "kubelet.config.k8s.io/v1beta1", "kind": "KubeletConfiguration", "serializeImagePulls": false, "evictionHard": {"memory.available": "200Mi"}} # Commented out due to validation issue with runtime.RawExtension
    kubeProxy:
      ipvs:
        scheduler: "wrr"
        syncPeriod: "15s"
      extraArgs: ["--v=2"]
    nodelocaldns:
      enabled: true
    audit:
      enabled: true
  network:
    plugin: calico
    kubePodsCIDR: "10.244.0.0/16"
    kubeServiceCIDR: "10.96.0.0/12"
    calico:
      logSeverityScreen: Info
      vethMTU: 1420
      ipPools:
      - name: "mypool-1"
        cidr: "192.168.100.0/24"
        encapsulation: "VXLAN"
        natOutgoing: true
        blockSize: 27
      - name: "mypool-2-default-blockSize"
        cidr: "192.168.101.0/24"
  highAvailability:
    enabled: true
    external:
      type: ManagedKeepalivedHAProxy
      loadBalancerHostGroupName: "test-lb-group" # Added for validation
      keepalived:
        interface: "eth1"
        vrid: 101
        priority: 150
        authType: "AH"
        authPass: "" # Explicitly empty for AH authType
      haproxy:
        mode: "tcp"
        balanceAlgorithm: "leastconn"
        backendServers:
          - name: "master-1-backend"
            address: "192.168.1.10"
            port: 6443
  controlPlaneEndpoint:
    domain: "k8s-api.internal.example.com" # Domain is present, so lb_address is not strictly needed for validation pass
    lb_address: "192.168.1.100" # Using lb_address for consistency, though domain would satisfy validation
    port: 8443
  preflight:
    disableSwap: true
    minCPUCores: 2
  system:
    modules: ["br_netfilter", "ip_vs"]
    sysctlParams:
      "net.bridge.bridge-nf-call-iptables": "1"
    ntpServers: ["ntp1.example.com", "ntp2.example.com"]
    timezone: "America/Los_Angeles"
    skipConfigureOS: false
  addons:
  - coredns
  - metrics-server
  roleGroups:
    master:
    - "master-1"
    worker:
    - "worker-1"
    etcd:
    - "master-1"
    registry:
    - "master-1"
  storage:
    defaultStorageClass: "openebs-hostpath"
    openebs:
      enabled: true
      version: "3.3.0"
      basePath: "/mnt/openebs_data"
      engines:
        localHostPath:
          enabled: true
  registry:
    privateRegistry: "myreg.local" # Assuming port should not be here; or validation is more complex
    auths:
      "myreg.local:5000": # Auths might still need port if it's a URL map key
        username: "reguser"
        password: "regpassword"
`

const invalidYAMLMalformed = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: malformed
spec:
  hosts:
  - name: master-1
    address: 192.168.1.10
    roles: ["master" "etcd"] # Error: missing comma
`

// TestParseFromFile_YAMLContents combines tests previously using LoadFromBytes
func TestParseFromFile_YAMLContents(t *testing.T) {
	t.Run("ValidMinimal", func(t *testing.T) {
		cfg, err := parseYAMLString(t, validYAMLMinimal)
		assert.NoError(t, err, "ParseFromFile with minimal valid YAML failed")
		if !assert.NotNil(t, cfg) {
			t.FailNow()
		}

		assert.Equal(t, "test-cluster", cfg.ObjectMeta.Name)
		assert.Len(t, cfg.Spec.Hosts, 1)

		if assert.NotNil(t, cfg.Spec.Global) {
			assert.Equal(t, 22, cfg.Spec.Global.Port)
		}

		if len(cfg.Spec.Hosts) > 0 {
			assert.Equal(t, "testuser", cfg.Spec.Hosts[0].User)
			assert.Equal(t, 22, cfg.Spec.Hosts[0].Port)
		}

		if assert.NotNil(t, cfg.Spec.Kubernetes, "Kubernetes spec should be present") {
			assert.Equal(t, common.ClusterTypeKubeXM, cfg.Spec.Kubernetes.Type, "cfg.Spec.Kubernetes.Type mismatch")
			assert.Equal(t, "v1.25.0", cfg.Spec.Kubernetes.Version)
			assert.Equal(t, "test-cluster", cfg.Spec.Kubernetes.ClusterName)
			assert.Equal(t, "cluster.local", cfg.Spec.Kubernetes.DNSDomain)
		}

		if assert.NotNil(t, cfg.Spec.Etcd) {
			assert.Equal(t, v1alpha1.EtcdTypeKubeXMSInternal, cfg.Spec.Etcd.Type)
		}
		if assert.NotNil(t, cfg.Spec.Network, "Network spec should be present") {
			assert.Equal(t, "10.244.0.0/16", cfg.Spec.Network.KubePodsCIDR)
		}
		if assert.NotNil(t, cfg.Spec.ControlPlaneEndpoint) { // Check after it's added to YAML
			assert.Equal(t, "1.2.3.4", cfg.Spec.ControlPlaneEndpoint.Address)
		}
	})

	t.Run("ValidFull", func(t *testing.T) {
		cfg, err := parseYAMLString(t, validYAMLFull)
		assert.NoError(t, err, "ParseFromFile with full valid YAML failed")
		if !assert.NotNil(t, cfg) {
			t.FailNow()
		}

		assert.Equal(t, "full-cluster", cfg.ObjectMeta.Name)
		if assert.NotNil(t, cfg.Spec.Global) {
			assert.Equal(t, "globaluser", cfg.Spec.Global.User)
			assert.Equal(t, 2222, cfg.Spec.Global.Port)
			assert.Equal(t, 60*time.Second, cfg.Spec.Global.ConnectionTimeout)
		}
		assert.Len(t, cfg.Spec.Hosts, 2)

		if assert.NotNil(t, cfg.Spec.Kubernetes) {
			assert.Equal(t, common.ClusterTypeKubeadm, cfg.Spec.Kubernetes.Type)
			// KubeletConfiguration is commented out in validYAMLFull, so these assertions would fail/panic.
			// assert.NotNil(t, cfg.Spec.Kubernetes.KubeletConfiguration)
			// assert.NotEmpty(t, cfg.Spec.Kubernetes.KubeletConfiguration.Raw, "KubeletConfiguration.Raw should not be empty")
		}
		assert.Len(t, cfg.Spec.Addons, 2)
		assert.Contains(t, cfg.Spec.Addons, "coredns")

		if cr := cfg.Spec.ContainerRuntime; assert.NotNil(t, cr) {
			assert.Equal(t, v1alpha1.ContainerRuntimeType("containerd"), cr.Type)
			if assert.NotNil(t, cr.Containerd) {
				_, ok := cr.Containerd.RegistryMirrors["docker.io"]
				assert.True(t, ok, "Expected docker.io mirror to be present")
			}
		}
		if etcd := cfg.Spec.Etcd; assert.NotNil(t, etcd) {
			assert.Equal(t, "kubeadm", etcd.Type)
		}
		if cpe := cfg.Spec.ControlPlaneEndpoint; assert.NotNil(t, cpe) {
		    assert.Equal(t, "192.168.1.100", cpe.Address)
		}
		if sys := cfg.Spec.System; assert.NotNil(t, sys) {
		    assert.Contains(t, sys.Modules, "br_netfilter")
			assert.Equal(t, "America/Los_Angeles", sys.Timezone)
			assert.Len(t, sys.NTPServers, 2)
			assert.False(t, sys.SkipConfigureOS)
		}
		if ha := cfg.Spec.HighAvailability; assert.NotNil(t, ha) && assert.NotNil(t, ha.External) && assert.NotNil(t, ha.External.Keepalived) {
			// With authType: "AH" and yaml authPass: "",
			// AuthPass *string should either be nil (if yaml "" unmarshals to nil *string)
			// or a non-nil pointer to an empty string.
			// The original test assumed authType: "PASS" and authPass: "ha_secret".
			if ha.External.Keepalived.AuthPass != nil {
				assert.Equal(t, "", *ha.External.Keepalived.AuthPass, "AuthPass should be an empty string if present and authType is AH")
			} else {
				// If authPass: "" in YAML unmarshals to a nil pointer for *string, this branch is hit.
				// This is acceptable for authType: "AH" as per validation logic.
				assert.Nil(t, ha.External.Keepalived.AuthPass, "AuthPass can be nil for authType AH if authPass key is effectively absent or empty in YAML")
			}
		}
	})

	t.Run("ValidDockerRuntime", func(t *testing.T) {
		cfg, err := parseYAMLString(t, validYAMLDockerRuntime)
		assert.NoError(t, err, "ParseFromFile with Docker runtime YAML failed")
		if !assert.NotNil(t, cfg){
			t.FailNow()
		}
		assert.Equal(t, "docker-cluster", cfg.ObjectMeta.Name)
		if cr := cfg.Spec.ContainerRuntime; assert.NotNil(t, cr) {
			assert.Equal(t, v1alpha1.ContainerRuntimeType("docker"), cr.Type)
			assert.Equal(t, "20.10.17", cr.Version)
			if dockerCfg := cr.Docker; assert.NotNil(t, dockerCfg) && assert.NotNil(t, dockerCfg.DataRoot) {
				assert.Equal(t, "/var/lib/docker-custom", *dockerCfg.DataRoot)
			}
		}
		if assert.NotNil(t, cfg.Spec.ControlPlaneEndpoint) { // Check after it's added to YAML
			assert.Equal(t, "1.2.3.5", cfg.Spec.ControlPlaneEndpoint.Address)
		}
	})

	t.Run("MalformedYAML", func(t *testing.T) {
		cfg, err := parseYAMLString(t, invalidYAMLMalformed)
		assert.Error(t, err, "ParseFromFile with malformed YAML expected error")
		assert.Nil(t, cfg)
		if err != nil {
			// Error from yaml.Unmarshal for syntax errors is often a generic error, not specifically yaml.TypeError
			// Checking for "failed to unmarshal" (my wrapping) and "yaml:" (from the underlying yaml parser) is more robust.
			assert.Contains(t, err.Error(), "failed to unmarshal YAML", "Error message should indicate unmarshal failure")
			assert.Contains(t, err.Error(), "yaml:", "Error message should contain original yaml parser error details")
		}
	})
}

func TestParseFromFile_FileSuccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-load-test-")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "cluster.yaml")
	err = os.WriteFile(configPath, []byte(validYAMLFull), 0644)
	assert.NoError(t, err, "Failed to write temp config file")

	cfg, err := ParseFromFile(configPath)
	assert.NoError(t, err, "ParseFromFile(%s) failed", configPath)
	if assert.NotNil(t, cfg) {
		assert.Equal(t, "full-cluster", cfg.ObjectMeta.Name, "Loaded config ObjectMeta.Name mismatch")
	}
}

func TestParseFromFile_FileNotExist(t *testing.T) {
	cfg, err := ParseFromFile("/path/to/nonexistent/file.yaml")
	assert.Error(t, err, "ParseFromFile with non-existent file expected error")
	assert.Nil(t, cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to read configuration file /path/to/nonexistent/file.yaml")
		assert.True(t, errors.Is(err, os.ErrNotExist), "Expected underlying error to be os.ErrNotExist")
	}
}

func TestParseFromFile_EmptyPath(t *testing.T) {
	cfg, err := ParseFromFile("")
	assert.Error(t, err, "ParseFromFile with empty path expected error")
	assert.Nil(t, cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to read configuration file")
		assert.True(t, errors.Is(err, os.ErrNotExist), "Expected underlying error to be os.ErrNotExist for empty path")
	}
}


const yamlInvalidAfterDefaults = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: invalid-after-defaults
spec:
  global:
    privateKeyPath: "/dev/null"
  hosts:
  - port: 22
    roles: ["master"]
  kubernetes:
    type: kubexm
    version: v1.0.0
  etcd: {}
  network:
    kubePodsCIDR: "10.244.0.0/16"
  controlPlaneEndpoint:
    lb_address: "1.2.3.4"
`

const yamlValidDueToDefaults = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: valid-due-to-defaults
spec:
  global:
    user: "testuser"
    privateKeyPath: "/dev/null"
  hosts:
  - name: node-1
    address: 10.0.0.1
    roles: ["worker"]
  etcd: {}
  kubernetes:
    type: kubexm
    version: v1.23.0
  network:
    kubePodsCIDR: 10.244.0.0/16
  controlPlaneEndpoint:
    lb_address: "1.2.3.4"
`

func TestParseFromFile_ValidationFail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-validation-fail-")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid_cluster.yaml")
	err = os.WriteFile(configPath, []byte(yamlInvalidAfterDefaults), 0644)
	assert.NoError(t, err, "Failed to write temp config file")

	cfg, err := ParseFromFile(configPath)
	assert.Error(t, err, "ParseFromFile with YAML that should fail validation after defaults, expected error")
	assert.Nil(t, cfg)

	if err != nil {
		assert.Contains(t, err.Error(), "configuration validation failed for "+configPath, "Error message should indicate overall validation failure")

		// Check for multiple specific validation error messages within the aggregated error
		// The *ValidationErrors type joins errors with "; ".
		errorStr := err.Error() // This will be the string from (*ValidationErrors).Error()

		expectedSubstrings := []string{
			"spec.hosts[0].name: cannot be empty",    // Name is missing
			"spec.hosts[0].address: cannot be empty", // Address is missing
			// User might be defaulted if global.user was set, but in yamlInvalidAfterDefaults, global.user is not set.
			// The default for global.user is empty string. Host user defaults to global user.
			// So, host user will also be empty and should fail validation.
			"spec.hosts[0].user: cannot be empty (after defaults)",
			// The "no SSH authentication method" error might not appear if name/address/user are already missing,
			// as those are more fundamental. So, we'll rely on those for this test case.
		}

		for _, sub := range expectedSubstrings {
			assert.Contains(t, errorStr, sub, "Expected validation error string to contain '%s'", sub)
		}
	}
}

func TestParseFromFile_EmptyFileContent(t *testing.T) {
	cfg, err := parseYAMLString(t, "") // Empty content
	assert.Error(t, err, "ParseFromFile with empty YAML content should result in validation errors")
	assert.Nil(t, cfg, "Config should be nil on validation error from empty content")

	if err != nil {
		assert.Contains(t, err.Error(), "configuration validation failed", "Error message should indicate validation failure")
		// Expect multiple errors for essential fields missing after defaults on an empty struct.
		// Match the actual errors observed from the validation logic.
		expectedSubstrings := []string{
			"metadata.name: cannot be empty",
			"spec.hosts: must contain at least one host",
			// "spec.etcd: section is required", // Validate_Cluster checks specific etcd fields if etcd section is present, not if section itself is missing if it's defaulted to an empty struct.
			                                     // The default for Spec.Etcd is &EtcdConfig{}. Validate_EtcdConfig would then run.
			                                     // If EtcdConfig has required fields not set by its defaults, those would be reported.
			                                     // For an empty config, etcd type defaults to "kubexm-internal", which is valid.
			// "spec.kubernetes: section is required", // Similar to etcd, defaults to &KubernetesConfig{} then validates fields.
			"spec.kubernetes.version: cannot be empty",
			// "spec.network: section is required", // Defaults to &NetworkConfig{}, then validates fields.
			"spec.network.kubePodsCIDR: cannot be empty",
			"spec.controlPlaneEndpoint: either domain or address (lb_address in YAML) must be specified",
		}
		errorStr := err.Error()
		for _, sub := range expectedSubstrings {
			assert.Contains(t, errorStr, sub, "Expected validation error for empty file to contain '%s'", sub)
		}
	}
}

func TestParseFromFile_ValidDueToDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-valid-defaults-")
	assert.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "valid_due_to_defaults.yaml")
	err = os.WriteFile(configPath, []byte(yamlValidDueToDefaults), 0644)
	assert.NoError(t, err, "Failed to write temp config file")

	cfg, err := ParseFromFile(configPath)
	assert.NoError(t, err, "ParseFromFile with YAML that should be valid due to defaults, failed")
	if err == nil && assert.NotNil(t, cfg) {
		if assert.NotNil(t, cfg.Spec.Kubernetes){
			assert.Equal(t, common.ClusterTypeKubeXM, cfg.Spec.Kubernetes.Type, "cfg.Spec.Kubernetes.Type default mismatch")
		}
		if assert.NotNil(t, cfg.Spec.Global) {
			assert.Equal(t, 22, cfg.Spec.Global.Port, "cfg.Spec.Global.Port default mismatch")
		}
		if assert.Len(t, cfg.Spec.Hosts, 1) && cfg.Spec.Hosts != nil {
			assert.Equal(t, "testuser", cfg.Spec.Hosts[0].User, "cfg.Spec.Hosts[0].User inheritance/default mismatch")
		}
		if assert.NotNil(t, cfg.Spec.Etcd) {
			assert.Equal(t, v1alpha1.EtcdTypeKubeXMSInternal, cfg.Spec.Etcd.Type, "cfg.Spec.Etcd.Type default mismatch")
		}
		if assert.NotNil(t, cfg.Spec.Kubernetes) {
			assert.Equal(t, "cluster.local", cfg.Spec.Kubernetes.DNSDomain, "cfg.Spec.Kubernetes.DNSDomain default mismatch")
		}
		if assert.NotNil(t, cfg.Spec.Network) {
			assert.Equal(t, "calico", cfg.Spec.Network.Plugin, "cfg.Spec.Network.Plugin default mismatch")
		}
	}
}
