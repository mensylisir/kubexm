package config

import "time"

// This package is a placeholder to allow pkg/runtime to compile.
// It will be properly implemented as a separate module later.

// Cluster holds the overall cluster configuration.
type Cluster struct {
	Spec ClusterSpec
	// TODO: Add other fields like APIVersion, Kind, Metadata if parsing full YAML.
}

// ClusterSpec defines the specification of the cluster.
type ClusterSpec struct {
	Hosts  []HostSpec
	Global GlobalSpec
	// TODO: Add other cluster-wide configurations (e.g., Etcd, Kubernetes, Network plugin configs)
}

// GlobalSpec contains global configurations for the cluster deployment.
type GlobalSpec struct {
	ConnectionTimeout time.Duration // Example: 30s
	WorkDir           string        // Default work directory on remote hosts
	Verbose           bool          // Global verbosity setting
	IgnoreErr         bool          // Global ignore error setting
	// TODO: Add other global settings (e.g., user, ssh port if not per host)
}

// HostSpec defines the configuration for a single host in the cluster.
type HostSpec struct {
	Name            string
	Address         string
	InternalAddress string
	Port            int
	User            string
	Password        string
	PrivateKey      []byte // Can be loaded from PrivateKeyPath if path is given
	PrivateKeyPath  string
	Roles           []string // List of roles for the host
	Labels          map[string]string
	Type            string // Type of connection, e.g., "ssh", "local". If empty, default to "ssh".
	WorkDir         string // Host-specific work directory
	// TODO: Add host-specific overrides for global settings if needed
}
