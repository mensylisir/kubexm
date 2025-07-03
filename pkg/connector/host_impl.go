package connector

import (
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// hostImpl implements the Host interface using v1alpha1.HostSpec.
type hostImpl struct {
	spec v1alpha1.HostSpec
}

// NewHostFromSpec creates a new Host object from its specification.
// It's a constructor for hostImpl.
// Assumes the input spec has already had defaults applied by the config/API layer.
func NewHostFromSpec(spec v1alpha1.HostSpec) Host {
	return &hostImpl{
		spec: spec,
	}
}

func (h *hostImpl) GetName() string {
	return h.spec.Name
}

func (h *hostImpl) GetAddress() string {
	return h.spec.Address
}

func (h *hostImpl) GetPort() int {
	// Assumes h.spec.Port has been defaulted by API layer (e.g., to 22) if it was originally 0.
	// If it's still 0 here, it implies an intentional configuration or an issue upstream in defaulting.
	// The connector should use the value as provided in the spec.
	return h.spec.Port
}

func (h *hostImpl) GetUser() string {
	// Assumes h.spec.User has been defaulted by API layer if it was originally empty.
	return h.spec.User
}

func (h *hostImpl) GetRoles() []string {
	if h.spec.Roles == nil {
		return []string{} // Return empty slice instead of nil for safety
	}
	// Make a copy to prevent external modification if h.spec.Roles is later changed.
	rolesCopy := make([]string, len(h.spec.Roles))
	copy(rolesCopy, h.spec.Roles)
	return rolesCopy
}

func (h *hostImpl) GetHostSpec() v1alpha1.HostSpec {
	return h.spec // Returns a copy of the spec
}

// GetArch returns the architecture of the host from the spec.
// It normalizes common variations like x86_64 to amd64 and aarch64 to arm64.
// It defaults to "amd64" if the Arch field in the spec is empty.
func (h *hostImpl) GetArch() string {
	arch := h.spec.Arch
	if arch == "x86_64" {
		return "amd64"
	}
	if arch == "aarch64" {
		return "arm64"
	}
	// The h.spec.Arch is expected to be defaulted by the API layer if it was initially empty.
	// If it's still empty here, return it as is; further layers might handle or error.
	return arch
}
