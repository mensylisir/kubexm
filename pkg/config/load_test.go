package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3" // For direct unmarshal check in one test
)

const validYAMLMinimal = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  global: # Add global user to pass validation after defaults
    user: "testuser"
  hosts:
  - name: master-1
    address: 192.168.1.10
    roles: ["master", "etcd"]
    # User field will be inherited from global for this host
  kubernetes:
    version: v1.25.0
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
    connectionTimeout: 60s
    workDir: /tmp/global_work
    verbose: true
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
    workDir: /tmp/host_work
  - name: worker-1
    address: 192.168.1.20
    roles: ["worker"]
    user: workeruser # Explicit user for this host
  containerRuntime:
    type: containerd
    version: "1.6.9"
  containerd:
    useSystemdCgroup: true
    registryMirrors: # Changed from RegistryMirrorsConfig to match struct
      "docker.io":
      - "https://mirror.docker.com"
      - "https://another.mirror.com"
  etcd:
    type: stacked
    version: v3.5.9
    managed: true # Added to satisfy potential IsEnabled in module factory
  kubernetes:
    version: v1.25.3
    clusterName: my-k8s-cluster
    podSubnet: "10.244.0.0/16"
    serviceSubnet: "10.96.0.0/12"
  network:
    plugin: calico
  highAvailability:
    type: keepalived
    # vip: 192.168.1.100
  addons:
  - name: coredns
    enabled: true
  - name: metrics-server
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

func TestLoadFromBytes_ValidMinimal(t *testing.T) {
	// This minimal YAML now includes global.user to pass validation after defaults.
	cfg, err := LoadFromBytes([]byte(validYAMLMinimal))
	if err != nil {
		t.Fatalf("LoadFromBytes with minimal valid YAML failed: %v", err)
	}

	if cfg.Metadata.Name != "test-cluster" {
		t.Errorf("Metadata.Name = %s, want test-cluster", cfg.Metadata.Name)
	}
	if len(cfg.Spec.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(cfg.Spec.Hosts))
	}
	if cfg.Spec.Hosts[0].User != "testuser" { // Inherited from global
		t.Errorf("Host[0].User = %s, want testuser", cfg.Spec.Hosts[0].User)
	}
}

func TestLoadFromBytes_ValidFull(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(validYAMLFull))
	if err != nil {
		t.Fatalf("LoadFromBytes with full valid YAML failed: %v", err)
	}
	if cfg.Metadata.Name != "full-cluster" {
		t.Errorf("Metadata.Name = %s, want full-cluster", cfg.Metadata.Name)
	}
	if cfg.Spec.Global.User != "globaluser" {
		t.Errorf("Global.User = %s, want globaluser", cfg.Spec.Global.User)
	}
	if len(cfg.Spec.Hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(cfg.Spec.Hosts))
	}
	if cfg.Spec.Hosts[0].User != "hostuser" {
		t.Errorf("Host[0].User = %s, want hostuser", cfg.Spec.Hosts[0].User)
	}
	if cfg.Spec.ContainerRuntime.Type != "containerd" {
		t.Errorf("ContainerRuntime.Type = %s, want containerd", cfg.Spec.ContainerRuntime.Type)
	}
	if cfg.Spec.Containerd == nil || len(cfg.Spec.Containerd.RegistryMirrors["docker.io"]) != 2 {
		t.Error("Containerd mirrors for docker.io not parsed correctly or Containerd spec is nil")
	}
}

func TestLoadFromBytes_MalformedYAML(t *testing.T) {
	_, err := LoadFromBytes([]byte(invalidYAMLMalformed))
	if err == nil {
		t.Fatal("LoadFromBytes with malformed YAML expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal yaml config") {
		t.Errorf("Error message = %q, expected to contain 'failed to unmarshal'", err.Error())
	}
}

func TestLoad_FileSuccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-load-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configPath, []byte(validYAMLFull), 0644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(%s) failed: %v", configPath, err)
	}
	if cfg.Metadata.Name != "full-cluster" {
		t.Errorf("Loaded config Metadata.Name = %s, want full-cluster", cfg.Metadata.Name)
	}
}

func TestLoad_FileNotExist(t *testing.T) {
	_, err := Load("/path/to/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("Load with non-existent file expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("Error message = %q, expected to contain 'failed to read'", err.Error())
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("Load with empty path expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path cannot be empty") {
		t.Errorf("Error message = %q, want 'path cannot be empty'", err.Error())
	}
}
