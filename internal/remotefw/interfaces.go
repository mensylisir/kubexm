// Package remotefw (Remote Framework) provides shared interfaces used by
// upper layers (Step, Task) to reference lower-layer host abstractions
// without importing the connector package directly.
//
// This package lives at the bottom of the dependency graph (Framework layer)
// and has NO internal imports. Both connector and upper-layer packages
// import from remotefw.
//
// Dependency order:
//   remotefw → v1alpha1 (type alias only)
//   connector → remotefw, v1alpha1
//   runner → connector, remotefw
//   step/task → remotefw, runner bridges only
package remotefw

import (
	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
)

// HostSpec is a type alias for v1alpha1.HostSpec.
// It allows connector to import remotefw without creating a cycle
// (since v1alpha1 does not import connector).
//
//go:typealias
type HostSpec = v1alpha1.HostSpec

// Host represents a target host in the cluster. It is implemented by
// connector.Host but referenced from upper layers (step, task, types) via
// this interface to avoid direct connector package imports.
type Host interface {
	GetName() string
	SetName(name string)
	GetAddress() string
	SetAddress(str string)
	GetInternalAddress() string
	GetInternalIPv4Address() string
	GetInternalIPv6Address() string
	SetInternalAddress(str string)
	GetPort() int
	SetPort(port int)
	GetUser() string
	SetUser(u string)
	GetPassword() string
	SetPassword(password string)
	GetPrivateKey() string
	SetPrivateKey(privateKey string)
	GetPrivateKeyPath() string
	SetPrivateKeyPath(path string)
	GetArch() string
	SetArch(arch string)
	GetTimeout() int64
	SetTimeout(timeout int64)
	GetRoles() []string
	SetRoles(roles []string)
	IsRole(role string) bool
	GetHostSpec() HostSpec
}

// localHost is a host implementation for local operations (no SSH required).
type localHost struct {
	spec HostSpec
}

// NewLocalHost creates a local host abstraction for operations that should run
// on the current machine without exposing connector types to upper layers.
func NewLocalHost() Host {
	return &localHost{
		spec: HostSpec{
			Name:    "localhost",
			Address: "127.0.0.1",
			Port:    22,
			User:    "root",
			Arch:    "amd64",
		},
	}
}

func (h *localHost) GetName() string             { return h.spec.Name }
func (h *localHost) SetName(name string)         { h.spec.Name = name }
func (h *localHost) GetAddress() string          { return h.spec.Address }
func (h *localHost) SetAddress(addr string)      { h.spec.Address = addr }
func (h *localHost) GetInternalAddress() string  { return h.spec.InternalAddress }
func (h *localHost) SetInternalAddress(addr string) {
	h.spec.InternalAddress = addr
}
func (h *localHost) GetInternalIPv4Address() string {
	if h.spec.InternalAddress == "" {
		return ""
	}
	return splitOnce(h.spec.InternalAddress, ",")[0]
}
func (h *localHost) GetInternalIPv6Address() string {
	addrs := splitAddresses(h.spec.InternalAddress)
	if len(addrs) >= 2 {
		return addrs[1]
	}
	return ""
}
func (h *localHost) GetPort() int              { return h.spec.Port }
func (h *localHost) SetPort(port int)          { h.spec.Port = port }
func (h *localHost) GetUser() string           { return h.spec.User }
func (h *localHost) SetUser(u string)          { h.spec.User = u }
func (h *localHost) GetPassword() string       { return h.spec.Password }
func (h *localHost) SetPassword(password string) { h.spec.Password = password }
func (h *localHost) GetPrivateKey() string     { return h.spec.PrivateKey }
func (h *localHost) SetPrivateKey(privateKey string) {
	h.spec.PrivateKey = privateKey
}
func (h *localHost) GetPrivateKeyPath() string { return h.spec.PrivateKeyPath }
func (h *localHost) SetPrivateKeyPath(path string) {
	h.spec.PrivateKeyPath = path
}
func (h *localHost) GetArch() string         { return h.spec.Arch }
func (h *localHost) SetArch(arch string)     { h.spec.Arch = arch }
func (h *localHost) GetTimeout() int64       { return h.spec.Timeout }
func (h *localHost) SetTimeout(timeout int64) { h.spec.Timeout = timeout }
func (h *localHost) GetRoles() []string {
	if h.spec.Roles == nil {
		return []string{}
	}
	rolesCopy := make([]string, len(h.spec.Roles))
	copy(rolesCopy, h.spec.Roles)
	return rolesCopy
}
func (h *localHost) SetRoles(roles []string) { h.spec.Roles = roles }
func (h *localHost) IsRole(role string) bool {
	if res, ok := h.spec.RoleTable[role]; ok {
		return res
	}
	return false
}
func (h *localHost) GetHostSpec() HostSpec { return h.spec }

func splitOnce(s, sep string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func splitAddresses(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
