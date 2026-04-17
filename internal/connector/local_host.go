package connector

import (
	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
)

// localHost is a host implementation for local operations (no SSH required).
type localHost struct {
	spec v1alpha1.HostSpec
}

// NewLocalHost creates a new local host for operations that don't require SSH.
// This is useful for download operations where binaries are downloaded locally
// based on cluster config rather than executed on remote hosts.
func NewLocalHost() Host {
	return &localHost{
		spec: v1alpha1.HostSpec{
			Name:    "localhost",
			Address: "127.0.0.1",
			Port:    22,
			User:    "root",
			Arch:    "amd64",
		},
	}
}

func (h *localHost) GetName() string    { return h.spec.Name }
func (h *localHost) SetName(name string) { h.spec.Name = name }

func (h *localHost) GetAddress() string       { return h.spec.Address }
func (h *localHost) SetAddress(addr string)   { h.spec.Address = addr }

func (h *localHost) GetInternalAddress() string { return h.spec.InternalAddress }
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
func (h *localHost) SetInternalAddress(addr string) { h.spec.InternalAddress = addr }

func (h *localHost) GetPort() int    { return h.spec.Port }
func (h *localHost) SetPort(port int) { h.spec.Port = port }

func (h *localHost) GetUser() string    { return h.spec.User }
func (h *localHost) SetUser(u string)   { h.spec.User = u }

func (h *localHost) GetPassword() string          { return h.spec.Password }
func (h *localHost) SetPassword(password string)   { h.spec.Password = password }

func (h *localHost) GetPrivateKey() string         { return h.spec.PrivateKey }
func (h *localHost) SetPrivateKey(privateKey string) { h.spec.PrivateKey = privateKey }

func (h *localHost) GetPrivateKeyPath() string          { return h.spec.PrivateKeyPath }
func (h *localHost) SetPrivateKeyPath(path string)      { h.spec.PrivateKeyPath = path }

func (h *localHost) GetArch() string { return h.spec.Arch }
func (h *localHost) SetArch(arch string) { h.spec.Arch = arch }

func (h *localHost) GetTimeout() int64    { return h.spec.Timeout }
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

func (h *localHost) GetHostSpec() v1alpha1.HostSpec { return h.spec }

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
