package connector

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

func TestHostImpl_GettersAndSetters(t *testing.T) {
	spec := v1alpha1.HostSpec{
		Name:            "test-host",
		Address:         "192.168.1.100",
		InternalAddress: "10.0.0.1,fe80::1",
		Port:            22,
		User:            "testuser",
		Password:        "testpass",
		PrivateKey:      "-----BEGIN RSA PRIVATE KEY-----",
		PrivateKeyPath:  "/path/to/key",
		Arch:            "amd64",
		Timeout:         30,
		Roles:           []string{"master", "worker"},
		RoleTable:       map[string]bool{"master": true, "worker": true},
	}

	host := NewHostFromSpec(spec)

	// Test Getters
	t.Run("Getters", func(t *testing.T) {
		if host.GetName() != "test-host" {
			t.Errorf("GetName() = %v, want %v", host.GetName(), "test-host")
		}
		if host.GetAddress() != "192.168.1.100" {
			t.Errorf("GetAddress() = %v, want %v", host.GetAddress(), "192.168.1.100")
		}
		if host.GetInternalAddress() != "10.0.0.1,fe80::1" {
			t.Errorf("GetInternalAddress() = %v, want %v", host.GetInternalAddress(), "10.0.0.1,fe80::1")
		}
		if host.GetInternalIPv4Address() != "10.0.0.1" {
			t.Errorf("GetInternalIPv4Address() = %v, want %v", host.GetInternalIPv4Address(), "10.0.0.1")
		}
		if host.GetInternalIPv6Address() != "fe80::1" {
			t.Errorf("GetInternalIPv6Address() = %v, want %v", host.GetInternalIPv6Address(), "fe80::1")
		}
		if host.GetPort() != 22 {
			t.Errorf("GetPort() = %v, want %v", host.GetPort(), 22)
		}
		if host.GetUser() != "testuser" {
			t.Errorf("GetUser() = %v, want %v", host.GetUser(), "testuser")
		}
		if host.GetPassword() != "testpass" {
			t.Errorf("GetPassword() = %v, want %v", host.GetPassword(), "testpass")
		}
		if host.GetPrivateKey() != "-----BEGIN RSA PRIVATE KEY-----" {
			t.Errorf("GetPrivateKey() = %v, want %v", host.GetPrivateKey(), "-----BEGIN RSA PRIVATE KEY-----")
		}
		if host.GetPrivateKeyPath() != "/path/to/key" {
			t.Errorf("GetPrivateKeyPath() = %v, want %v", host.GetPrivateKeyPath(), "/path/to/key")
		}
		if host.GetArch() != "amd64" {
			t.Errorf("GetArch() = %v, want %v", host.GetArch(), "amd64")
		}
		if host.GetTimeout() != 30 {
			t.Errorf("GetTimeout() = %v, want %v", host.GetTimeout(), 30)
		}
	})

	// Test Setters
	t.Run("Setters", func(t *testing.T) {
		host.SetName("new-host")
		if host.GetName() != "new-host" {
			t.Errorf("After SetName, GetName() = %v, want %v", host.GetName(), "new-host")
		}

		host.SetAddress("192.168.1.200")
		if host.GetAddress() != "192.168.1.200" {
			t.Errorf("After SetAddress, GetAddress() = %v, want %v", host.GetAddress(), "192.168.1.200")
		}

		host.SetInternalAddress("10.0.0.2")
		if host.GetInternalAddress() != "10.0.0.2" {
			t.Errorf("After SetInternalAddress, GetInternalAddress() = %v, want %v", host.GetInternalAddress(), "10.0.0.2")
		}

		host.SetPort(2222)
		if host.GetPort() != 2222 {
			t.Errorf("After SetPort, GetPort() = %v, want %v", host.GetPort(), 2222)
		}

		host.SetUser("newuser")
		if host.GetUser() != "newuser" {
			t.Errorf("After SetUser, GetUser() = %v, want %v", host.GetUser(), "newuser")
		}

		host.SetPassword("newpass")
		if host.GetPassword() != "newpass" {
			t.Errorf("After SetPassword, GetPassword() = %v, want %v", host.GetPassword(), "newpass")
		}

		host.SetPrivateKey("newkey")
		if host.GetPrivateKey() != "newkey" {
			t.Errorf("After SetPrivateKey, GetPrivateKey() = %v, want %v", host.GetPrivateKey(), "newkey")
		}

		host.SetPrivateKeyPath("/new/path")
		if host.GetPrivateKeyPath() != "/new/path" {
			t.Errorf("After SetPrivateKeyPath, GetPrivateKeyPath() = %v, want %v", host.GetPrivateKeyPath(), "/new/path")
		}

		host.SetArch("arm64")
		if host.GetArch() != "arm64" {
			t.Errorf("After SetArch, GetArch() = %v, want %v", host.GetArch(), "arm64")
		}

		host.SetTimeout(60)
		if host.GetTimeout() != 60 {
			t.Errorf("After SetTimeout, GetTimeout() = %v, want %v", host.GetTimeout(), 60)
		}
	})
}

