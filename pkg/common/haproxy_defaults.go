package common

// HAProxy specific defaults (complementing port in components.go)
const (
	DefaultHAProxyMode      = "tcp"        // Default mode for HAProxy.
	DefaultHAProxyAlgorithm = "roundrobin" // Default load balancing algorithm for HAProxy.
)
