package common

import "time"

// Default SSH connection parameters.
const (
	// DefaultSSHPort is already defined in network_constants.go
	// DefaultSSHPort = 22
	DefaultConnectionTimeout = 30 * time.Second
)
