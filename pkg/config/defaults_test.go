package config

import (
	"testing"
	"time"
	"reflect" // For DeepEqual
	"fmt" // For Fprintf in detailed map/slice checks
)

func TestSetDefaults_Global(t *testing.T) {
	cfg := &Cluster{Spec: ClusterSpec{Global: GlobalSpec{}}}
	SetDefaults(cfg)

	if cfg.APIVersion != DefaultAPIVersion {
		t.Errorf("Default APIVersion not set, got %s", cfg.APIVersion)
	}
	if cfg.Kind != ClusterKind {
		t.Errorf("Default Kind not set, got %s", cfg.Kind)
	}
	if cfg.Spec.Global.Port != 22 {
		t.Errorf("Global.Port default = %d, want 22", cfg.Spec.Global.Port)
	}
	if cfg.Spec.Global.ConnectionTimeout != 30*time.Second {
		t.Errorf("Global.ConnectionTimeout default = %v, want 30s", cfg.Spec.Global.ConnectionTimeout)
	}
	if cfg.Spec.Global.WorkDir != "/tmp/kubexms_work" {
		t.Errorf("Global.WorkDir default = %s, want /tmp/kubexms_work", cfg.Spec.Global.WorkDir)
	}
}

func TestSetDefaults_HostInheritanceAndDefaults(t *testing.T) {
	cfg := &Cluster{
		Spec: ClusterSpec{
			Global: GlobalSpec{
				User:           "global_user",
				Port:           2222,
				PrivateKeyPath: "/global/.ssh/id_rsa",
				WorkDir:        "/global_work",
			},
			Hosts: []HostSpec{
				{Name: "host1"}, // Should inherit all from global
				{Name: "host2", User: "host2_user", Port: 23, WorkDir: "/host2_work"}, // Overrides some, inherits PKP
				{Name: "host3", PrivateKeyPath: "/host3/.ssh/id_rsa"}, // Inherits User, Port, WorkDir
			},
		},
	}
	SetDefaults(cfg)

	// Host1 checks (full inheritance)
	host1 := cfg.Spec.Hosts[0]
	if host1.User != "global_user" { t.Errorf("Host1.User = %s, want global_user", host1.User) }
	if host1.Port != 2222 { t.Errorf("Host1.Port = %d, want 2222", host1.Port) }
	if host1.PrivateKeyPath != "/global/.ssh/id_rsa" { t.Errorf("Host1.PrivateKeyPath = %s", host1.PrivateKeyPath) }
	if host1.WorkDir != "/global_work" { t.Errorf("Host1.WorkDir = %s, want /global_work", host1.WorkDir) }
	if host1.Type != "ssh" { t.Errorf("Host1.Type = %s, want ssh", host1.Type) }

	// Host2 checks (overrides and inheritance)
	host2 := cfg.Spec.Hosts[1]
	if host2.User != "host2_user" { t.Errorf("Host2.User = %s, want host2_user", host2.User) }
	if host2.Port != 23 { t.Errorf("Host2.Port = %d, want 23", host2.Port) }
	if host2.PrivateKeyPath != "/global/.ssh/id_rsa" { t.Errorf("Host2.PrivateKeyPath = %s", host2.PrivateKeyPath) } // Inherited
	if host2.WorkDir != "/host2_work" { t.Errorf("Host2.WorkDir = %s, want /host2_work", host2.WorkDir) }

	// Host3 checks (mix)
	host3 := cfg.Spec.Hosts[2]
	if host3.User != "global_user" {t.Errorf("Host3.User = %s, want global_user", host3.User)}
	if host3.Port != 2222 {t.Errorf("Host3.Port = %d, want 2222", host3.Port)}
	if host3.PrivateKeyPath != "/host3/.ssh/id_rsa" {t.Errorf("Host3.PrivateKeyPath = %s", host3.PrivateKeyPath)}
	if host3.WorkDir != "/global_work" {t.Errorf("Host3.WorkDir = %s", host3.WorkDir)}

	// Check fallback workdir if global is also empty
	cfgNoGlobalWorkDir := &Cluster{ Spec: ClusterSpec{ Hosts: []HostSpec{{Name: "host4"}} }}
	SetDefaults(cfgNoGlobalWorkDir) // Global WorkDir will be /tmp/kubexms_work, then host4 inherits it.
	if cfgNoGlobalWorkDir.Spec.Hosts[0].WorkDir != "/tmp/kubexms_work" {
	    // If global was empty, it would be /tmp/kubexms_work_host4
	    // But global is defaulted first, then host inherits.
		t.Errorf("Host4.WorkDir = %s, expected /tmp/kubexms_work", cfgNoGlobalWorkDir.Spec.Hosts[0].WorkDir)
	}

	// Check fallback port if global is also 0 (it defaults to 22 first, so host inherits 22)
	cfgNoGlobalPort := &Cluster{ Spec: ClusterSpec{ Global: GlobalSpec{Port:0}, Hosts: []HostSpec{{Name: "host5"}} }}
	SetDefaults(cfgNoGlobalPort)
	if cfgNoGlobalPort.Spec.Hosts[0].Port != 22 {
		t.Errorf("Host5.Port = %d, expected 22", cfgNoGlobalPort.Spec.Hosts[0].Port)
	}

}

