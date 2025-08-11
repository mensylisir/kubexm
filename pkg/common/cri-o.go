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
	SignaturePolicyPath         = "/etc/crio/policy.json"
	CRIORuntimePath             = "/usr/libexec/crio"
	CRIOMonitorPath             = "/usr/libexec/crio"
)

var DefaultUnqualifiedSearchRegistries = []string{"registry.k8s.io", "docker.io", "quay.io"}
