package connector

import (
	"reflect"
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Not strictly needed for these tests as ObjectMeta is not used by hostImpl directly
)

func TestHostImpl(t *testing.T) {
	specNoDefaults := v1alpha1.HostSpec{
		Name:    "node1",
		Address: "192.168.1.100",
		// Port, User, Arch, Roles are zero/nil
	}

	specWithValues := v1alpha1.HostSpec{
		// ObjectMeta: metav1.ObjectMeta{Name: "node2"}, // Name in ObjectMeta is not used by hostImpl.GetName()
		Name:       "node2-name",
		Address:    "10.0.0.2",
		Port:       2222,
		User:       "testuser",
		Arch:       "arm64",
		Roles:      []string{"master", "etcd"},
	}

	specX86Arch := v1alpha1.HostSpec{Name: "node3", Address: "1.1.1.3", Arch: "x86_64"}
	specAArch64Arch := v1alpha1.HostSpec{Name: "node4", Address: "1.1.1.4", Arch: "aarch64"}
	specOtherArch := v1alpha1.HostSpec{Name: "node5", Address: "1.1.1.5", Arch: "riscv64"}


	tests := []struct {
		name            string
		spec            v1alpha1.HostSpec
		expectedName    string
		expectedAddr    string
		expectedPort    int
		expectedUser    string
		expectedArch    string
		expectedRoles   []string
		expectedRawSpec v1alpha1.HostSpec // To check if GetHostSpec returns the original spec
	}{
		{
			name:            "spec_with_zero_values_for_port_user_arch_roles",
			spec:            specNoDefaults,
			expectedName:    "node1",
			expectedAddr:    "192.168.1.100",
			expectedPort:    0,  // No fallback in GetPort(); expects spec to be defaulted.
			expectedUser:    "", // No fallback in GetUser(); expects spec to be defaulted.
			expectedArch:    "", // No fallback in GetArch(); expects spec to be defaulted. Normalized from empty is empty.
			expectedRoles:   []string{},
			expectedRawSpec: specNoDefaults,
		},
		{
			name:            "spec_with_all_values_set",
			spec:            specWithValues,
			expectedName:    "node2-name",
			expectedAddr:    "10.0.0.2",
			expectedPort:    2222,
			expectedUser:    "testuser",
			expectedArch:    "arm64", // Direct value
			expectedRoles:   []string{"master", "etcd"},
			expectedRawSpec: specWithValues,
		},
		{
			name:            "arch_normalization_x86_64",
			spec:            specX86Arch, // Arch: "x86_64", Port: 0, User: ""
			expectedName:    "node3",
			expectedAddr:    "1.1.1.3",
			expectedPort:    0,    // Expect 0 as spec.Port is 0
			expectedUser:    "",
			expectedArch:    "amd64", // Normalized
			expectedRoles:   []string{},
			expectedRawSpec: specX86Arch,
		},
		{
			name:            "arch_normalization_aarch64",
			spec:            specAArch64Arch, // Arch: "aarch64", Port: 0, User: ""
			expectedName:    "node4",
			expectedAddr:    "1.1.1.4",
			expectedPort:    0,    // Expect 0 as spec.Port is 0
			expectedUser:    "",
			expectedArch:    "arm64", // Normalized
			expectedRoles:   []string{},
			expectedRawSpec: specAArch64Arch,
		},
		{
			name:            "arch_other_value",
			spec:            specOtherArch, // Arch: "riscv64", Port: 0, User: ""
			expectedName:    "node5",
			expectedAddr:    "1.1.1.5",
			expectedPort:    0,    // Expect 0 as spec.Port is 0
			expectedUser:    "",
			expectedArch:    "riscv64", // Unchanged
			expectedRoles:   []string{},
			expectedRawSpec: specOtherArch,
		},
		{
			name: "roles_is_nil_in_spec",
			spec: v1alpha1.HostSpec{Name: "node6", Address: "1.1.1.6", Roles: nil}, // Port:0, User:"", Arch:""
			expectedName:    "node6",
			expectedAddr:    "1.1.1.6",
			expectedPort:    0,    // Expect 0
			expectedUser:    "",
			expectedArch:    "",   // Expect "" as spec.Arch is "", normalization doesn't change empty to amd64
			expectedRoles:   []string{},
			expectedRawSpec: v1alpha1.HostSpec{Name: "node6", Address: "1.1.1.6", Roles: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := NewHostFromSpec(tt.spec)

			if got := host.GetName(); got != tt.expectedName {
				t.Errorf("GetName() = %v, want %v", got, tt.expectedName)
			}
			if got := host.GetAddress(); got != tt.expectedAddr {
				t.Errorf("GetAddress() = %v, want %v", got, tt.expectedAddr)
			}
			if got := host.GetPort(); got != tt.expectedPort {
				t.Errorf("GetPort() = %v, want %v", got, tt.expectedPort)
			}
			if got := host.GetUser(); got != tt.expectedUser {
				t.Errorf("GetUser() = %v, want %v", got, tt.expectedUser)
			}
			if got := host.GetArch(); got != tt.expectedArch {
				t.Errorf("GetArch() = %v, want %v", got, tt.expectedArch)
			}
			if gotRoles := host.GetRoles(); !reflect.DeepEqual(gotRoles, tt.expectedRoles) {
				t.Errorf("GetRoles() = %v, want %v", gotRoles, tt.expectedRoles)
			}

			// Test that GetRoles returns a copy
			if len(tt.spec.Roles) > 0 {
				// Create a host instance for this specific test part to avoid interference
				hostForRoleCopyTest := NewHostFromSpec(tt.spec)
				rolesFromGetter := hostForRoleCopyTest.GetRoles()
				if len(rolesFromGetter) > 0 { // Ensure there's something to modify
					originalFirstRole := rolesFromGetter[0]
					rolesFromGetter[0] = "modified_role_in_copy" // Modify the copy
					// Get roles again from a fresh call to ensure internal state wasn't changed
					freshRoles := NewHostFromSpec(tt.spec).GetRoles()
					if len(freshRoles) > 0 && freshRoles[0] == "modified_role_in_copy" {
						t.Errorf("GetRoles() did not return a copy, internal spec's role was modified. Expected %s, got %s", originalFirstRole, freshRoles[0])
					}
				}
			}


			if gotSpec := host.GetHostSpec(); !reflect.DeepEqual(gotSpec, tt.expectedRawSpec) {
				t.Errorf("GetHostSpec() = %#v, want %#v", gotSpec, tt.expectedRawSpec)
			}
			// Test that GetHostSpec returns a copy
			hostForSpecCopyTest := NewHostFromSpec(tt.spec)
			returnedSpec := hostForSpecCopyTest.GetHostSpec()
			originalName := returnedSpec.Name
			returnedSpec.Name = "modified_spec_name_in_copy" // Modify the copy

			freshSpec := NewHostFromSpec(tt.spec).GetHostSpec()
			if freshSpec.Name == "modified_spec_name_in_copy" {
				 t.Errorf("GetHostSpec() did not return a copy, internal spec's name was modified. Expected %s, got %s", originalName, freshSpec.Name)
			}
		})
	}
}
