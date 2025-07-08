package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
)

const (
	validConfigYAML = `
apiVersion: kubexm.mensylisir.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  type: kubexm
  hosts:
  - name: node1
    address: 192.168.1.101
    port: 22
    user: testuser
    arch: amd64
    roles: ["etcd", "master", "worker"]
  - name: node2
    address: 192.168.1.102
    roles: ["worker"]
  roleGroups:
    etcd:
    - node1
    master:
    - node1
    worker:
    - node1
    - node2
  kubernetes:
    version: "v1.28.2"
    containerRuntimeType: "containerd" # Simulating flattened field
  etcd:
    type: "kubexm"
  network:
    plugin: "calico"
    kubePodsCIDR: "10.233.64.0/18"
    kubeServiceCIDR: "10.233.0.0/18"
`
	invalidConfigMissingHostsYAML = `
apiVersion: kubexm.mensylisir.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-invalid
spec:
  kubernetes:
    version: "v1.28.2"
  etcd:
    type: "kubexm"
  network:
    plugin: "calico"
`
	invalidConfigBadRoleYAML = `
apiVersion: kubexm.mensylisir.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-bad-role
spec:
  hosts:
  - name: node1
    address: 192.168.1.101
  roleGroups:
    etcd:
    - node-not-exist # This host is not in spec.hosts
  kubernetes:
    version: "v1.28.2"
  etcd:
    type: "kubexm"
  network:
    plugin: "calico"
`
)

func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}
	return filePath
}

func TestParseFromFile_ValidConfig(t *testing.T) {
	filePath := createTempConfigFile(t, validConfigYAML)

	cfg, err := ParseFromFile(filePath)
	if err != nil {
		t.Fatalf("ParseFromFile() with valid config failed: %v", err)
	}

	if cfg == nil {
		t.Fatalf("ParseFromFile() returned nil config for a valid input")
	}

	// Check metadata
	if cfg.APIVersion != v1alpha1.SchemeGroupVersion.Group+"/"+v1alpha1.SchemeGroupVersion.Version {
		t.Errorf("Expected APIVersion %s, got %s", v1alpha1.SchemeGroupVersion.Group+"/"+v1alpha1.SchemeGroupVersion.Version, cfg.APIVersion)
	}
	if cfg.Kind != "Cluster" {
		t.Errorf("Expected Kind Cluster, got %s", cfg.Kind)
	}
	if cfg.ObjectMeta.Name != "test-cluster" {
		t.Errorf("Expected metadata.name test-cluster, got %s", cfg.ObjectMeta.Name)
	}

	// Check a few defaulted fields
	if cfg.Spec.Global == nil {
		t.Fatal("Expected spec.global to be non-nil after defaulting")
	}
	if cfg.Spec.Global.Port != common.DefaultSSHPort { // Assuming global port defaults to common.DefaultSSHPort if not set
		t.Errorf("Expected global.port default %d, got %d", common.DefaultSSHPort, cfg.Spec.Global.Port)
	}
	if cfg.Spec.Hosts[1].Port != common.DefaultSSHPort { // node2 did not specify port
		t.Errorf("Expected hosts[1].port default %d, got %d", common.DefaultSSHPort, cfg.Spec.Hosts[1].Port)
	}
	if cfg.Spec.Hosts[1].User != "" { // node2 did not specify user, should be empty or defaulted from global if global user was set
		// This depends on how SetDefaults_Cluster handles user defaulting logic.
		// If global user is also empty, host user remains empty.
	}
	if cfg.Spec.Hosts[1].Arch != common.DefaultArch {
		t.Errorf("Expected hosts[1].arch default %s, got %s", common.DefaultArch, cfg.Spec.Hosts[1].Arch)
	}
	if cfg.Spec.Etcd.DataDir == "" { // Check if EtcdConfig.DataDir was defaulted
		t.Error("Expected etcd.dataDir to be defaulted, but it's empty")
	}
	if cfg.Spec.Kubernetes.ClusterName != "test-cluster" { // Defaulted from metadata.name
		t.Errorf("Expected kubernetes.clusterName to be defaulted to 'test-cluster', got '%s'", cfg.Spec.Kubernetes.ClusterName)
	}
	if cfg.Spec.Type != common.ClusterTypeKubeXM {
		t.Errorf("Expected spec.type to be defaulted to '%s', got '%s'", common.ClusterTypeKubeXM, cfg.Spec.Type)
	}

	// Check a specific value
	if cfg.Spec.Hosts[0].Name != "node1" {
		t.Errorf("Expected hosts[0].name node1, got %s", cfg.Spec.Hosts[0].Name)
	}
	if cfg.Spec.Network.KubePodsCIDR != "10.233.64.0/18" {
		t.Errorf("Expected network.kubePodsCIDR 10.233.64.0/18, got %s", cfg.Spec.Network.KubePodsCIDR)
	}
}

