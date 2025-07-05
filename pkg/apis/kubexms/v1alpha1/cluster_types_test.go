package v1alpha1

import (
	"strings"
	"testing"
	"time"
	// "reflect" // For DeepEqual if needed for complex structs, not used in these initial tests
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- Test SetDefaults_Cluster ---

func TestSetDefaults_Cluster_TypeMetaAndGlobal(t *testing.T) {
	cfg := &Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test"}} // Basic cfg
	SetDefaults_Cluster(cfg)

	if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
		t.Errorf("Default APIVersion not set correctly, got %s, want %s", cfg.APIVersion, SchemeGroupVersion.String())
	}
	if cfg.Kind != "Cluster" {
		t.Errorf("Default Kind not set correctly, got %s, want Cluster", cfg.Kind)
	}
	// ClusterSpec.Type was removed. Defaulting is now handled by KubernetesConfig.Type.
	if cfg.Spec.Global == nil {
		t.Fatal("Spec.Global should be initialized by SetDefaults_Cluster")
	}
	if cfg.Spec.Global.Port != 22 {
		t.Errorf("Global.Port default = %d, want 22", cfg.Spec.Global.Port)
	}
	if cfg.Spec.Global.ConnectionTimeout != 30*time.Second {
		t.Errorf("Global.ConnectionTimeout default = %v, want 30s", cfg.Spec.Global.ConnectionTimeout)
	}
	if cfg.Spec.Global.WorkDir != "/tmp/kubexms_work" { // As per current SetDefaults_Cluster
		t.Errorf("Global.WorkDir default = %s, want /tmp/kubexms_work", cfg.Spec.Global.WorkDir)
	}
}

func TestValidate_Cluster_HostLocalType(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Type = "local"
	cfg.Spec.Hosts[0].Password = "" // SSH details not needed for local
	cfg.Spec.Hosts[0].PrivateKeyPath = ""
	cfg.Spec.Hosts[0].PrivateKey = ""
	// SetDefaults_Cluster is called in newValidV1alpha1ClusterForTest,
	// but we've mutated after. We rely on Validate_Cluster's logic for host type.
	err := Validate_Cluster(cfg)
	if err != nil {
		t.Errorf("Validate_Cluster() for host type 'local' failed: %v", err)
	}

	// Now test if a non-local host *without* ssh details fails
	cfg = newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Type = "ssh" // explicit or default
	cfg.Spec.Hosts[0].Password = ""
	cfg.Spec.Global.PrivateKeyPath = "" // Ensure global default is also empty for this test
	cfg.Spec.Hosts[0].PrivateKeyPath = ""
	cfg.Spec.Hosts[0].PrivateKey = ""
	// Re-apply defaults to ensure host.User etc. are set from global, but SSH creds remain empty.
	// This is tricky because newValidV1alpha1ClusterForTest already sets a global PrivateKeyPath.
	// We need a more controlled setup for this specific test.

	cfgClean := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ssh-no-creds"},
		Spec: ClusterSpec{
			Global:     &GlobalSpec{User: "test", Port: 22, WorkDir: "/tmp"}, // No SSH creds in global
			Hosts:      []HostSpec{{Name: "ssh-host", Address: "1.2.3.4", Type: "ssh"}}, // Explicitly ssh, no creds
			Kubernetes: &KubernetesConfig{Version: "v1.25.0"},
			Network:    &NetworkConfig{KubePodsCIDR: "10.244.0.0/16"},
			Etcd:       &EtcdConfig{},
			ControlPlaneEndpoint: &ControlPlaneEndpointSpec{Address: "1.2.3.4"},
		},
	}
	SetDefaults_Cluster(cfgClean) // Apply defaults to the clean config

	err = Validate_Cluster(cfgClean)
	if err == nil {
		t.Error("Validate_Cluster() expected error for non-local host without SSH details, but got nil")
	} else if !strings.Contains(err.Error(), "no SSH authentication method provided for non-local host") {
		t.Errorf("Validate_Cluster() error for non-local host without SSH details = %v, want to contain 'no SSH authentication method provided'", err)
	}
}

