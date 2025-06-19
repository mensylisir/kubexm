package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	// "gopkg.in/yaml.v3" // Not strictly needed if relying on LoadFromBytes
	"github.com/kubexms/kubexms/pkg/apis/kubexms/v1alpha1"
)

const validYAMLMinimal = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  global:
    user: "testuser"
    # port: 22 # Defaulted by v1alpha1.SetDefaults_Cluster
    # connectionTimeout: 30s # Defaulted by v1alpha1.SetDefaults_Cluster
  hosts:
  - name: master-1
    address: 192.168.1.10
    roles: ["master", "etcd"]
    # user: "testuser" # Inherited from global by v1alpha1.SetDefaults_Cluster
    # port: 22 # Inherited from global by v1alpha1.SetDefaults_Cluster
  kubernetes: # kubernetes section is required by v1alpha1.Validate_Cluster
    version: v1.25.0
    # clusterName will be defaulted by SetDefaults_KubernetesConfig
    # dnsDomain will be defaulted by SetDefaults_KubernetesConfig
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
    ignoreErr: false
    skipPreflight: false
  hosts:
  - name: master-1
    address: 192.168.1.10
    internalAddress: 10.0.0.10
    port: 22 # Explicit port for this host
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
    # workDir: /tmp/host_work # Removed, not in v1alpha1.HostSpec
  - name: worker-1
    address: 192.168.1.20
    roles: ["worker"]
    user: workeruser
    # Port for worker-1 will be inherited from global (2222)
  containerRuntime:
    type: containerd
    version: "1.6.9"
  containerd:
    # version: "1.6.9" # Can inherit or be specific
    registryMirrors:
      "docker.io":
      - "https://mirror.docker.com"
      - "https://another.mirror.com"
    insecureRegistries:
      - "my.insecure.registry:5000"
    useSystemdCgroup: true
    # configPath: "/etc/containerd/custom_config.toml"
  etcd:
    type: stacked
    version: v3.5.9
    # dataDir: "/var/custom/etcd"
    clientPort: 2378 # Example of non-default pointer
    extraArgs:
      election-timeout: "1200"
  kubernetes:
    version: v1.25.3
    clusterName: my-k8s-cluster
    dnsDomain: custom.cluster.local
    proxyMode: ipvs
    apiServer: # Changed from apiServerArgs
      extraArgs:
        "audit-log-maxage": "30"
    kubelet: # Changed from kubeletArgs
      extraArgs:
        "cgroup-driver": "systemd"
    nodelocaldns:
      enabled: true
  network:
    plugin: calico
    # version: v3.24.5 # Removed as it's not in v1alpha1.NetworkConfig directly
    podSubnet: "10.244.0.0/16"
    serviceSubnet: "10.96.0.0/12"
    calico:
      logSeverityScreen: Info
      # VethMTU: 1420 # Example if we want to test non-default MTU
      ipPools:
      - name: "mypool-1"
        cidr: "192.168.100.0/24"
        encapsulation: "VXLAN"
        natOutgoing: true
        blockSize: 27
      - name: "mypool-2-default-blockSize"
        cidr: "192.168.101.0/24"
        # blockSize will be defaulted
  highAvailability:
    type: keepalived
    vip: 192.168.1.100
  preflight:
    disableSwap: true
    minCPUCores: 2
  kernel:
    modules: ["br_netfilter", "ip_vs"]
    sysctlParams:
      "net.bridge.bridge-nf-call-iptables": "1"
  addons:
  - name: coredns
    enabled: true
    namespace: kube-system
  - name: metrics-server
    # enabled: true # Defaulted by SetDefaults_AddonConfig
    sources:
      chart:
        name: metrics-server
        repo: https://kubernetes-sigs.github.io/metrics-server/
        version: 0.6.1
        values: ["args={--kubelet-insecure-tls}"]
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

	if cfg.ObjectMeta.Name != "test-cluster" { // Changed to ObjectMeta
		t.Errorf("ObjectMeta.Name = %s, want test-cluster", cfg.ObjectMeta.Name)
	}
	if len(cfg.Spec.Hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(cfg.Spec.Hosts))
	}
	// Check defaulted global values
	if cfg.Spec.Global == nil {
		t.Fatal("cfg.Spec.Global should be initialized by SetDefaults_Cluster")
	}
	if cfg.Spec.Global.Port != 22 {
		t.Errorf("cfg.Spec.Global.Port = %d, want 22 (default)", cfg.Spec.Global.Port)
	}
	// Check host inherited and specific values
	if cfg.Spec.Hosts[0].User != "testuser" {
		t.Errorf("Host[0].User = %s, want testuser (from global)", cfg.Spec.Hosts[0].User)
	}
	if cfg.Spec.Hosts[0].Port != 22 { // Inherited from global default
		t.Errorf("Host[0].Port = %d, want 22 (inherited from global default)", cfg.Spec.Hosts[0].Port)
	}
	// Check Kubernetes basic values
	if cfg.Spec.Kubernetes == nil {
		t.Fatal("cfg.Spec.Kubernetes should be initialized by SetDefaults_Cluster")
	}
	if cfg.Spec.Kubernetes.Version != "v1.25.0" {
		t.Errorf("cfg.Spec.Kubernetes.Version = %s, want v1.25.0", cfg.Spec.Kubernetes.Version)
	}
	if cfg.Spec.Kubernetes.ClusterName != "test-cluster" { // Defaulted from ObjectMeta.Name
		t.Errorf("cfg.Spec.Kubernetes.ClusterName = %s, want test-cluster (defaulted)", cfg.Spec.Kubernetes.ClusterName)
	}
	if cfg.Spec.Kubernetes.DNSDomain != "cluster.local" { // Defaulted
		t.Errorf("cfg.Spec.Kubernetes.DNSDomain = %s, want cluster.local (defaulted)", cfg.Spec.Kubernetes.DNSDomain)
	}
}

