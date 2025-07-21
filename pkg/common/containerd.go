package common

const (
	ContainerdDefaultEndpoint    = "unix:///run/containerd/containerd.sock"
	ContainerdSocketPath         = "unix:///run/containerd/containerd.sock"
	ContainerdDefaultConfDir     = "/etc/containerd"
	ContainerdDefaultConfigFile  = "/etc/containerd/config.toml"
	CrictlDefaultConfigFile      = "/etc/crictl.yaml"
	ContainerdDefaultSystemdFile = "/etc/systemd/system/containerd.service"
	ContainerdDefaultDropInFile  = "/etc/systemd/system/containerd.service.d/kubexm.conf"
	ContainerdDefaultRoot        = "/var/lib/containerd"
	ContainerdDefaultState       = "/run/containerd"
	DefaultContainerdVersion     = "2.1.3"
	DefaultRuncVersion           = "v1.3.0"
	ContainerdPluginCRI          = "io.containerd.grpc.v1.cri"
	DefaultContainerdPath        = "/var/lib/containerd"
	DefaultContainerdConfig      = "config.toml"
	DefaultContainerdPauseImage  = "registry.k8s.io/pause:3.9"
)
