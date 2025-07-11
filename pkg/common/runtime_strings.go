package common

// Container Runtime string name constants.
// These are string representations, distinct from ContainerRuntimeType in types.go but can correspond to them.
// Useful when raw string values are needed for configuration, commands, or labels.
const (
	RuntimeDocker     = "docker"
	RuntimeContainerd = "containerd"
	RuntimeCRIO       = "cri-o"
	RuntimeIsula      = "isula"
)