func TestSetDefaults_Cluster_HostInheritanceAndDefaults(t *testing.T) {
	cfg := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-host-defaults"},
		Spec: ClusterSpec{
			Global: &GlobalSpec{ // Global must be non-nil for host inheritance tests
				User:           "global_user",
				Port:           2222,
				PrivateKeyPath: "/global/.ssh/id_rsa",
				WorkDir:        "/global_work",
			},
			Hosts: []HostSpec{
				{Name: "host1"}, // Should inherit all from global
				{Name: "host2", User: "host2_user", Port: 23}, // Overrides some
				{Name: "host3", PrivateKeyPath: "/host3/.ssh/id_rsa"},
			},
		},
	}
	SetDefaults_Cluster(cfg) // This will also call SetDefaults for sub-components if they exist

	// Host1 checks
	host1 := cfg.Spec.Hosts[0]
	if host1.User != "global_user" { t.Errorf("Host1.User = '%s', want 'global_user'", host1.User) }
	if host1.Port != 2222 { t.Errorf("Host1.Port = %d, want 2222", host1.Port) }
	if host1.PrivateKeyPath != "/global/.ssh/id_rsa" { t.Errorf("Host1.PrivateKeyPath = '%s'", host1.PrivateKeyPath) }
	if host1.Type != "ssh" { t.Errorf("Host1.Type = '%s', want 'ssh'", host1.Type) }
	if host1.Labels == nil {t.Error("Host1.Labels should be initialized")}
	if host1.Roles == nil {t.Error("Host1.Roles should be initialized")}
	if host1.Taints == nil {t.Error("Host1.Taints should be initialized")}


	// Host2 checks
	host2 := cfg.Spec.Hosts[1]
	if host2.User != "host2_user" { t.Errorf("Host2.User = '%s', want 'host2_user'", host2.User) }
	if host2.Port != 23 { t.Errorf("Host2.Port = %d, want 23", host2.Port) }
	if host2.PrivateKeyPath != "/global/.ssh/id_rsa" {t.Errorf("Host2.PrivateKeyPath = '%s'", host2.PrivateKeyPath)}


	// Test host defaulting when GlobalSpec is nil (should use hardcoded defaults for host fields)
	cfgNoGlobal := &Cluster{
	   ObjectMeta: metav1.ObjectMeta{Name: "no-global"},
	   Spec: ClusterSpec{
		   Global: nil, // Explicitly nil Global
		   Hosts: []HostSpec{{Name: "hostOnly"}},
	   },
	}
	SetDefaults_Cluster(cfgNoGlobal)
	hostOnly := cfgNoGlobal.Spec.Hosts[0]
	if hostOnly.Port != 22 { // Port defaults to 22 if Global is nil, then host inherits (which is also 22)
	   // This needs to be more carefully checked against SetDefaults_Cluster logic.
	   // SetDefaults_Cluster initializes cfg.Spec.Global if nil.
	   // So hostOnly.Port will be inherited from the defaulted Global.Port (22).
	   t.Errorf("hostOnly.Port = %d, want 22 (from defaulted global)", hostOnly.Port)
	}
	if hostOnly.Type != "ssh" {t.Errorf("hostOnly.Type = %s, want ssh", hostOnly.Type)}

}

func TestSetDefaults_Cluster_ComponentStructsInitialization(t *testing.T) {
	cfg := &Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-components"}}
	// Spec fields are initially nil pointers
	SetDefaults_Cluster(cfg)

	if cfg.Spec.ContainerRuntime == nil { t.Error("Spec.ContainerRuntime is nil after SetDefaults_Cluster") }
	if cfg.Spec.Etcd == nil { t.Error("Spec.Etcd is nil after SetDefaults_Cluster") }
	if cfg.Spec.Kubernetes == nil { t.Error("Spec.Kubernetes is nil after SetDefaults_Cluster") }
	if cfg.Spec.Network == nil { t.Error("Spec.Network is nil after SetDefaults_Cluster") }
	if cfg.Spec.HighAvailability == nil { t.Error("Spec.HighAvailability is nil after SetDefaults_Cluster") }
	if cfg.Spec.Preflight == nil { t.Error("Spec.Preflight is nil after SetDefaults_Cluster") }
	if cfg.Spec.System == nil { t.Error("Spec.System (which includes former Kernel) is nil after SetDefaults_Cluster") }
	if cfg.Spec.Addons == nil { t.Error("Spec.Addons should be initialized to empty slice, not nil") }
	// DNS is a non-pointer field in ClusterSpec, so it's part of the struct.
	// SetDefaults_DNS(&cfg.Spec.DNS) is called in SetDefaults_Cluster.
	// Check a DNS default
	if cfg.Spec.DNS.CoreDNS.UpstreamDNSServers == nil { // Assuming UpstreamDNSServers is defaulted to empty slice
		t.Error("Spec.DNS.CoreDNS.UpstreamDNSServers should be initialized by SetDefaults_DNS via SetDefaults_Cluster")
	}
	if cfg.Spec.System.SysctlParams == nil {
		t.Error("Spec.System.SysctlParams should be initialized by SetDefaults_SystemSpec via SetDefaults_Cluster")
	}
}

