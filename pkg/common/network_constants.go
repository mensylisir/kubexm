package common

// Network-related constants following Kubernetes best practices
const (
	// Default network CIDR blocks
	DefaultKubePodsCIDR    = "10.244.0.0/16"  // Default Pod CIDR for Kubernetes clusters
	DefaultKubeServiceCIDR = "10.96.0.0/12"   // Default Service CIDR for Kubernetes clusters
	DefaultKubeClusterCIDR = "10.244.0.0/16"  // Default Cluster CIDR (alias for Pod CIDR)
	
	// Default DNS settings
	DefaultDNSServiceIP     = "10.96.0.10"    // Default DNS service IP
	DefaultDNSClusterDomain = "cluster.local" // Default cluster domain
	DefaultDNSUpstream      = "8.8.8.8"       // Default upstream DNS server
	DefaultDNSSecondary     = "8.8.4.4"       // Default secondary DNS server
	
	// Default network plugin settings
	DefaultCNIVersion       = "v1.0.0"        // Default CNI version
	DefaultCalicoVersion    = "v3.26.1"       // Default Calico version
	DefaultFlannelVersion   = "v0.22.0"       // Default Flannel version
	DefaultCiliumVersion    = "v1.14.0"       // Default Cilium version
	
	// Default network ports
	DefaultSSHPort          = 22              // Default SSH port
	DefaultAPIServerPort    = 6443            // Default Kubernetes API server port
	DefaultEtcdClientPort   = 2379            // Default etcd client port
	DefaultEtcdPeerPort     = 2380            // Default etcd peer port
)