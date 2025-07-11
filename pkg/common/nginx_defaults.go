package common

// Nginx LoadBalancer specific defaults (complementing port in components.go and path in paths.go)
const (
	DefaultNginxMode      = "stream"      // Default mode for Nginx LB (TCP).
	DefaultNginxAlgorithm = "round_robin" // Default load balancing algorithm for Nginx.
)
