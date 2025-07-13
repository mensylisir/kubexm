package common

const (
	DockerDefaultConfDirTarget          = "/etc/docker"
	DockerDefaultConfigFileTarget       = "/etc/docker/daemon.json"
	DockerDefaultAuthFile               = "/etc/docker/auths.json"
	DockerDefaultSystemdFile            = "/etc/systemd/system/docker.service"
	DockerDefaultDropInFile             = "/etc/systemd/system/docker.service.d/kubexm.conf"
	CniDockerdSystemdFile               = "/etc/systemd/system/cri-dockerd.service"
	DefaultDockerEndpoint               = "unix:///var/run/docker.sock"
	DockerSocketPath                    = "unix:///var/run/docker.sock"
	CriDockerdSocketPath                = "/var/run/cri-dockerd.sock"
	DockerDefaultDataRoot               = "/var/lib/docker"
	DefaultDockerPath                   = "/var/lib/docker"
	DefaultDockerConfig                 = "daemon.json"
	DockerLogOptMaxSizeDefault          = "100m"
	DockerLogOptMaxFileDefault          = "5"
	DockerMaxConcurrentDownloadsDefault = 3
	DockerMaxConcurrentUploadsDefault   = 5
	DefaultDockerBridgeName             = "docker0"
	DockerLogDriverJSONFile             = "json-file"
	DockerLogDriverJournald             = "journald"
	DockerLogDriverSyslog               = "syslog"
	DockerLogDriverFluentd              = "fluentd"
	DockerLogDriverNone                 = "none"
	DefaultDockerVersion                = "24.0.0"
	DefaultCriDockerdVersion            = "v0.3.4"
	DockerDefaultPidFile                = "/var/run/docker.pid"
	StorageDriverOverlay2               = "overlay2"
	StorageDriverBtrfs                  = "btrfs"
)
