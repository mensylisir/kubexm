package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	// "gopkg.in/yaml.v3" // Not strictly needed if relying on LoadFromBytes
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

const validYAMLMinimal = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  type: kubexm # Added ClusterSpec type
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
  etcd: # Added etcd spec for default type testing
    type: kubexm
  kubernetes: # kubernetes section is required by v1alpha1.Validate_Cluster
    version: v1.25.0
    # clusterName will be defaulted by SetDefaults_KubernetesConfig
    # dnsDomain will be defaulted by SetDefaults_KubernetesConfig
`

const validYAMLDockerRuntime = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: docker-cluster
spec:
  global:
    user: "dockeruser"
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
      criDockerdVersion: "v0.3.1" # Example version
  kubernetes:
    version: v1.24.0
    containerManager: "systemd"
`

const validYAMLFull = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: full-cluster
spec:
  type: kubeadm # Testing the other cluster type
  global:
    user: globaluser
    port: 2222
    connectionTimeout: 60s
    workDir: /tmp/global_work_enriched # Make unique for test
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
    disabledPlugins: ["io.containerd.internal.v1.opt"]
  etcd:
    type: kubeadm # Testing new etcd type
    version: v3.5.9
    clientPort: 2378
    peerPort: 2381 # Different from default
    dataDir: "/mnt/etcd_data_custom"
    extraArgs:
      - "--election-timeout=1200" # Changed to slice
      - "--heartbeat-interval=60"
    backupDir: "/opt/etcd_backup"
    backupPeriodHours: 12
    keepBackupNumber: 5
    snapshotCount: 50000
    logLevel: debug
  kubernetes:
    version: v1.25.3
    clusterName: my-k8s-cluster
    dnsDomain: custom.cluster.local
    proxyMode: ipvs
    containerManager: systemd # For kubelet cgroup driver default
    maxPods: 150
    featureGates:
      "EphemeralContainers": true
    apiServer:
      admissionPlugins: ["NodeRestriction","NamespaceLifecycle"]
      extraArgs:
        - "--audit-log-maxage=30" # Changed to slice
        - "--etcd-servers=http://127.0.0.1:2379" # Example of specific field
    controllerManager:
      extraArgs: ["--leader-elect=false"]
    kubelet:
      cgroupDriver: "systemd" # Explicitly set
      evictionHard:
        "memory.available": "5%"
        "nodefs.available": "10%"
      extraArgs:
        - "--kube-reserved=cpu=500m,memory=1Gi" # Changed to slice
    kubeletConfiguration: |
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      serializeImagePulls: false
      evictionHard:
        memory.available: "200Mi" # This would override KubeletConfig.EvictionHard if both used
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
    podSubnet: "10.244.0.0/16"
    serviceSubnet: "10.96.0.0/12"
    calico:
      logSeverityScreen: Info
      vethMTU: 1420 # Explicitly set
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
    type: keepalived+haproxy # More complex HA type
    vip: 192.168.1.100
    controlPlaneEndpointDomain: "k8s-api.internal.example.com"
    controlPlaneEndpointPort: 8443
    keepalived:
      interface: "eth1"
      vrid: 101
      priority: 150
      authType: "PASS"
      authPass: "ha_secret"
    haproxy:
      frontendBindAddress: "0.0.0.0" # Can be same as VIP or specific node IP if VIP managed by keepalived on nodes
      # frontendPort: 8443 # Will default from controlPlaneEndpointPort or HAConfig.FrontendPort if that was a field
      mode: "tcp"
      balanceAlgorithm: "leastconn"
      backendServers: # These would typically be your control plane nodes
        - name: "master-1-backend" # Name derived from actual host for clarity
          address: "192.168.1.10" # Matches master-1 host address
          port: 6443 # Kube API server port on node
        # Add other masters if any
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
  roleGroups:
    master:
      hosts: ["master-1"]
    worker:
      hosts: ["worker-1"]
    etcd:
      hosts: ["master-1"] # Assuming etcd runs on master for this full config
    registry:
      hosts: ["master-1"] # Add registry role to master-1 for testing
  # New sections for Storage, Registry, OS
  storage:
    defaultStorageClass: "openebs-hostpath"
    openebs:
      enabled: true
      version: "3.3.0"
      basePath: "/mnt/openebs_data"
      engines:
        localHostPath:
          enabled: true
        # cstor: { enabled: true } # Example if enabling another
  registry:
    privateRegistry: "myreg.local:5000"
    insecureRegistries: ["myreg.local:5000"]
    auths:
      "myreg.local:5000":
        username: "reguser"
        password: "regpassword"
  os:
    ntpServers: ["ntp1.example.com", "ntp2.example.com"]
    timezone: "America/Los_Angeles"
    skipConfigureOS: false
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
	// Check ClusterSpec.Type
	if cfg.Spec.Type != v1alpha1.ClusterTypeKubeXM {
		t.Errorf("cfg.Spec.Type = %s, want %s", cfg.Spec.Type, v1alpha1.ClusterTypeKubeXM)
	}
	// Check Etcd.Type
	if cfg.Spec.Etcd == nil {
		t.Fatal("cfg.Spec.Etcd should be initialized")
	}
	if cfg.Spec.Etcd.Type != v1alpha1.EtcdTypeKubeXMSInternal { // "kubexm"
		t.Errorf("cfg.Spec.Etcd.Type = %s, want %s", cfg.Spec.Etcd.Type, v1alpha1.EtcdTypeKubeXMSInternal)
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
	if cfg.Spec.Global == nil {
		t.Fatal("Spec.Global is nil")
	}
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

	if cfg.Spec.ContainerRuntime == nil {
		t.Fatal("Spec.ContainerRuntime is nil")
	}
	if cfg.Spec.ContainerRuntime.Type != "containerd" {
		t.Errorf("ContainerRuntime.Type = %s, want containerd", cfg.Spec.ContainerRuntime.Type)
	}
	if cfg.Spec.ContainerRuntime.Containerd == nil { // Corrected path
		t.Fatal("Spec.ContainerRuntime.Containerd is nil")
	}
	if mirrors, ok := cfg.Spec.ContainerRuntime.Containerd.RegistryMirrors["docker.io"]; !ok || len(mirrors) != 2 { // Corrected path
		t.Error("Containerd mirrors for docker.io not parsed correctly")
	}
	if cfg.Spec.ContainerRuntime.Containerd.UseSystemdCgroup == nil || !*cfg.Spec.ContainerRuntime.Containerd.UseSystemdCgroup { // Corrected path
		t.Error("Containerd.UseSystemdCgroup should be true")
	}

	if cfg.Spec.Etcd == nil {
		t.Fatal("Spec.Etcd is nil")
	}
	if cfg.Spec.Etcd.Type != v1alpha1.EtcdTypeInternal { // "kubeadm"
		t.Errorf("Etcd.Type = %s, want %s", cfg.Spec.Etcd.Type, v1alpha1.EtcdTypeInternal)
	}
	if cfg.Spec.Etcd.ClientPort == nil || *cfg.Spec.Etcd.ClientPort != 2378 {
		t.Errorf("Etcd.ClientPort = %v, want 2378", cfg.Spec.Etcd.ClientPort)
	}

	// Check ClusterSpec.Type
	if cfg.Spec.Type != v1alpha1.ClusterTypeKubeadm { // Corrected constant name
		t.Errorf("cfg.Spec.Type = %s, want %s", cfg.Spec.Type, v1alpha1.ClusterTypeKubeadm)
	}

	// Check RoleGroups including Registry
	if cfg.Spec.RoleGroups == nil {
		t.Fatal("Spec.RoleGroups is nil")
	}
	if cfg.Spec.RoleGroups.Registry.Hosts == nil || len(cfg.Spec.RoleGroups.Registry.Hosts) != 1 || cfg.Spec.RoleGroups.Registry.Hosts[0] != "master-1" {
		t.Errorf("RoleGroups.Registry.Hosts not parsed correctly: %v", cfg.Spec.RoleGroups.Registry.Hosts)
	}

	if cfg.Spec.Kubernetes == nil {
		t.Fatal("Spec.Kubernetes is nil")
	}
	if cfg.Spec.Kubernetes.ClusterName != "my-k8s-cluster" {
		t.Errorf("Kubernetes.ClusterName = %s, want my-k8s-cluster", cfg.Spec.Kubernetes.ClusterName)
	}
	if cfg.Spec.Kubernetes.DNSDomain != "custom.cluster.local" {
		t.Errorf("Kubernetes.DNSDomain = %s, want custom.cluster.local", cfg.Spec.Kubernetes.DNSDomain)
	}
	// Updated assertions for ExtraArgs as []string
	k8sCfg := cfg.Spec.Kubernetes
	if k8sCfg.APIServer == nil || len(k8sCfg.APIServer.ExtraArgs) == 0 || k8sCfg.APIServer.ExtraArgs[0] != "--audit-log-maxage=30" {
		t.Errorf("Kubernetes.APIServer.ExtraArgs not parsed correctly: %v", k8sCfg.APIServer.ExtraArgs)
	}
	if k8sCfg.ContainerManager != "systemd" {
		t.Errorf("K8s ContainerManager failed: %s", k8sCfg.ContainerManager)
	}
	if k8sCfg.MaxPods == nil || *k8sCfg.MaxPods != 150 {
		t.Errorf("K8s MaxPods failed: %v", k8sCfg.MaxPods)
	}
	if k8sCfg.FeatureGates == nil || !k8sCfg.FeatureGates["EphemeralContainers"] {
		t.Error("K8s FeatureGates failed")
	}
	if k8sCfg.APIServer == nil || len(k8sCfg.APIServer.AdmissionPlugins) == 0 {
		t.Error("K8s APIServer.AdmissionPlugins failed")
	}
	if k8sCfg.Kubelet == nil || k8sCfg.Kubelet.CgroupDriver == nil || *k8sCfg.Kubelet.CgroupDriver != "systemd" {
		t.Errorf("K8s Kubelet.CgroupDriver failed: %v", k8sCfg.Kubelet.CgroupDriver)
	}
	if k8sCfg.Kubelet.EvictionHard == nil || k8sCfg.Kubelet.EvictionHard["memory.available"] != "5%" {
		t.Error("K8s Kubelet.EvictionHard failed")
	}
	if k8sCfg.KubeletConfiguration == nil || len(k8sCfg.KubeletConfiguration.Raw) == 0 {
		t.Error("K8s KubeletConfiguration failed")
	}
	if k8sCfg.KubeProxy == nil || k8sCfg.KubeProxy.IPVS == nil || k8sCfg.KubeProxy.IPVS.Scheduler != "wrr" {
		t.Error("K8s KubeProxy.IPVS.Scheduler failed")
	}
	if k8sCfg.Audit == nil || k8sCfg.Audit.Enabled == nil || !*k8sCfg.Audit.Enabled {
		t.Error("K8s Audit.Enabled failed")
	}

	netCfg := cfg.Spec.Network
	if netCfg == nil {
		t.Fatal("Spec.Network is nil")
	}
	if netCfg.Plugin != "calico" {
		t.Errorf("Network.Plugin = %s, want calico", netCfg.Plugin)
	}
	if netCfg.KubePodsCIDR != "10.244.0.0/16" { // Corrected field name
		t.Errorf("Network.KubePodsCIDR = %s, want 10.244.0.0/16", netCfg.KubePodsCIDR)
	}
	if netCfg.Calico == nil {
		t.Fatal("Spec.Network.Calico should not be nil for plugin 'calico'")
	}
	if netCfg.Calico.LogSeverityScreen == nil || *netCfg.Calico.LogSeverityScreen != "Info" {
		t.Errorf("Calico LogSeverityScreen = %v, want 'Info'", netCfg.Calico.LogSeverityScreen)
	}
	if netCfg.Calico.VethMTU == nil || *netCfg.Calico.VethMTU != 1420 {
		t.Errorf("Calico VethMTU failed: %v", netCfg.Calico.VethMTU)
	}

	if len(netCfg.Calico.IPPools) != 2 {
		t.Fatalf("Expected 2 Calico IPPools, got %d", len(netCfg.Calico.IPPools))
	}
	pool1 := netCfg.Calico.IPPools[0]
	if pool1.Name != "mypool-1" {
		t.Errorf("IPPool[0].Name = %s, want mypool-1", pool1.Name)
	}
	if pool1.CIDR != "192.168.100.0/24" {
		t.Errorf("IPPool[0].CIDR = %s", pool1.CIDR)
	}
	if pool1.Encapsulation != "VXLAN" {
		t.Errorf("IPPool[0].Encapsulation = %s", pool1.Encapsulation)
	}
	if pool1.NatOutgoing == nil || !*pool1.NatOutgoing {
		t.Error("IPPool[0].NatOutgoing should be true")
	}
	if pool1.BlockSize == nil || *pool1.BlockSize != 27 {
		t.Errorf("IPPool[0].BlockSize = %v, want 27", pool1.BlockSize)
	}

	pool2 := netCfg.Calico.IPPools[1]
	if pool2.Name != "mypool-2-default-blockSize" {
		t.Errorf("IPPool[1].Name incorrect")
	}
	if pool2.BlockSize == nil || *pool2.BlockSize != 26 {
		t.Errorf("IPPool[1].BlockSize = %v, want 26 (default)", pool2.BlockSize)
	}

	haCfg := cfg.Spec.HighAvailability
	if haCfg == nil {
		t.Fatal("Spec.HighAvailability is nil")
	}
	// haCfg.Type and haCfg.VIP are removed.
	// The type is now within haCfg.External.Type or haCfg.Internal.Type.
	// The VIP/Endpoint is in ClusterSpec.ControlPlaneEndpoint.
	// This test block needs to be re-evaluated based on new structure if specific HA fields are to be checked.
	// For now, commenting out direct access to removed fields.
	// if haCfg.Type != "keepalived+haproxy" {
	// 	t.Errorf("HighAvailability.Type = %s, want keepalived+haproxy", haCfg.Type)
	// }
	// if haCfg.VIP != "192.168.1.100" {
	// 	t.Errorf("HighAvailability.VIP = %s, want 192.168.1.100", haCfg.VIP)
	// }

	// ControlPlaneEndpointDomain and Port are also not directly in HighAvailabilityConfig
	// if haCfg.ControlPlaneEndpointDomain != "k8s-api.internal.example.com" {
	// 	t.Errorf("HA ControlPlaneEndpointDomain failed: %s", haCfg.ControlPlaneEndpointDomain)
	// }
	// if haCfg.ControlPlaneEndpointPort == nil || *haCfg.ControlPlaneEndpointPort != 8443 {
	// 	t.Errorf("HA ControlPlaneEndpointPort failed: %v", haCfg.ControlPlaneEndpointPort)
	// }

	// Example check for the new structure, assuming External LB is configured
	if haCfg.External == nil || haCfg.External.Type != "ManagedKeepalivedHAProxy" { // Example type check
		t.Errorf("Expected HA External Type 'ManagedKeepalivedHAProxy', got %v", haCfg.External)
	}
	if haCfg.External.Keepalived == nil { // Corrected path
		t.Fatal("HA.External.Keepalived section is nil")
	}
	if haCfg.External.Keepalived.Interface == nil || *haCfg.External.Keepalived.Interface != "eth1" { // Corrected path
		t.Errorf("HA.External.Keepalived.Interface = %v, want 'eth1'", haCfg.External.Keepalived.Interface)
	}
	if haCfg.External.Keepalived.VRID == nil || *haCfg.External.Keepalived.VRID != 101 { // Corrected path
		t.Errorf("HA.External.Keepalived.VRID = %v, want 101", haCfg.External.Keepalived.VRID)
	}
	// ... more Keepalived assertions (priority, authType, authPass)

	if haCfg.External.HAProxy == nil { // Corrected path
		t.Fatal("HA.External.HAProxy section is nil")
	}
	if haCfg.External.HAProxy.BalanceAlgorithm == nil || *haCfg.External.HAProxy.BalanceAlgorithm != "leastconn" { // Corrected path
		t.Errorf("HA.External.HAProxy.BalanceAlgorithm = %v, want 'leastconn'", haCfg.External.HAProxy.BalanceAlgorithm)
	}
	if len(haCfg.External.HAProxy.BackendServers) != 1 || haCfg.External.HAProxy.BackendServers[0].Name != "master-1-backend" { // Corrected path
		t.Errorf("HA.External.HAProxy.BackendServers not parsed as expected: %v", haCfg.External.HAProxy.BackendServers)
	}
	// Check that HAProxy.FrontendPort defaulted correctly
	// Current SetDefaults_HAProxyConfig defaults FrontendPort to 6443.
	if haCfg.External.HAProxy.FrontendPort == nil || *haCfg.External.HAProxy.FrontendPort != 6443 { // Corrected path
		t.Errorf("HA.External.HAProxy.FrontendPort = %v, want 6443 (HAProxy default)", haCfg.External.HAProxy.FrontendPort)
	}

	if cfg.Spec.Preflight == nil {
		t.Fatal("Spec.Preflight is nil")
	}
	if cfg.Spec.Preflight.DisableSwap == nil || !*cfg.Spec.Preflight.DisableSwap {
		t.Error("Preflight.DisableSwap should be true")
	}
	if cfg.Spec.Preflight.MinCPUCores == nil || *cfg.Spec.Preflight.MinCPUCores != 2 {
		t.Errorf("Preflight.MinCPUCores = %v, want 2", cfg.Spec.Preflight.MinCPUCores)
	}

	etcdCfg := cfg.Spec.Etcd
	if etcdCfg == nil {
		t.Fatal("Etcd config is nil")
	}
	if etcdCfg.PeerPort == nil || *etcdCfg.PeerPort != 2381 {
		t.Errorf("Etcd PeerPort failed, got %v", etcdCfg.PeerPort)
	}
	if etcdCfg.DataDir == nil || *etcdCfg.DataDir != "/mnt/etcd_data_custom" {
		t.Errorf("Etcd DataDir failed, got %v", etcdCfg.DataDir)
	}
	if len(etcdCfg.ExtraArgs) != 2 || etcdCfg.ExtraArgs[0] != "--election-timeout=1200" {
		t.Errorf("Etcd ExtraArgs failed: %v", etcdCfg.ExtraArgs)
	}
	if etcdCfg.BackupDir == nil || *etcdCfg.BackupDir != "/opt/etcd_backup" {
		t.Errorf("Etcd BackupDir failed: %v", etcdCfg.BackupDir)
	}
	if etcdCfg.LogLevel == nil || *etcdCfg.LogLevel != "debug" {
		t.Errorf("Etcd LogLevel failed: %v", etcdCfg.LogLevel)
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

	storageCfg := cfg.Spec.Storage
	if storageCfg == nil {
		t.Fatal("Storage config is nil")
	}
	if storageCfg.DefaultStorageClass == nil || *storageCfg.DefaultStorageClass != "openebs-hostpath" {
		t.Errorf("Storage DefaultStorageClass failed: %v", storageCfg.DefaultStorageClass)
	}
	if storageCfg.OpenEBS == nil || storageCfg.OpenEBS.Enabled == nil || !*storageCfg.OpenEBS.Enabled {
		t.Error("Storage OpenEBS.Enabled failed")
	}
	if storageCfg.OpenEBS.Version == nil || *storageCfg.OpenEBS.Version != "3.3.0" {
		t.Errorf("Storage OpenEBS.Version failed: %v", storageCfg.OpenEBS.Version)
	}
	if storageCfg.OpenEBS.Engines == nil || storageCfg.OpenEBS.Engines.LocalHostPath == nil || storageCfg.OpenEBS.Engines.LocalHostPath.Enabled == nil || !*storageCfg.OpenEBS.Engines.LocalHostPath.Enabled {
		t.Error("Storage OpenEBS.Engines.LocalHostPath.Enabled failed")
	}

	registryCfg := cfg.Spec.Registry
	if registryCfg == nil {
		t.Fatal("Registry config is nil")
	}
	if registryCfg.PrivateRegistry != "myreg.local:5000" {
		t.Errorf("Registry PrivateRegistry failed: %s", registryCfg.PrivateRegistry)
	}
	if len(registryCfg.InsecureRegistries) == 0 || registryCfg.InsecureRegistries[0] != "myreg.local:5000" {
		t.Error("Registry InsecureRegistries failed")
	}
	if auth, ok := registryCfg.Auths["myreg.local:5000"]; !ok || auth.Username != "reguser" {
		t.Error("Registry Auths failed")
	}

	osCfg := cfg.Spec.OS
	if osCfg == nil {
		t.Fatal("OS config is nil")
	}
	if len(osCfg.NtpServers) != 2 || osCfg.NtpServers[0] != "ntp1.example.com" {
		t.Error("OS NtpServers failed")
	}
	if osCfg.Timezone == nil || *osCfg.Timezone != "America/Los_Angeles" {
		t.Errorf("OS Timezone failed: %v", osCfg.Timezone)
	}
	if osCfg.SkipConfigureOS == nil || *osCfg.SkipConfigureOS != false {
		t.Error("OS SkipConfigureOS failed")
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

func TestLoadFromBytes_ValidDockerRuntime(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(validYAMLDockerRuntime))
	if err != nil {
		t.Fatalf("LoadFromBytes with Docker runtime YAML failed: %v", err)
	}
	if cfg.ObjectMeta.Name != "docker-cluster" {
		t.Errorf("ObjectMeta.Name = %s, want docker-cluster", cfg.ObjectMeta.Name)
	}
	if cfg.Spec.ContainerRuntime == nil {
		t.Fatal("Spec.ContainerRuntime is nil")
	}
	if cfg.Spec.ContainerRuntime.Type != "docker" {
		t.Errorf("ContainerRuntime.Type = %s, want docker", cfg.Spec.ContainerRuntime.Type)
	}
	if cfg.Spec.ContainerRuntime.Version != "20.10.17" {
		t.Errorf("ContainerRuntime.Version = %s, want 20.10.17", cfg.Spec.ContainerRuntime.Version)
	}
	dockerCfg := cfg.Spec.ContainerRuntime.Docker
	if dockerCfg == nil {
		t.Fatal("Spec.ContainerRuntime.Docker is nil")
	}
	if dockerCfg.DataRoot == nil || *dockerCfg.DataRoot != "/var/lib/docker-custom" {
		t.Errorf("Docker.DataRoot = %v, want /var/lib/docker-custom", dockerCfg.DataRoot)
	}
	if dockerCfg.LogDriver == nil || *dockerCfg.LogDriver != "journald" {
		t.Errorf("Docker.LogDriver = %v, want journald", dockerCfg.LogDriver)
	}
	if len(dockerCfg.ExecOpts) == 0 || dockerCfg.ExecOpts[0] != "native.cgroupdriver=systemd" {
		t.Errorf("Docker.ExecOpts = %v, want [\"native.cgroupdriver=systemd\"]", dockerCfg.ExecOpts)
	}
	if cfg.Spec.Kubernetes == nil {
		t.Fatal("Spec.Kubernetes is nil")
	}
	if cfg.Spec.Kubernetes.ContainerManager != "systemd" {
		t.Errorf("Kubernetes.ContainerManager = %s, want systemd for Docker with systemd cgroup", cfg.Spec.Kubernetes.ContainerManager)
	}
	if dockerCfg.InstallCRIDockerd == nil || !*dockerCfg.InstallCRIDockerd {
		t.Errorf("Docker.InstallCRIDockerd expected true, got %v", dockerCfg.InstallCRIDockerd)
	}
	if dockerCfg.CRIDockerdVersion == nil || *dockerCfg.CRIDockerdVersion != "v0.3.1" {
		t.Errorf("Docker.CRIDockerdVersion = %v, want 'v0.3.1'", dockerCfg.CRIDockerdVersion)
	}
}

const yamlInvalidAfterDefaults = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: invalid-after-defaults
spec:
  type: kubexm
  hosts:
  # Missing name, address, user which are normally defaulted or inherited.
  # If global is also missing, user will be empty after defaults and fail validation.
  - port: 22
    roles: ["master"]
  # Missing etcd, kubernetes, network sections which are required by Validate_Cluster
`

const yamlValidDueToDefaults = `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: valid-due-to-defaults
spec:
  # type will be defaulted to kubexm
  global:
    user: "testuser" # This will be inherited by host
    # port: 22, connectionTimeout: 30s will be defaulted
  hosts:
  - name: node-1
    address: 10.0.0.1
    roles: ["worker"]
    # port, user will be inherited or defaulted
  etcd: {} # Will be defaulted
  kubernetes:
    version: v1.23.0 # Other k8s fields will be defaulted
  network:
    kubePodsCIDR: 10.244.0.0/16 # Other network fields will be defaulted
`

func TestParseFromFile_ValidationFail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-validation-fail-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid_cluster.yaml")
	if err := os.WriteFile(configPath, []byte(yamlInvalidAfterDefaults), 0644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	_, err = ParseFromFile(configPath)
	if err == nil {
		t.Fatalf("ParseFromFile with YAML that should fail validation after defaults, expected error, got nil")
	}
	// Check for a specific validation error message if possible, e.g. missing host name or required section
	// Example: This YAML will likely fail because etcd, kubernetes, network are missing.
	// Or because host name/address/user are missing and global is not there to provide them.
	expectedErrorSubstrings := []string{
		// Based on current Validate_Cluster, these are likely:
		"spec.hosts[0].name: cannot be empty",
		"spec.hosts[0].address: cannot be empty",
		"spec.hosts[0].user: cannot be empty",
		"spec.etcd: section is required", // Or similar depending on how defaults for Etcd behave
		"spec.kubernetes: section is required",
		"spec.network: section is required",
	}

	foundOne := false
	for _, sub := range expectedErrorSubstrings {
		if strings.Contains(err.Error(), sub) {
			foundOne = true
			break
		}
	}
	if !foundOne {
		t.Errorf("Expected one of validation errors %v, but got: %v", expectedErrorSubstrings, err)
	}
}

func TestParseFromFile_ValidDueToDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-valid-defaults-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "valid_due_to_defaults.yaml")
	if err := os.WriteFile(configPath, []byte(yamlValidDueToDefaults), 0644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	cfg, err := ParseFromFile(configPath)
	if err != nil {
		t.Fatalf("ParseFromFile with YAML that should be valid due to defaults, failed: %v", err)
	}

	// Check some defaulted values
	if cfg.Spec.Type != v1alpha1.ClusterTypeKubeXM {
		t.Errorf("cfg.Spec.Type = %s, want %s (defaulted)", cfg.Spec.Type, v1alpha1.ClusterTypeKubeXM)
	}
	if cfg.Spec.Global.Port != 22 {
		t.Errorf("cfg.Spec.Global.Port = %d, want 22 (defaulted)", cfg.Spec.Global.Port)
	}
	if cfg.Spec.Hosts[0].User != "testuser" {
		t.Errorf("cfg.Spec.Hosts[0].User = %s, want 'testuser' (inherited)", cfg.Spec.Hosts[0].User)
	}
	if cfg.Spec.Etcd.Type != v1alpha1.EtcdTypeKubeXMSInternal { // Default for EtcdConfig
		t.Errorf("cfg.Spec.Etcd.Type = %s, want %s (defaulted)", cfg.Spec.Etcd.Type, v1alpha1.EtcdTypeKubeXMSInternal)
	}
	if cfg.Spec.Kubernetes.DNSDomain != "cluster.local" {
		t.Errorf("cfg.Spec.Kubernetes.DNSDomain = %s, want cluster.local (defaulted)", cfg.Spec.Kubernetes.DNSDomain)
	}
	if cfg.Spec.Network.Plugin != "calico" { // Default for NetworkConfig
		t.Errorf("cfg.Spec.Network.Plugin = %s, want calico (defaulted)", cfg.Spec.Network.Plugin)
	}
}