func TestHostImpl_ArchConversion(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		expected string
	}{
		{"x86_64 to amd64", "x86_64", "amd64"},
		{"aarch64 to arm64", "aarch64", "arm64"},
		{"amd64 unchanged", "amd64", "amd64"},
		{"arm64 unchanged", "arm64", "arm64"},
		{"unknown unchanged", "riscv64", "riscv64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := v1alpha1.HostSpec{Arch: tt.arch}
			host := NewHostFromSpec(spec)
			got := host.GetArch()
			if got != tt.expected {
				t.Errorf("GetArch() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHostImpl_Roles(t *testing.T) {
	spec := v1alpha1.HostSpec{
		Roles:     []string{"master"},
		RoleTable: map[string]bool{"master": true},
	}
	host := NewHostFromSpec(spec)

	t.Run("GetRoles", func(t *testing.T) {
		roles := host.GetRoles()
		if len(roles) != 1 || roles[0] != "master" {
			t.Errorf("GetRoles() = %v, want [master]", roles)
		}
	})

	t.Run("SetRoles", func(t *testing.T) {
		newRoles := []string{"worker", "etcd"}
		host.SetRoles(newRoles)
		roles := host.GetRoles()
		if len(roles) != 2 {
			t.Errorf("After SetRoles, GetRoles() length = %v, want 2", len(roles))
		}
	})

	t.Run("IsRole", func(t *testing.T) {
		if !host.IsRole("master") {
			t.Error("IsRole(master) = false, want true")
		}
		if host.IsRole("nonexistent") {
			t.Error("IsRole(nonexistent) = true, want false")
		}
	})

	t.Run("GetRoles_NilSafe", func(t *testing.T) {
		emptySpec := v1alpha1.HostSpec{}
		emptyHost := NewHostFromSpec(emptySpec)
		roles := emptyHost.GetRoles()
		if roles == nil {
			t.Error("GetRoles() should return empty slice, not nil")
		}
		if len(roles) != 0 {
			t.Errorf("GetRoles() length = %v, want 0", len(roles))
		}
	})
}

func TestHostImpl_GetInternalIPv6Address_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		internalAddress string
		expectedIPv6    string
	}{
		{"Both IPv4 and IPv6", "10.0.0.1,fe80::1", "fe80::1"},
		{"Only IPv4", "10.0.0.1", ""},
		{"Empty", "", ""},
		{"Three addresses", "10.0.0.1,fe80::1,extra", "fe80::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := v1alpha1.HostSpec{InternalAddress: tt.internalAddress}
			host := NewHostFromSpec(spec)
			got := host.GetInternalIPv6Address()
			if got != tt.expectedIPv6 {
				t.Errorf("GetInternalIPv6Address() = %v, want %v", got, tt.expectedIPv6)
			}
		})
	}
}

func TestHostImpl_GetHostSpec(t *testing.T) {
	spec := v1alpha1.HostSpec{
		Name:    "test",
		Address: "192.168.1.1",
	}
	host := NewHostFromSpec(spec)

	gotSpec := host.GetHostSpec()
	if gotSpec.Name != spec.Name || gotSpec.Address != spec.Address {
		t.Errorf("GetHostSpec() returned incorrect spec")
	}
}
