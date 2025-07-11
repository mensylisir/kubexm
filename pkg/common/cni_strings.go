package common

// CNI plugin string name constants.
// These are string representations, distinct from CNIType in types.go but can correspond to them.
// Useful when raw string values are needed for configuration, commands, or labels.
const (
	CNICalicoStr    = "calico"
	CNIFlannelStr  = "flannel"
	CNICiliumStr    = "cilium"
	CNIMultusStr    = "multus"
	CNIKubeOvnStr   = "kube-ovn"
	CNIHybridnetStr = "hybridnet"
)