// --- Test Validate_Cluster ---

// Helper to create a minimally valid Cluster config for validation tests
func newValidV1alpha1ClusterForTest() *Cluster {
	cfg := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "valid-cluster"},
		Spec: ClusterSpec{
			Global: &GlobalSpec{User: "testuser", Port: 22, PrivateKeyPath: "/dev/null", WorkDir: "/tmp", ConnectionTimeout: 5 * time.Second}, // Added PrivateKeyPath to Global for host inheritance
			Hosts:  []HostSpec{{Name: "m1", Address: "1.1.1.1", Port: 22, User: "testuser", Roles: []string{"master"}}}, // Will inherit PrivateKeyPath
			Kubernetes: &KubernetesConfig{Version: "v1.25.0"}, // PodSubnet removed
			Network:    &NetworkConfig{KubePodsCIDR: "10.244.0.0/16"}, // PodSubnet equivalent moved here
			Etcd:       &EtcdConfig{},    // Etcd section is required by Validate_Cluster
			ControlPlaneEndpoint: &ControlPlaneEndpointSpec{Address: "1.2.3.4"}, // Ensure CPE is valid by default
			// DNS is value type, will be zero-value initialized. SetDefaults_DNS will be called.
			// Other components can be nil if their sections are optional and their Validate_* funcs handle nil
		},
	}
	// Apply full defaults before validation, as Validate_Cluster expects this.
	SetDefaults_Cluster(cfg)
	return cfg
}

func TestValidate_Cluster_ValidMinimal(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	// SetDefaults_Cluster is called in newValidV1alpha1ClusterForTest
	err := Validate_Cluster(cfg)
	if err != nil {
		t.Errorf("Validate_Cluster() with a minimal valid config failed: %v", err)
	}
}

func TestValidate_Cluster_TypeMeta(t *testing.T) {
   cfg := newValidV1alpha1ClusterForTest()
   cfg.APIVersion = "wrong.group/v1beta1"
   cfg.Kind = "NotCluster"
   err := Validate_Cluster(cfg)
   if err == nil { t.Fatal("Expected validation error for TypeMeta, got nil") }
   verrs := err.(*ValidationErrors)
   if !strings.Contains(verrs.Error(), "apiVersion: must be") {t.Errorf("Missing APIVersion error: %v", verrs)}
   if !strings.Contains(verrs.Error(), "kind: must be Cluster") {t.Errorf("Missing Kind error: %v", verrs)}
}


