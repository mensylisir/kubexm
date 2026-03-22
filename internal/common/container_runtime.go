package common

type ContainerRuntimeType string

const (
	RuntimeTypeDocker     ContainerRuntimeType = "docker"
	RuntimeTypeContainerd ContainerRuntimeType = "containerd"
	RuntimeTypeCRIO       ContainerRuntimeType = "cri-o"
	RuntimeTypeIsula      ContainerRuntimeType = "isula"
)