func TestLoadFromBytes_ValidFull(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(validYAMLFull))
	if err != nil {
		t.Fatalf("LoadFromBytes with full valid YAML failed: %v", err)
	}
	if cfg.ObjectMeta.Name != "full-cluster" { // Changed to ObjectMeta
		t.Errorf("ObjectMeta.Name = %s, want full-cluster", cfg.ObjectMeta.Name)
	}
	if cfg.Spec.Global == nil { t.Fatal("Spec.Global is nil") }
	if cfg.Spec.Global.User != "globaluser" {
		t.Errorf("Global.User = %s, want globaluser", cfg.Spec.Global.User)
	}
	if cfg.Spec.Global.Port != 2222 {
		t.Errorf("Global.Port = %d, want 2222", cfg.Spec.Global.Port)
	}
	if len(cfg.Spec.Hosts) != 2 {
		t.Fatalf("Expected 2 hosts, got %d", len(cfg.Spec.Hosts))
	}
	if cfg.Spec.Hosts[0].User != "hostuser" {
		t.Errorf("Host[0].User = %s, want hostuser", cfg.Spec.Hosts[0].User)
	}
	if cfg.Spec.Hosts[0].Port != 22 {
		t.Errorf("Host[0].Port = %d, want 22", cfg.Spec.Hosts[0].Port)
	}
	if cfg.Spec.Hosts[1].User != "workeruser" {
		t.Errorf("Host[1].User = %s, want workeruser", cfg.Spec.Hosts[1].User)
	}
	if cfg.Spec.Hosts[1].Port != 2222 { // Inherited from global
		t.Errorf("Host[1].Port = %d, want 2222 (inherited)", cfg.Spec.Hosts[1].Port)
	}

	if cfg.Spec.ContainerRuntime == nil { t.Fatal("Spec.ContainerRuntime is nil") }
	if cfg.Spec.ContainerRuntime.Type != "containerd" {
		t.Errorf("ContainerRuntime.Type = %s, want containerd", cfg.Spec.ContainerRuntime.Type)
	}
	if cfg.Spec.Containerd == nil { t.Fatal("Spec.Containerd is nil") }
	if mirrors, ok := cfg.Spec.Containerd.RegistryMirrors["docker.io"]; !ok || len(mirrors) != 2 {
		t.Error("Containerd mirrors for docker.io not parsed correctly")
	}
	if cfg.Spec.Containerd.UseSystemdCgroup == nil || !*cfg.Spec.Containerd.UseSystemdCgroup {
		t.Error("Containerd.UseSystemdCgroup should be true")
	}

	if cfg.Spec.Etcd == nil { t.Fatal("Spec.Etcd is nil") }
	if cfg.Spec.Etcd.Type != "stacked" {
		t.Errorf("Etcd.Type = %s, want stacked", cfg.Spec.Etcd.Type)
	}
	if cfg.Spec.Etcd.ClientPort == nil || *cfg.Spec.Etcd.ClientPort != 2378 {
		t.Errorf("Etcd.ClientPort = %v, want 2378", cfg.Spec.Etcd.ClientPort)
	}

	if cfg.Spec.Kubernetes == nil { t.Fatal("Spec.Kubernetes is nil") }
	if cfg.Spec.Kubernetes.ClusterName != "my-k8s-cluster" {
		t.Errorf("Kubernetes.ClusterName = %s, want my-k8s-cluster", cfg.Spec.Kubernetes.ClusterName)
	}
	if cfg.Spec.Kubernetes.DNSDomain != "custom.cluster.local" {
		t.Errorf("Kubernetes.DNSDomain = %s, want custom.cluster.local", cfg.Spec.Kubernetes.DNSDomain)
	}
	if cfg.Spec.Kubernetes.APIServer == nil || cfg.Spec.Kubernetes.APIServer.ExtraArgs["audit-log-maxage"] != "30" {
		t.Error("Kubernetes.APIServer.ExtraArgs not parsed correctly")
	}

	if cfg.Spec.Network == nil { t.Fatal("Spec.Network is nil") }
	if cfg.Spec.Network.Plugin != "calico" {
		t.Errorf("Network.Plugin = %s, want calico", cfg.Spec.Network.Plugin)
	}
    if cfg.Spec.Network.PodSubnet != "10.244.0.0/16" {
        t.Errorf("Network.PodSubnet = %s, want 10.244.0.0/16", cfg.Spec.Network.PodSubnet)
    }
    // Add assertions for Calico fields
    if cfg.Spec.Network.Calico == nil {
        t.Fatal("Spec.Network.Calico should not be nil for plugin 'calico'")
    }
    if cfg.Spec.Network.Calico.LogSeverityScreen == nil || *cfg.Spec.Network.Calico.LogSeverityScreen != "Info" {
        t.Errorf("Calico LogSeverityScreen = %v, want 'Info'", cfg.Spec.Network.Calico.LogSeverityScreen)
    }
    // if cfg.Spec.Network.Calico.GetVethMTU() != 1420 { // Example if VethMTU was set in YAML
    //     t.Errorf("Calico VethMTU = %d, want 1420", cfg.Spec.Network.Calico.GetVethMTU())
    // }

    if len(cfg.Spec.Network.Calico.IPPools) != 2 {
        t.Fatalf("Expected 2 Calico IPPools, got %d", len(cfg.Spec.Network.Calico.IPPools))
    }
    pool1 := cfg.Spec.Network.Calico.IPPools[0]
    if pool1.Name != "mypool-1" { t.Errorf("IPPool[0].Name = %s, want mypool-1", pool1.Name) }
    if pool1.CIDR != "192.168.100.0/24" { t.Errorf("IPPool[0].CIDR = %s", pool1.CIDR) }
    if pool1.Encapsulation != "VXLAN" { t.Errorf("IPPool[0].Encapsulation = %s", pool1.Encapsulation) }
    if pool1.NatOutgoing == nil || !*pool1.NatOutgoing { t.Error("IPPool[0].NatOutgoing should be true") }
    if pool1.BlockSize == nil || *pool1.BlockSize != 27 { t.Errorf("IPPool[0].BlockSize = %v, want 27", pool1.BlockSize) }

    pool2 := cfg.Spec.Network.Calico.IPPools[1]
    if pool2.Name != "mypool-2-default-blockSize" {t.Errorf("IPPool[1].Name incorrect")}
    if pool2.BlockSize == nil || *pool2.BlockSize != 26 { // Check if it defaulted correctly
         t.Errorf("IPPool[1].BlockSize = %v, want 26 (default)", pool2.BlockSize)
    }

	if cfg.Spec.HighAvailability == nil { t.Fatal("Spec.HighAvailability is nil") }
	if cfg.Spec.HighAvailability.Type != "keepalived" {
		t.Errorf("HighAvailability.Type = %s, want keepalived", cfg.Spec.HighAvailability.Type)
	}
    if cfg.Spec.HighAvailability.VIP != "192.168.1.100" {
        t.Errorf("HighAvailability.VIP = %s, want 192.168.1.100", cfg.Spec.HighAvailability.VIP)
    }

	if cfg.Spec.Preflight == nil { t.Fatal("Spec.Preflight is nil")}
	if cfg.Spec.Preflight.DisableSwap == nil || !*cfg.Spec.Preflight.DisableSwap {
		t.Error("Preflight.DisableSwap should be true")
	}
    if cfg.Spec.Preflight.MinCPUCores == nil || *cfg.Spec.Preflight.MinCPUCores != 2 {
        t.Errorf("Preflight.MinCPUCores = %v, want 2", cfg.Spec.Preflight.MinCPUCores)
    }


	if len(cfg.Spec.Addons) != 2 {
		t.Fatalf("Expected 2 addons, got %d", len(cfg.Spec.Addons))
	}
	if cfg.Spec.Addons[0].Name != "coredns" {
		t.Errorf("Addon[0].Name = %s, want coredns", cfg.Spec.Addons[0].Name)
	}
	if cfg.Spec.Addons[0].Enabled == nil || !*cfg.Spec.Addons[0].Enabled {
		t.Error("Addon coredns should be enabled")
	}
    if cfg.Spec.Addons[1].Name != "metrics-server" {
        t.Errorf("Addon[1].Name = %s, want metrics-server", cfg.Spec.Addons[1].Name)
    }
    if cfg.Spec.Addons[1].Sources.Chart == nil || cfg.Spec.Addons[1].Sources.Chart.Name != "metrics-server" {
        t.Error("Addon metrics-server chart source not parsed correctly")
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
	if cfg.ObjectMeta.Name != "full-cluster" { // Changed to ObjectMeta
		t.Errorf("Loaded config ObjectMeta.Name = %s, want full-cluster", cfg.ObjectMeta.Name)
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
