package config

import (
	"strings"
	"testing"
	// "time" // Not needed for these validation tests
)

// Helper to create a basic valid config for modification in tests
func newValidBaseConfig() Cluster {
	return Cluster{
		APIVersion: DefaultAPIVersion, Kind: ClusterKind,
		Metadata: Metadata{Name: "valid-cluster"},
		Spec: ClusterSpec{
			Global: GlobalSpec{User: "testuser", Port: 22, ConnectionTimeout: 30 * time.Second, WorkDir: "/tmp"},
			Hosts:  []HostSpec{{Name: "m1", Address: "1.1.1.1", Roles: []string{"master"}}},
			ContainerRuntime: &ContainerRuntimeSpec{Type: "containerd"},
			Containerd:       &ContainerdSpec{}, // Initialized by defaults
			Etcd:             &EtcdSpec{Type: "stacked"},      // Initialized by defaults
			Kubernetes:       &KubernetesSpec{Version: "v1.25.0"}, // Initialized by defaults
			Network:          &NetworkSpec{},      // Initialized by defaults
			HighAvailability: &HighAvailabilitySpec{}, // Initialized by defaults
			Addons:           []AddonSpec{},       // Initialized by defaults
		},
	}
}


func TestValidate_ValidMinimalAfterDefaults(t *testing.T) {
	cfg := newValidBaseConfig()
	// SetDefaults would have been called by Load functions. For direct Validate test, call it.
	SetDefaults(&cfg)
	err := Validate(&cfg)
	if err != nil {
		t.Errorf("Validate() with a minimal valid config (after defaults) failed: %v", err)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Cluster
		wantErr string
	}{
		{"missing metadata.name", func() Cluster { c := newValidBaseConfig(); c.Metadata.Name = ""; return c }(), "metadata.name: cannot be empty"},
		{"missing hosts", func() Cluster { c := newValidBaseConfig(); c.Spec.Hosts = []HostSpec{}; return c }(), "spec.hosts: must contain at least one host"},
		{"missing host.name", func() Cluster { c := newValidBaseConfig(); c.Spec.Hosts[0].Name = ""; return c }(), ".name: cannot be empty"},
		{"missing host.address", func() Cluster { c := newValidBaseConfig(); c.Spec.Hosts[0].Address = ""; return c }(), ".address: cannot be empty"},
		{"missing k8s version", func() Cluster { c := newValidBaseConfig(); c.Spec.Kubernetes.Version = ""; return c }(), "spec.kubernetes.version: cannot be empty"},
		{"missing host user (after global user also empty)", func() Cluster {
			c := newValidBaseConfig();
			c.Spec.Global.User = ""; // Clear global user
			c.Spec.Hosts[0].User = "";   // Clear host user (would inherit from empty global)
			return c
		}(), ".user: cannot be empty"},
		{"missing host auth (ssh type)", func() Cluster {
			c := newValidBaseConfig()
			c.Spec.Global.Password = ""
			c.Spec.Global.PrivateKeyPath = ""
			c.Spec.Hosts[0].Password = ""
			c.Spec.Hosts[0].PrivateKey = ""
			c.Spec.Hosts[0].PrivateKeyPath = ""
			c.Spec.Hosts[0].Type = "ssh" // Explicitly SSH
			return c
		}(), "no SSH authentication method provided"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Defaults are applied first, then validation.
			// For some tests, defaults might fix the issue (e.g. APIVersion/Kind).
			// For others (like missing name), defaults won't fix it.
			SetDefaults(&tt.cfg)
			err := Validate(&tt.cfg)
			if err == nil {
				t.Fatalf("Validate() expected error for %s, got nil", tt.name)
			}
			validationErrs, ok := err.(*ValidationErrors)
			if !ok {
				t.Fatalf("Validate() error for %s is not ValidationErrors type: %T", tt.name, err)
			}
			found := false
			for _, eStr := range validationErrs.Errors {
				if strings.Contains(eStr, tt.wantErr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Validate() error for %s = %v, want to contain %q", tt.name, validationErrs, tt.wantErr)
			}
		})
	}
}

