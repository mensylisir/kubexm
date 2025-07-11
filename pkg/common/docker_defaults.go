package common

// Docker specific defaults for daemon configuration.
const (
	DockerDefaultDataRoot               = "/var/lib/docker" // Default data directory for Docker.
	DockerLogOptMaxSizeDefault          = "100m"            // Default max size for Docker log files.
	DockerLogOptMaxFileDefault          = "5"               // Default max number of log files for Docker.
	DockerMaxConcurrentDownloadsDefault = 3                 // Default max concurrent image downloads for Docker.
	DockerMaxConcurrentUploadsDefault   = 5                 // Default max concurrent image uploads for Docker.
	DefaultDockerBridgeName             = "docker0"         // Default bridge name for Docker.
	DockerLogDriverJSONFile             = "json-file"
	DockerLogDriverJournald             = "journald"
	DockerLogDriverSyslog               = "syslog"
	DockerLogDriverFluentd              = "fluentd"
	DockerLogDriverNone                 = "none"
)
