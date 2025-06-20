package connector

import (
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// hostImpl implements the Host interface using v1alpha1.HostSpec.
type hostImpl struct {
	spec v1alpha1.HostSpec
	// Potentially store global defaults here if needed for GetPort/GetUser fallbacks
	// globalUser string
	// globalPort int
}

// NewHostFromSpec creates a new Host object from its specification.
// It's a constructor for hostImpl.
// TODO: Consider passing global defaults if spec fields can inherit them.
func NewHostFromSpec(spec v1alpha1.HostSpec /*, globalUser string, globalPort int */) Host {
	return &hostImpl{
		spec: spec,
		// globalUser: globalUser,
		// globalPort: globalPort,
	}
}

func (h *hostImpl) GetName() string {
	return h.spec.Name
}

func (h *hostImpl) GetAddress() string {
	return h.spec.Address
}

func (h *hostImpl) GetPort() int {
	// If port is not set in spec, it should have been defaulted by now
	// either by v1alpha1.SetDefaults_HostSpec or by the RuntimeBuilder
	// based on global config.
	if h.spec.Port == 0 {
		// This indicates a potential issue if defaults were not applied before creating Host.
		// For robustness, could return a common default like 22, but ideally spec is complete.
		return 22 // Fallback default SSH port
	}
	return h.spec.Port
}

func (h *hostImpl) GetUser() string {
	// Similar to GetPort, user should be defaulted if empty in spec.
	// If h.spec.User == "" && h.globalUser != "" {
	// 	return h.globalUser
	// }
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
