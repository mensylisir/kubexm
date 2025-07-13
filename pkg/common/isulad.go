package common

const (
	IsuladDefaultConfigFileTarget = "/etc/isulad/daemon.json"
	IsuladDefaultAuthFile         = "/etc/isulad/auths.json"
	IsuladDefaultSystemdFile      = "/etc/systemd/system/isulad.service"
	IsuladDefaultDropInFile       = "/etc/systemd/system/isulad.service.d/kubexm.conf"
	IsuladDefaultEndpoint         = "unix:///var/run/isulad.sock"
	IsuladDefaultPidFile          = "/var/run/isulad.pid"
	IsuladDefaultVersion          = "2.1.3"
	IsuladDefaultDataRoot         = "/var/lib/isulad"
	IsuladLogOptMaxSizeDefault    = "100m"
	IsuladLogOptMaxFileDefault    = "5"
)
