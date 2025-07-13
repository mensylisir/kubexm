package common

const (
	ContainerdDefaultEndpoint    = "unix:///run/containerd/containerd.sock"
	ContainerdSocketPath         = "unix:///run/containerd/containerd.sock"
	ContainerdDefaultConfDir     = "/etc/containerd"
	ContainerdDefaultConfigFile  = "/etc/containerd/config.toml"
	ContainerdDefaultSystemdFile = "/etc/systemd/system/containerd.service"
	ContainerdDefaultDropInFile  = "/etc/systemd/system/containerd.service.d/kubexm.conf"
	ContainerdDefaultRoot        = "/var/lib/containerd"
	ContainerdDefaultState       = "/run/containerd"
	DefaultContainerdVersion     = "1.7.11"
	DefaultRuncVersion           = "v1.1.7"
	ContainerdPluginCRI          = "io.containerd.grpc.v1.cri"
	DefaultContainerdPath        = "/var/lib/containerd"
	DefaultContainerdConfig      = "config.toml"
)
