package common

import "time"

const (
	DefaultSSHPort           = 22
	DefaultConnectionTimeout = 30 * time.Second
)

type HostConnectionType string

const (
	HostConnectionTypeSSH   HostConnectionType = "ssh"
	HostConnectionTypeLocal HostConnectionType = "local"
)

const (
	HostTypeSSH   = HostConnectionTypeSSH
	HostTypeLocal = HostConnectionTypeLocal
)
