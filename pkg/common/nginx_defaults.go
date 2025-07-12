package common

// Nginx LoadBalancer specific defaults (complementing port in components.go and path in paths.go)
const (
	DefaultNginxMode        = "tcp"         // Default mode for Nginx LB (TCP).
	DefaultNginxAlgorithm   = "round_robin" // Default load balancing algorithm for Nginx.
	DefaultNginxListenPort  = 6443          // Default port for Nginx LB to listen on.
	DefaultNginxConfigFilePath = "/etc/nginx/nginx.conf" // Default config file path for Nginx.
)
