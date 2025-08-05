package common

const (
	DefaultNginxMode                   = "tcp"
	DefaultNginxAlgorithm              = "round_robin"
	DefaultNginxListenPort             = 6443
	DefaultNginxConfigDir              = "/etc/nginx"
	DefaultNginxConfigFilePath         = "/etc/nginx/nginx.conf"
	DefaultNginxConfigFilePathTarget   = "/etc/nginx/nginx.conf"
	DefaultNginxConfig                 = "nginx.conf"
	DefaultNginxLBModes                = "tcp"
	NginxLBTCPModes                    = "tcp"
	NginxLBHTTPModes                   = "http"
	DefaultNginxLBAlgorithms           = "round_robin"
	NginxLBRoundRobin                  = "round_robin"
	NginxLBLeastConn                   = "least_conn"
	NginxLBLeastTime                   = "least_time"
	NginxLBRandom                      = "random"
	NginxLBIPHash                      = "ip_hash"
	NginxLBHash                        = "hash"
	DefaultNginxHealthCheckMaxFails    = 2
	DefaultNginxHealthCheckFailTimeout = "10s"
	DefaultNginxListenAddress          = "0.0.0.0"
	DefaultNginxLBUpstreamServerWeight = 1
)
