package common

const (
	DefaultRuncPath             = "/usr/local/bin/runc"
	DefaultRuntimeType          = "oci"
	DefaultRuntimeRoot          = "/run/runc"
	CRIODefaultEndpoint         = "unix:///var/run/crio/crio.sock"
	CRIODefaultVersion          = "1.28.2"
	CRIODefaultGraphRoot        = "/var/lib/containers/storage"
	CRIODefaultRunRoot          = "/var/run/containers/storage"
	CRIODefaultAuthFile         = "/etc/containers/auth.json"
	CRIODefaultConfDir          = "/etc/crio"
	CRIODefaultConfigFile       = "/etc/crio/crio.conf"
	RegistriesDefaultConfDir    = "/etc/containers"
	RegistriesDefaultConfigFile = "/etc/containers/registries.conf"
	CRIODefaultSystemdFile      = "/etc/systemd/system/crio.service"
	CRIODefaultDropInFile       = "/etc/systemd/system/crio.service.d/kubexm.conf"
)

var DefaultUnqualifiedSearchRegistries = []string{"docker.io", "quay.io"}
