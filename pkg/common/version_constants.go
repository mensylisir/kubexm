package common

// Version constants for components
const (
	// Kubernetes component versions
	DefaultKubernetesVersion = "v1.28.0"      // Default Kubernetes version
	DefaultEtcdVersion      = "v3.5.9"        // Default etcd version
	DefaultDockerVersion    = "24.0.0"        // Default Docker version
	DefaultContainerdVersion = "1.7.0"       // Default containerd version
	DefaultRuncVersion      = "v1.1.7"        // Default runc version
	DefaultCriDockerdVersion = "v0.3.4"       // Default cri-dockerd version
	DefaultHelmVersion      = "v3.12.0"       // Default Helm version
	DefaultCrictlVersion    = "v1.28.0"       // Default crictl version
	
	// Load balancer versions
	DefaultHAProxyVersion   = "2.8.0"         // Default HAProxy version
	DefaultNginxVersion     = "1.24.0"        // Default Nginx version
	DefaultKeepalivedVersion = "2.2.8"        // Default Keepalived version
	DefaultKubeVIPVersion   = "v0.6.0"        // Default kube-vip version
	
	// Additional component versions
	DefaultCoreDNSVersion   = "v1.10.1"       // Default CoreDNS version
	DefaultPauseVersion     = "3.9"           // Default pause container version
	DefaultNodeLocalDNSVersion = "1.22.20"   // Default node-local-dns version
	DefaultRegistryVersion  = "2.8.2"        // Default registry version
	DefaultHarborVersion    = "v2.8.2"        // Default Harbor version
	
	// CNI plugin versions
	DefaultCNIPluginsVersion = "v1.3.0"      // Default CNI plugins version
	DefaultCalicoNodeVersion = "v3.26.1"     // Default Calico node version
	DefaultFlannelImageVersion = "v0.22.0"   // Default Flannel image version
	DefaultCiliumImageVersion = "v1.14.0"    // Default Cilium image version
	DefaultKubeOvnVersion   = "v1.11.5"      // Default Kube-OVN version
	DefaultMultusVersion    = "v3.9.3"       // Default Multus version
	DefaultHybridnetVersion = "v0.8.6"       // Default Hybridnet version
)

// Version compatibility matrix
var (
	// SupportedKubernetesVersions defines the list of supported Kubernetes versions
	SupportedKubernetesVersions = []string{
		"v1.26.0", "v1.26.1", "v1.26.2", "v1.26.3", "v1.26.4", "v1.26.5", "v1.26.6", "v1.26.7", "v1.26.8", "v1.26.9", "v1.26.10", "v1.26.11", "v1.26.12", "v1.26.13", "v1.26.14", "v1.26.15",
		"v1.27.0", "v1.27.1", "v1.27.2", "v1.27.3", "v1.27.4", "v1.27.5", "v1.27.6", "v1.27.7", "v1.27.8", "v1.27.9", "v1.27.10", "v1.27.11", "v1.27.12",
		"v1.28.0", "v1.28.1", "v1.28.2", "v1.28.3", "v1.28.4", "v1.28.5", "v1.28.6", "v1.28.7", "v1.28.8", "v1.28.9",
		"v1.29.0", "v1.29.1", "v1.29.2", "v1.29.3", "v1.29.4",
		"v1.30.0", "v1.30.1", "v1.30.2",
	}
	
	// SupportedEtcdVersions defines the list of supported etcd versions
	SupportedEtcdVersions = []string{
		"v3.5.6", "v3.5.7", "v3.5.8", "v3.5.9", "v3.5.10", "v3.5.11", "v3.5.12", "v3.5.13", "v3.5.14", "v3.5.15",
	}
	
	// SupportedDockerVersions defines the list of supported Docker versions
	SupportedDockerVersions = []string{
		"20.10.0", "20.10.1", "20.10.2", "20.10.3", "20.10.4", "20.10.5", "20.10.6", "20.10.7", "20.10.8", "20.10.9", "20.10.10", "20.10.11", "20.10.12", "20.10.13", "20.10.14", "20.10.15", "20.10.16", "20.10.17", "20.10.18", "20.10.19", "20.10.20", "20.10.21", "20.10.22", "20.10.23", "20.10.24", "20.10.25",
		"23.0.0", "23.0.1", "23.0.2", "23.0.3", "23.0.4", "23.0.5", "23.0.6",
		"24.0.0", "24.0.1", "24.0.2", "24.0.3", "24.0.4", "24.0.5", "24.0.6", "24.0.7", "24.0.8", "24.0.9",
		"25.0.0", "25.0.1", "25.0.2", "25.0.3", "25.0.4", "25.0.5",
		"26.0.0", "26.0.1", "26.0.2", "26.1.0", "26.1.1", "26.1.2", "26.1.3", "26.1.4",
		"27.0.0", "27.0.1", "27.0.2", "27.0.3", "27.1.0", "27.1.1", "27.1.2", "27.2.0", "27.2.1", "27.3.0", "27.3.1",
	}
	
	// SupportedContainerdVersions defines the list of supported containerd versions
	SupportedContainerdVersions = []string{
		"1.6.0", "1.6.1", "1.6.2", "1.6.3", "1.6.4", "1.6.5", "1.6.6", "1.6.7", "1.6.8", "1.6.9", "1.6.10", "1.6.11", "1.6.12", "1.6.13", "1.6.14", "1.6.15", "1.6.16", "1.6.17", "1.6.18", "1.6.19", "1.6.20", "1.6.21", "1.6.22", "1.6.23", "1.6.24", "1.6.25", "1.6.26", "1.6.27", "1.6.28", "1.6.29", "1.6.30", "1.6.31", "1.6.32", "1.6.33", "1.6.34", "1.6.35", "1.6.36",
		"1.7.0", "1.7.1", "1.7.2", "1.7.3", "1.7.4", "1.7.5", "1.7.6", "1.7.7", "1.7.8", "1.7.9", "1.7.10", "1.7.11", "1.7.12", "1.7.13", "1.7.14", "1.7.15", "1.7.16", "1.7.17", "1.7.18", "1.7.19", "1.7.20", "1.7.21", "1.7.22", "1.7.23",
	}
)