func TestValidate_InvalidValues(t *testing.T) {
	// Test invalid host port
	cfgInvalidPort := newValidBaseConfig()
	cfgInvalidPort.Spec.Hosts[0].Port = 70000 // Invalid port
	SetDefaults(&cfgInvalidPort) // Apply defaults (which might set port if it was 0)
	err := Validate(&cfgInvalidPort)
	if err == nil || !strings.Contains(err.Error(), "port: 70000 is invalid") {
		t.Errorf("Expected port validation error, got: %v", err)
	}

	// Test invalid PodSubnet CIDR
	cfgInvalidCIDR := newValidBaseConfig()
	cfgInvalidCIDR.Spec.Kubernetes.PodSubnet = "invalid-cidr"
	SetDefaults(&cfgInvalidCIDR)
	err = Validate(&cfgInvalidCIDR)
	if err == nil || !strings.Contains(err.Error(), "podSubnet: invalid CIDR 'invalid-cidr'") {
		t.Errorf("Expected podSubnet CIDR validation error, got: %v", err)
	}

	// Test invalid Host Address (not IP, not valid hostname)
	cfgInvalidHostAddr := newValidBaseConfig()
	cfgInvalidHostAddr.Spec.Hosts[0].Address = "not an ip or host!!" // Contains invalid chars
	SetDefaults(&cfgInvalidHostAddr)
	err = Validate(&cfgInvalidHostAddr)
	if err == nil || !strings.Contains(err.Error(), "not a valid IP address or hostname") {
		t.Errorf("Expected host address validation error for '!!', got: %v", err)
	}

	cfgInvalidHostAddr2 := newValidBaseConfig()
	cfgInvalidHostAddr2.Spec.Hosts[0].Address = "-leadinghyphen.com"
	SetDefaults(&cfgInvalidHostAddr2)
	err = Validate(&cfgInvalidHostAddr2)
	// Note: the regex used in Validate might allow this if not careful, depending on its strictness.
	// The provided regex `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`
	// should correctly reject this.
	if err == nil || !strings.Contains(err.Error(), "not a valid IP address or hostname") {
		t.Errorf("Expected host address validation error for leading hyphen, got: %v", err)
	}

	// Test invalid ContainerRuntime Type
	cfgInvalidRuntime := newValidBaseConfig()
	cfgInvalidRuntime.Spec.ContainerRuntime.Type = "cri-o" // Assuming "cri-o" is not in our valid list for now
	SetDefaults(&cfgInvalidRuntime)
	err = Validate(&cfgInvalidRuntime)
	if err == nil || !strings.Contains(err.Error(), "containerRuntime.type: invalid type 'cri-o'") {
		t.Errorf("Expected container runtime type validation error, got: %v", err)
	}
}

func TestValidate_DuplicateHostNames(t *testing.T) {
	cfg := newValidBaseConfig() // Start with a valid base
	cfg.Spec.Hosts = append(cfg.Spec.Hosts, HostSpec{Name:"m1", Address:"1.1.1.2", Port:22, User:"u"}) // Add duplicate name

	SetDefaults(&cfg)
	err := Validate(&cfg)
	if err == nil || !strings.Contains(err.Error(), ".name: 'm1' is duplicated") {
		// The error message includes path like spec.hosts[1:m1]
		t.Errorf("Expected duplicate hostname error, got: %v", err)
	}
}

func TestValidationErrors_ErrorMethod(t *testing.T) {
	ve := &ValidationErrors{}
	if ve.Error() != "no validation errors" {
		t.Errorf("Empty ValidationErrors.Error() incorrect: %s", ve.Error())
	}
	ve.Add("error 1")
	if ve.Error() != "error 1" {
		t.Errorf("Single error string incorrect: %s", ve.Error())
	}
	ve.Add("error 2: %s", "detail")
	expected := "2 validation errors occurred:\n\t* error 1\n\t* error 2: detail"
	if ve.Error() != expected {
		t.Errorf("Multiple errors string incorrect. Got:\n%s\nWant:\n%s", ve.Error(), expected)
	}
}
