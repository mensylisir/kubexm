package common

// CNI plugin string name constants.
// These are string representations, distinct from CNIType in types.go but can correspond to them.
// Useful when raw string values are needed for configuration, commands, or labels.
const (
	CNICalico    = "calico"
	CNIFlannel  = "flannel"
	CNICilium    = "cilium"
	CNIMultus    = "multus"
	CNIKubeOvn   = "kube-ovn"
	CNIHybridnet = "hybridnet"
)
