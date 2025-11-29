package connector

import (
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"

	"strings"
)

type hostImpl struct {
	spec v1alpha1.HostSpec
}

func NewHostFromSpec(spec v1alpha1.HostSpec) Host {
	return &hostImpl{
		spec: spec,
	}
}

func (h *hostImpl) GetName() string {
	return h.spec.Name
}

func (h *hostImpl) SetName(name string) {
	h.spec.Name = name
}

func (h *hostImpl) GetAddress() string {
	return h.spec.Address
}

func (h *hostImpl) SetAddress(address string) {
	h.spec.Address = address
}

func (h *hostImpl) GetInternalAddress() string {
	return h.spec.InternalAddress
}

func (h *hostImpl) GetInternalIPv4Address() string {
	return strings.Split(h.spec.InternalAddress, ",")[0]
}

func (h *hostImpl) GetInternalIPv6Address() string {
	internalIPv6Address := ""
	nodeAddresses := strings.Split(h.spec.InternalAddress, ",")
	if len(nodeAddresses) >= 2 {
		internalIPv6Address = nodeAddresses[1]
	}
	return internalIPv6Address
}

func (h *hostImpl) SetInternalAddress(address string) {
	h.spec.InternalAddress = address
}

func (h *hostImpl) GetPort() int {
	return h.spec.Port
}

func (h *hostImpl) SetPort(port int) {
	h.spec.Port = port
}

func (h *hostImpl) GetUser() string {
	return h.spec.User
}

func (h *hostImpl) SetUser(u string) {
	h.spec.User = u
}

func (h *hostImpl) GetPassword() string {
	return h.spec.Password
}

func (h *hostImpl) SetPassword(password string) {
	h.spec.Password = password
}

func (h *hostImpl) GetPrivateKey() string {
	return h.spec.PrivateKey
}

func (h *hostImpl) SetPrivateKey(privateKey string) {
	h.spec.PrivateKey = privateKey
}

func (h *hostImpl) GetPrivateKeyPath() string {
	return h.spec.PrivateKeyPath
}

func (h *hostImpl) SetPrivateKeyPath(path string) {
	h.spec.PrivateKeyPath = path
}

func (h *hostImpl) GetArch() string {
	arch := h.spec.Arch
	if arch == common.ArchX8664 {
		return common.ArchAMD64
	}
	if arch == common.ArchAarch64 {
		return common.ArchARM64
	}

	return arch
}

func (h *hostImpl) SetArch(arch string) {
	h.spec.Arch = arch
}

func (h *hostImpl) GetTimeout() int64 {
	return h.spec.Timeout
}

func (h *hostImpl) SetTimeout(timeout int64) {
	h.spec.Timeout = timeout
}

func (h *hostImpl) GetRoles() []string {
	if h.spec.Roles == nil {
		return []string{} // Return empty slice instead of nil for safety
	}
	rolesCopy := make([]string, len(h.spec.Roles))
	copy(rolesCopy, h.spec.Roles)
	return rolesCopy
}

func (h *hostImpl) SetRoles(roles []string) {
	h.spec.Roles = roles
}

func (h *hostImpl) SetRole(role string) {
	h.spec.RoleTable[role] = true
	h.spec.Roles = append(h.spec.Roles, role)
}

func (h *hostImpl) IsRole(role string) bool {
	if res, ok := h.spec.RoleTable[role]; ok {
		return res
	}
	return false
}

func (h *hostImpl) GetHostSpec() v1alpha1.HostSpec {
	return h.spec
}