func TestParseFromFile_NonExistentFile(t *testing.T) {
	_, err := ParseFromFile("non_existent_config.yaml")
	if err == nil {
		t.Fatal("ParseFromFile() with non-existent file did not return an error")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("Expected error about reading file, got: %v", err)
	}
}

func TestParseFromFile_InvalidYAML(t *testing.T) {
	filePath := createTempConfigFile(t, "this: is: not: valid: yaml")
	_, err := ParseFromFile(filePath)
	if err == nil {
		t.Fatal("ParseFromFile() with invalid YAML did not return an error")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal YAML") {
		t.Errorf("Expected error about unmarshalling YAML, got: %v", err)
	}
}

func TestParseFromFile_ValidationFailure_MissingHosts(t *testing.T) {
	filePath := createTempConfigFile(t, invalidConfigMissingHostsYAML)
	_, err := ParseFromFile(filePath)
	if err == nil {
		t.Fatal("ParseFromFile() with config missing hosts did not return an error")
	}
	if !strings.Contains(err.Error(), "spec.hosts: must contain at least one host") {
		t.Errorf("Expected validation error about missing hosts, got: %v", err)
	}
}

func TestParseFromFile_ValidationFailure_BadRoleGroupHost(t *testing.T) {
	filePath := createTempConfigFile(t, invalidConfigBadRoleYAML)
	_, err := ParseFromFile(filePath)
	if err == nil {
		t.Fatal("ParseFromFile() with bad role group host did not return an error")
	}
	// The exact error message depends on how Validate_RoleGroupsSpec formats it.
	// It should mention "node-not-exist" and "not defined in spec.hosts".
	expectedErrorPart := "host 'node-not-exist' (from range 'node-not-exist') is not defined in spec.hosts"
	if !strings.Contains(err.Error(), expectedErrorPart) {
		t.Errorf("Expected validation error about non-existent host in role group, got: %v", err)
	}
}

// Add more tests for other validation rules in v1alpha1.Validate_Cluster as they are implemented/refined.
// For example, testing specific field validations (IP format, port range, enum values for types, etc.)
// and interactions between different parts of the spec.
// Test case for default sysctl params
func TestParseFromFile_DefaultSysctlParams(t *testing.T) {
	configYAML := `
apiVersion: kubexm.mensylisir.io/v1alpha1
kind: Cluster
metadata:
  name: test-sysctl
spec:
  hosts:
  - name: node1
    address: 192.168.1.1
  kubernetes:
    version: v1.28.0
  etcd:
    type: kubexm
  network:
    plugin: calico
`
	filePath := createTempConfigFile(t, configYAML)
	cfg, err := ParseFromFile(filePath)
	if err != nil {
		t.Fatalf("ParseFromFile() failed: %v", err)
	}

	if cfg.Spec.System == nil {
		t.Fatal("Expected spec.system to be defaulted")
	}
	if cfg.Spec.System.SysctlParams["net.bridge.bridge-nf-call-iptables"] != "1" {
		t.Errorf("Expected default sysctl net.bridge.bridge-nf-call-iptables=1, got %s", cfg.Spec.System.SysctlParams["net.bridge.bridge-nf-call-iptables"])
	}
}
