package common

// HAProxy specific defaults (complementing port in components.go)
const (
	DefaultHAProxyMode                = "tcp"
	HAProxyModeTCP                    = "tcp"
	HAProxyModeHTTP                   = "http"
	DefaultHAProxyAlgorithm           = "roundrobin"
	HAProxyRoundrobin                 = "roundrobin"
	HAProxyStaticRR                   = "static-rr"
	HAProxyLeastconn                  = "leastconn"
	HAProxyFirst                      = "first"
	HAProxySource                     = "source"
	HAProxyURI                        = "uri"
	HAProxyUrlParam                   = "url_param"
	HAProxyHdr                        = "hdr"
	HAProxyRdpCookie                  = "rdp-cookie"
	DefaultHaproxyHealthCheckInterval = "2s"
	DefaultHaproxyRise                = 2
	DefaultHaproxyFall                = 3
	DefaultHaproxyFrontendBindAddress = "0.0.0.0"
	DefaultHaproxyFrontendPort        = "6443"
	DefaultHAProxyWeight              = 1
)

const (
	HAProxyDefaultConfDirTarget    = "/etc/haproxy"
	HAProxyDefaultConfigFileTarget = "/etc/haproxy/haproxy.cfg"
	HAProxyDefaultSystemdFile      = "/etc/systemd/system/haproxy.service"
	DefaultHAProxyConfig           = "haproxy.cfg"
)