func TestSetDefaults_ComponentStructsInitialization(t *testing.T) {
	cfg := &Cluster{Spec: ClusterSpec{
		// All component specs are initially nil pointers
	}}
	SetDefaults(cfg) // SetDefaults should initialize these if they are nil

	if cfg.Spec.ContainerRuntime == nil { t.Fatal("Spec.ContainerRuntime is nil after SetDefaults") }
	if cfg.Spec.ContainerRuntime.Type != "containerd" {
		t.Errorf("ContainerRuntime.Type default = %s, want containerd", cfg.Spec.ContainerRuntime.Type)
	}
	// If ContainerRuntime.Type defaults to "containerd", Containerd spec should also be initialized.
	if cfg.Spec.Containerd == nil {
		t.Fatal("Spec.Containerd is nil after SetDefaults when runtime is containerd")
	}
	if cfg.Spec.Etcd == nil {
		t.Fatal("Spec.Etcd is nil after SetDefaults")
	}
	if cfg.Spec.Etcd.Type != "stacked" {
		t.Errorf("Etcd.Type default = %s, want stacked", cfg.Spec.Etcd.Type)
	}
	if cfg.Spec.Kubernetes == nil {
		t.Fatal("Spec.Kubernetes is nil after SetDefaults")
	}
	if cfg.Spec.Network == nil {
		t.Fatal("Spec.Network is nil after SetDefaults")
	}
	if cfg.Spec.HighAvailability == nil {
		t.Fatal("Spec.HighAvailability is nil after SetDefaults")
	}

	// Check sub-structs of Kubernetes are initialized
	if cfg.Spec.Kubernetes.APIServer == nil { t.Error("K8s.APIServer is nil") }
	if cfg.Spec.Kubernetes.ControllerManager == nil { t.Error("K8s.ControllerManager is nil") }
	if cfg.Spec.Kubernetes.Scheduler == nil { t.Error("K8s.Scheduler is nil") }
	if cfg.Spec.Kubernetes.Kubelet == nil { t.Error("K8s.Kubelet is nil") }
	if cfg.Spec.Kubernetes.KubeProxy == nil { t.Error("K8s.KubeProxy is nil") }
}

func TestSetDefaults_SlicesAndMapsInitialized(t *testing.T) {
    cfg := &Cluster{Spec: ClusterSpec{
        Hosts: []HostSpec{{Name: "host1"}},
		// Other specs that contain slices/maps will be initialized by SetDefaults
		ContainerRuntime: &ContainerRuntimeSpec{}, // Pre-initialize to test internal initialization
		Containerd: &ContainerdSpec{},
		Etcd: &EtcdSpec{},
		Kubernetes: &KubernetesSpec{},
		Addons: []AddonSpec{}, // Pre-initialize to test internal initialization if any
    }}
    SetDefaults(cfg)

    if cfg.Spec.Hosts[0].Roles == nil {
        t.Error("HostSpec.Roles should be initialized to empty slice, not nil")
    }
    if cfg.Spec.Hosts[0].Labels == nil {
        t.Error("HostSpec.Labels should be initialized to empty map, not nil")
    }
    if cfg.Spec.Hosts[0].Taints == nil {
        t.Error("HostSpec.Taints should be initialized to empty slice, not nil")
    }
	// Ensure Containerd maps/slices are init even if Containerd struct was pre-existing
    if cfg.Spec.Containerd.RegistryMirrors == nil {
         t.Error("ContainerdSpec.RegistryMirrors should be initialized, not nil")
    }
    if cfg.Spec.Containerd.InsecureRegistries == nil {
         t.Error("ContainerdSpec.InsecureRegistries should be initialized, not nil")
    }
	if cfg.Spec.Etcd.Nodes == nil {
		t.Error("EtcdSpec.Nodes should be initialized, not nil")
	}
    if cfg.Spec.Kubernetes.FeatureGates == nil {
        t.Error("KubernetesSpec.FeatureGates should be initialized, not nil")
    }
    if cfg.Spec.Addons == nil { // This is top-level slice in ClusterSpec
        t.Error("ClusterSpec.Addons should be initialized, not nil")
    }
}

func TestSetDefaults_KubernetesClusterName(t *testing.T) {
	cfg := &Cluster{Metadata: Metadata{Name: "my-kube-cluster"}}
	SetDefaults(cfg)
	if cfg.Spec.Kubernetes.ClusterName != "my-kube-cluster" {
		t.Errorf("Kubernetes.ClusterName = %s, want 'my-kube-cluster'", cfg.Spec.Kubernetes.ClusterName)
	}

	cfg2 := &Cluster{Metadata: Metadata{Name: "another"}, Spec: ClusterSpec{Kubernetes: &KubernetesSpec{ClusterName: "override"}}}
	SetDefaults(cfg2)
	if cfg2.Spec.Kubernetes.ClusterName != "override" {
		t.Errorf("Kubernetes.ClusterName = %s, want 'override' (not defaulted)", cfg2.Spec.Kubernetes.ClusterName)
	}
}