func TestValidate_Cluster_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutator func(c *Cluster) // Function to make the config invalid
		wantErr string
	}{
		{"missing metadata.name", func(c *Cluster) { c.ObjectMeta.Name = "" }, "metadata.name: cannot be empty"},
		{"missing hosts", func(c *Cluster) { c.Spec.Hosts = []HostSpec{} }, "spec.hosts: must contain at least one host"},
		// ClusterSpec.Type related test removed.
		{"missing host.name", func(c *Cluster) { c.Spec.Hosts[0].Name = "" }, "spec.hosts[0].name: cannot be empty"},
		{"missing host.address", func(c *Cluster) { c.Spec.Hosts[0].Address = "" }, "spec.hosts[0:m1].address: cannot be empty"},
		{"missing host.user (after global also empty)", func(c *Cluster) {
			c.Spec.Global.User = "" // Ensure global user is empty
			c.Spec.Hosts[0].User = "" // Host user relies on global default
		}, "spec.hosts[0:m1].user: cannot be empty (after defaults)"},
		{"missing k8s section", func(c *Cluster) { c.Spec.Kubernetes = nil }, "spec.kubernetes: section is required"},
		// Note: If Kubernetes section is nil, Validate_Cluster adds "spec.kubernetes: section is required".
		// If Kubernetes section is present but version is empty, Validate_KubernetesConfig handles that.
		{"missing etcd section", func(c *Cluster) { c.Spec.Etcd = nil }, "spec.etcd: section is required"},
		{"missing network section", func(c *Cluster) { c.Spec.Network = nil }, "spec.network: section is required"},
		{"invalid DNS config", func(c *Cluster) { c.Spec.DNS.CoreDNS.UpstreamDNSServers = []string{""} }, "spec.dns.coredns.upstreamDNSServers[0]: server address cannot be empty"},
		{"invalid System config", func(c *Cluster) { c.Spec.System = &SystemSpec{NTPServers: []string{""}} }, "spec.system.ntpServers[0]: NTP server address cannot be empty"},
		{"rolegroup master host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{Master: MasterRoleSpec{Hosts: []string{"unknown-host"}}}
		}, "spec.roleGroups.master.hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup worker host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{Worker: WorkerRoleSpec{Hosts: []string{"m1", "unknown-host"}}} // m1 is valid
		}, "spec.roleGroups.worker.hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup custom host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "cg", Hosts: []string{"unknown-host"}}}}
		}, "spec.roleGroups.customRoles[0:cg].hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup valid hosts", func(c *Cluster) { // Ensure valid case doesn't fail due to new checks
			c.Spec.Hosts = append(c.Spec.Hosts, HostSpec{Name: "worker1", Address: "1.1.1.2", Port: 22, User: "testuser"})
			c.Spec.RoleGroups = &RoleGroupsSpec{Worker: WorkerRoleSpec{Hosts: []string{"worker1", "m1"}}}
		}, ""}, // Empty wantErr means no error expected for this specific mutation case related to RoleGroups host validation.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newValidV1alpha1ClusterForTest()
			tt.mutator(cfg)
			// SetDefaults_Cluster is called in newValidV1alpha1ClusterForTest.
			// After mutation, we call Validate_Cluster directly.
			// If a default would fix the mutation, the test might pass unexpectedly if SetDefaults_Cluster is called again.
			// The goal here is to test if Validate_Cluster catches issues in a (potentially post-defaulted then mutated) state.
			err := Validate_Cluster(cfg)
			if err == nil {
				t.Fatalf("Validate_Cluster() expected error for %s, got nil", tt.name)
			}
			validationErrs, ok := err.(*ValidationErrors)
			if !ok {
				t.Fatalf("Validate_Cluster() error for %s is not *ValidationErrors type: %T", tt.name, err)
			}
			found := false
			for _, eStr := range validationErrs.Errors {
				if strings.Contains(eStr, tt.wantErr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Validate_Cluster() error for %s = %v, want to contain %q", tt.name, validationErrs, tt.wantErr)
			}
		})
	}
}

// Minimal KubernetesConfig for compiling TestValidate_NetworkConfig_Calls_Validate_CiliumConfig
// This can be removed if a shared test utility or simplified KubernetesConfig is available for tests.
/*
type KubernetesConfig struct {
	// Dummy fields if needed by other validation calls within NetworkConfig validation chain
}
*/

func TestValidate_Cluster_InvalidHostValues(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Port = 70000 // Invalid port
	SetDefaults_Cluster(cfg) // Port might get defaulted if it was 0. Here it's explicitly set.
	err := Validate_Cluster(cfg)
	if err == nil || !strings.Contains(err.Error(), "spec.hosts[0:m1].port: 70000 is invalid") {
		t.Errorf("Expected port validation error, got: %v", err)
	}

	cfg = newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Address = "not an ip or host!!"
	SetDefaults_Cluster(cfg)
	err = Validate_Cluster(cfg)
	if err == nil || !strings.Contains(err.Error(), "is not a valid IP address or hostname") {
		t.Errorf("Expected host address validation error for '!!', got: %v", err)
	}
}

