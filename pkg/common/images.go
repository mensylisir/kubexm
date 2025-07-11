package common

// Image related constants
const (
	PauseImageName = "pause" // Name of the pause image.
	// DefaultKubeVIPImage is defined in components.go as it includes a version.
	// Default image repositories are in components.go
)

// Default image versions (complementing repositories in components.go and component names)
// These were originally in the main constants.go
const (
	DefaultK8sVersion        = "v1.28.2"  // Example default Kubernetes version.
	DefaultEtcdVersion       = "3.5.10-0" // Example default Etcd version.
	DefaultCoreDNSVersion    = "v1.10.1"  // Example default CoreDNS version.
	DefaultContainerdVersion = "1.7.11"   // Example default Containerd version.
	// DefaultKubeVIPImage includes version, so it's more of a full image identifier.
	// It's currently in components.go: DefaultKubeVIPImageRepository + specific tag.
	// Let's move the full DefaultKubeVIPImage here for consistency of versioned images.
	DefaultKubeVIPImage = "ghcr.io/kube-vip/kube-vip:v0.7.0" // Example default Kube-VIP image.

	// This was in constants.go, specific to binary installs.
	DefaultEtcdVersionForBinInstall = "v3.5.13"
)

// DefaultImageRegistry ("registry.k8s.io") is covered by DefaultK8sImageRegistry in components.go.
// This redundant definition has been removed.