func TestValidate_Cluster_DuplicateHostNames(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts = append(cfg.Spec.Hosts, HostSpec{Name: "m1", Address: "1.1.1.2", Port:22, User:"u"})
	SetDefaults_Cluster(cfg)
	err := Validate_Cluster(cfg)
	if err == nil || !strings.Contains(err.Error(), ".name: 'm1' is duplicated") {
		t.Errorf("Expected duplicate hostname error, got: %v", err)
	}
}

// Test for ValidationErrors methods (assuming it's still in cluster_types.go)
func TestValidationErrors_Methods_V1alpha1(t *testing.T) {
	ve := &ValidationErrors{} // Assuming ValidationErrors is defined in this package
	if ve.Error() != "no validation errors" {
		t.Errorf("Empty ValidationErrors.Error() incorrect: %s", ve.Error())
	}
	if !ve.IsEmpty() {t.Error("IsEmpty should be true for new ValidationErrors")}

	ve.Add("error 1: %s", "detail")
	if ve.Error() != "error 1: detail" { // Simplified Error() for single error
		t.Errorf("Single error string incorrect: %s", ve.Error())
	}
	if ve.IsEmpty() {t.Error("IsEmpty should be false after Add")}

	ve.Add("error 2")
	// The Error() method in cluster_types.go joins with "; ".
	// The old one from validate_test.go had a multi-line format. Adjusting expectation.
	expected := "error 1: detail; error 2"
	if ve.Error() != expected {
		t.Errorf("Multiple errors string incorrect. Got:\n'%s'\nWant:\n'%s'", ve.Error(), expected)
	}
}

func TestValidate_RoleGroupsSpec(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *RoleGroupsSpec
		wantErrMsg string
	}{
		{"nil_config", nil, ""}, // nil is acceptable
		{"empty_config", &RoleGroupsSpec{}, ""},
		{
			"master_host_empty",
			&RoleGroupsSpec{Master: MasterRoleSpec{Hosts: []string{"host1", ""}}},
			"master.hosts[1]: hostname cannot be empty",
		},
		{
			"worker_host_empty",
			&RoleGroupsSpec{Worker: WorkerRoleSpec{Hosts: []string{""}}},
			"worker.hosts[0]: hostname cannot be empty",
		},
		{
			"custom_role_empty_name",
			&RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "  ", Hosts: []string{"host1"}}}},
			"customRoles[0].name: custom role name cannot be empty",
		},
		{
			"custom_role_duplicate_name",
			&RoleGroupsSpec{CustomRoles: []CustomRoleSpec{
				{Name: "metrics", Hosts: []string{"host1"}},
				{Name: "metrics", Hosts: []string{"host2"}},
			}},
			"customRoles[1].name: custom role name 'metrics' is duplicated",
		},
		{
			"custom_role_host_empty",
			&RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "db", Hosts: []string{"host1", " "}}}},
			"customRoles[0].db.hosts[1]: hostname cannot be empty",
		},
		{
			"valid_custom_role",
			&RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "monitoring", Hosts: []string{"mon1", "mon2"}}}},
			"",
		},
		{
			"valid_multiple_roles",
			&RoleGroupsSpec{
				Master:      MasterRoleSpec{Hosts: []string{"master1"}},
				Worker:      WorkerRoleSpec{Hosts: []string{"worker1", "worker2"}},
				CustomRoles: []CustomRoleSpec{{Name: "gpu-nodes", Hosts: []string{"gpu1"}}},
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &ValidationErrors{}
			Validate_RoleGroupsSpec(tt.cfg, verrs, "spec.roleGroups")
			if tt.wantErrMsg == "" {
				if !verrs.IsEmpty() {
					t.Errorf("Validate_RoleGroupsSpec expected no error for %s, got %v", tt.name, verrs)
				}
			} else {
				if verrs.IsEmpty() {
					t.Fatalf("Validate_RoleGroupsSpec expected error for %s, got none", tt.name)
				}
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_RoleGroupsSpec error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
				}
			}
		})
	}
}
