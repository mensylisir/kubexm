package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/util/validation" // Import new validation package
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

func TestSetDefaults_SystemSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    *SystemSpec
		expected *SystemSpec
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty struct",
			input: &SystemSpec{},
			expected: &SystemSpec{
				NTPServers:         []string{},
				RPMs:               []string{},
				Debs:               []string{},
				PreInstallScripts:  []string{},
				PostInstallScripts: []string{},
				Modules:            []string{},
				SysctlParams:       make(map[string]string),
				// Timezone and PackageManager have no specific defaults other than their zero values
				// SkipConfigureOS defaults to false (zero value)
			},
		},
		{
			name: "partial fields set",
			input: &SystemSpec{
				NTPServers: []string{"ntp.example.com"},
				Timezone:   "Asia/Shanghai",
				SysctlParams: map[string]string{
					"net.ipv4.ip_forward": "1",
				},
			},
			expected: &SystemSpec{
				NTPServers:         []string{"ntp.example.com"},
				Timezone:           "Asia/Shanghai",
				RPMs:               []string{},
				Debs:               []string{},
				PreInstallScripts:  []string{},
				PostInstallScripts: []string{},
				Modules:            []string{},
				SysctlParams: map[string]string{
					"net.ipv4.ip_forward": "1",
				},
			},
		},
		{
			name: "all slice/map fields initially nil",
			input: &SystemSpec{
				NTPServers:         nil,
				RPMs:               nil,
				Debs:               nil,
				PreInstallScripts:  nil,
				PostInstallScripts: nil,
				Modules:            nil,
				SysctlParams:       nil,
			},
			expected: &SystemSpec{
				NTPServers:         []string{},
				RPMs:               []string{},
				Debs:               []string{},
				PreInstallScripts:  []string{},
				PostInstallScripts: []string{},
				Modules:            []string{},
				SysctlParams:       make(map[string]string),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_SystemSpec(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_SystemSpec(t *testing.T) {
	validSysctl := map[string]string{"net.ipv4.ip_forward": "1"}
	tests := []struct {
		name        string
		input       *SystemSpec
		expectErr   bool
		errContains []string
	}{
		{
			name:      "valid empty spec (after defaults)",
			input:     &SystemSpec{}, // Defaults will make it valid
			expectErr: false,
		},
		{
			name: "valid full spec",
			input: &SystemSpec{
				NTPServers:         []string{"0.pool.ntp.org"},
				Timezone:           "UTC",
				RPMs:               []string{"my-rpm"},
				Debs:               []string{"my-deb"},
				PackageManager:     "yum",
				PreInstallScripts:  []string{"echo pre"},
				PostInstallScripts: []string{"echo post"},
				SkipConfigureOS:    true,
				Modules:            []string{"br_netfilter"},
				SysctlParams:       validSysctl,
			},
			expectErr: false,
		},
		{
			name:        "NTP server empty string",
			input:       &SystemSpec{NTPServers: []string{""}},
			expectErr:   true,
			errContains: []string{"spec.system.ntpServers[0]: NTP server address cannot be empty"},
		},
		{
			name:        "Timezone is only whitespace",
			input:       &SystemSpec{Timezone: "   "},
			expectErr:   true,
			errContains: []string{"spec.system.timezone: cannot be only whitespace if specified"},
		},
		{
			name:        "RPM empty string",
			input:       &SystemSpec{RPMs: []string{"pkg1", " "}},
			expectErr:   true,
			errContains: []string{"spec.system.rpms[1]: RPM package name cannot be empty"},
		},
		{
			name:        "DEB empty string",
			input:       &SystemSpec{Debs: []string{" "}},
			expectErr:   true,
			errContains: []string{"spec.system.debs[0]: DEB package name cannot be empty"},
		},
		{
			name:        "PackageManager is only whitespace",
			input:       &SystemSpec{PackageManager: "  "},
			expectErr:   true,
			errContains: []string{"spec.system.packageManager: cannot be only whitespace if specified"},
		},
		{
			name:        "PreInstallScript empty string",
			input:       &SystemSpec{PreInstallScripts: []string{" "}},
			expectErr:   true,
			errContains: []string{"spec.system.preInstallScripts[0]: script cannot be empty"},
		},
		{
			name:        "PostInstallScript empty string",
			input:       &SystemSpec{PostInstallScripts: []string{"echo ok", " "}},
			expectErr:   true,
			errContains: []string{"spec.system.postInstallScripts[1]: script cannot be empty"},
		},
		{
			name:        "Module empty string",
			input:       &SystemSpec{Modules: []string{" "}},
			expectErr:   true,
			errContains: []string{"spec.system.modules[0]: module name cannot be empty"},
		},
		{
			name:        "Sysctl key empty string",
			input:       &SystemSpec{SysctlParams: map[string]string{" ": "1"}},
			expectErr:   true,
			errContains: []string{"spec.system.sysctlParams: sysctl key cannot be empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != nil {
				SetDefaults_SystemSpec(tt.input)
			}
			verrs := &validation.ValidationErrors{}
			Validate_SystemSpec(tt.input, verrs, "spec.system")
			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected validation errors for %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					fullErrorMsg := verrs.Error()
					for _, partialMsg := range tt.errContains {
						assert.Contains(t, fullErrorMsg, partialMsg, "Error for %s did not contain expected substring '%s'", tt.name, partialMsg)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no validation errors for %s, but got: %v", tt.name, verrs.Error())
			}
		})
	}
}

func TestValidate_Cluster_HostLocalType(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Type = "local"
	cfg.Spec.Hosts[0].Password = ""
	cfg.Spec.Hosts[0].PrivateKeyPath = ""
	cfg.Spec.Hosts[0].PrivateKey = ""

	err := Validate_Cluster(cfg)
	if err != nil {
		t.Errorf("Validate_Cluster() for host type 'local' failed: %v", err)
	}

	cfgClean := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ssh-no-creds"},
		Spec: ClusterSpec{
			Global:     &GlobalSpec{User: "test", Port: 22, WorkDir: "/tmp"},
			Hosts:      []HostSpec{{Name: "ssh-host", Address: "1.2.3.4", Type: "ssh"}},
			Kubernetes: &KubernetesConfig{Version: "v1.25.0"},
			Network:    &NetworkConfig{KubePodsCIDR: "10.244.0.0/16"},
			Etcd:       &EtcdConfig{},
			ControlPlaneEndpoint: &ControlPlaneEndpointSpec{Address: "1.2.3.4"},
		},
	}
	SetDefaults_Cluster(cfgClean)

	err = Validate_Cluster(cfgClean)
	if err == nil {
		t.Error("Validate_Cluster() expected error for non-local host without SSH details, but got nil")
	} else {
		validationErrs, ok := err.(*validation.ValidationErrors)
		if !ok {
			t.Fatalf("Validate_Cluster() error is not *validation.ValidationErrors type: %T", err)
		}
		assert.Contains(t, validationErrs.Error(), "no SSH authentication method provided for non-local host", "Validate_Cluster() error for non-local host without SSH details")
	}
}

func TestSetDefaults_Cluster_HostInheritanceAndDefaults(t *testing.T) {
	cfg := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-host-defaults"},
		Spec: ClusterSpec{
			Global: &GlobalSpec{
				User:           "global_user",
				Port:           2222,
				PrivateKeyPath: "/global/.ssh/id_rsa",
				WorkDir:        "/global_work",
			},
			Hosts: []HostSpec{
				{Name: "host1"},
				{Name: "host2", User: "host2_user", Port: 23},
				{Name: "host3", PrivateKeyPath: "/host3/.ssh/id_rsa"},
			},
		},
	}
	SetDefaults_Cluster(cfg)

	host1 := cfg.Spec.Hosts[0]
	assert.Equal(t, "global_user", host1.User, "Host1.User mismatch")
	assert.Equal(t, 2222, host1.Port, "Host1.Port mismatch")
	assert.Equal(t, "/global/.ssh/id_rsa", host1.PrivateKeyPath, "Host1.PrivateKeyPath mismatch")
	assert.Equal(t, "ssh", host1.Type, "Host1.Type mismatch")
	assert.NotNil(t, host1.Labels, "Host1.Labels should be initialized")
	assert.NotNil(t, host1.Roles, "Host1.Roles should be initialized")
	assert.NotNil(t, host1.Taints, "Host1.Taints should be initialized")

	host2 := cfg.Spec.Hosts[1]
	assert.Equal(t, "host2_user", host2.User, "Host2.User mismatch")
	assert.Equal(t, 23, host2.Port, "Host2.Port mismatch")
	assert.Equal(t, "/global/.ssh/id_rsa", host2.PrivateKeyPath, "Host2.PrivateKeyPath mismatch")

	cfgNoGlobal := &Cluster{
	   ObjectMeta: metav1.ObjectMeta{Name: "no-global"},
	   Spec: ClusterSpec{
		   Global: nil,
		   Hosts: []HostSpec{{Name: "hostOnly"}},
	   },
	}
	SetDefaults_Cluster(cfgNoGlobal)
	hostOnly := cfgNoGlobal.Spec.Hosts[0]
	assert.Equal(t, 22, hostOnly.Port, "hostOnly.Port mismatch")
	assert.Equal(t, "ssh", hostOnly.Type, "hostOnly.Type mismatch")
}

func TestSetDefaults_Cluster_ComponentStructsInitialization(t *testing.T) {
	cfg := &Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-components"}}
	SetDefaults_Cluster(cfg)

	assert.NotNil(t, cfg.Spec.ContainerRuntime, "Spec.ContainerRuntime is nil")
	assert.NotNil(t, cfg.Spec.Etcd, "Spec.Etcd is nil")
	assert.NotNil(t, cfg.Spec.Kubernetes, "Spec.Kubernetes is nil")
	assert.NotNil(t, cfg.Spec.Network, "Spec.Network is nil")
	assert.NotNil(t, cfg.Spec.HighAvailability, "Spec.HighAvailability is nil")
	assert.NotNil(t, cfg.Spec.Preflight, "Spec.Preflight is nil")
	assert.NotNil(t, cfg.Spec.System, "Spec.System is nil")
	assert.NotNil(t, cfg.Spec.Addons, "Spec.Addons is nil")
	assert.NotNil(t, cfg.Spec.DNS.CoreDNS.UpstreamDNSServers, "Spec.DNS.CoreDNS.UpstreamDNSServers is nil")
	assert.NotNil(t, cfg.Spec.System.SysctlParams, "Spec.System.SysctlParams is nil")
}

func newValidV1alpha1ClusterForTest() *Cluster {
	cfg := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "valid-cluster"},
		Spec: ClusterSpec{
			Global: &GlobalSpec{User: "testuser", Port: 22, PrivateKeyPath: "/dev/null", WorkDir: "/tmp", ConnectionTimeout: 5 * time.Second},
			Hosts:  []HostSpec{{Name: "m1", Address: "1.1.1.1", Port: 22, User: "testuser", Roles: []string{"master"}}},
			Kubernetes: &KubernetesConfig{Version: "v1.25.0"},
			Network:    &NetworkConfig{KubePodsCIDR: "10.244.0.0/16"},
			Etcd:       &EtcdConfig{},
			ControlPlaneEndpoint: &ControlPlaneEndpointSpec{Address: "1.2.3.4"},
		},
	}
	SetDefaults_Cluster(cfg)
	return cfg
}

func TestValidate_Cluster_ValidMinimal(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	err := Validate_Cluster(cfg)
	assert.NoError(t, err, "Validate_Cluster() with a minimal valid config failed")
}

func TestValidate_Cluster_TypeMeta(t *testing.T) {
   cfg := newValidV1alpha1ClusterForTest()
   cfg.APIVersion = "wrong.group/v1beta1"
   cfg.Kind = "NotCluster"
   err := Validate_Cluster(cfg)
   if assert.Error(t, err, "Expected validation error for TypeMeta, got nil") {
	   verrs, ok := err.(*validation.ValidationErrors)
	   assert.True(t, ok, "Error is not of type *validation.ValidationErrors")
	   assert.Contains(t, verrs.Error(), "apiVersion: must be", "Missing APIVersion error")
	   assert.Contains(t, verrs.Error(), "kind: must be Cluster", "Missing Kind error")
   }
}

func TestValidate_Cluster_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutator func(c *Cluster)
		wantErr string
	}{
		{"missing metadata.name", func(c *Cluster) { c.ObjectMeta.Name = "" }, "metadata.name: cannot be empty"},
		{"missing hosts", func(c *Cluster) { c.Spec.Hosts = []HostSpec{} }, "spec.hosts: must contain at least one host"},
		{"missing host.name", func(c *Cluster) { c.Spec.Hosts[0].Name = "" }, "spec.hosts[0].name: cannot be empty"},
		{"missing host.address", func(c *Cluster) { c.Spec.Hosts[0].Address = "" }, "spec.hosts[0:m1].address: cannot be empty"},
		{"missing host.user (after global also empty)", func(c *Cluster) {
			c.Spec.Global.User = ""
			c.Spec.Hosts[0].User = ""
		}, "spec.hosts[0:m1].user: cannot be empty (after defaults)"},
		{"missing k8s section", func(c *Cluster) { c.Spec.Kubernetes = nil }, "spec.kubernetes: section is required"},
		{"missing etcd section", func(c *Cluster) { c.Spec.Etcd = nil }, "spec.etcd: section is required"},
		{"missing network section", func(c *Cluster) { c.Spec.Network = nil }, "spec.network: section is required"},
		{"invalid DNS config", func(c *Cluster) { c.Spec.DNS.CoreDNS.UpstreamDNSServers = []string{""} }, "spec.dns.coredns.upstreamDNSServers[0]: server address cannot be empty"},
		{"invalid System config", func(c *Cluster) { c.Spec.System = &SystemSpec{NTPServers: []string{""}} }, "spec.system.ntpServers[0]: NTP server address cannot be empty"},
		{"rolegroup master host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{Master: MasterRoleSpec{Hosts: []string{"unknown-host"}}}
		}, "spec.roleGroups.master.hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup worker host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{Worker: WorkerRoleSpec{Hosts: []string{"m1", "unknown-host"}}}
		}, "spec.roleGroups.worker.hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup custom host not in spec.hosts", func(c *Cluster) {
			c.Spec.RoleGroups = &RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "cg", Hosts: []string{"unknown-host"}}}}
		}, "spec.roleGroups.customRoles[0:cg].hosts: host 'unknown-host' is not defined in spec.hosts"},
		{"rolegroup valid hosts", func(c *Cluster) {
			c.Spec.Hosts = append(c.Spec.Hosts, HostSpec{Name: "worker1", Address: "1.1.1.2", Port: 22, User: "testuser", PrivateKeyPath: "/dev/null"}) // Added PrivateKeyPath
			c.Spec.RoleGroups = &RoleGroupsSpec{Worker: WorkerRoleSpec{Hosts: []string{"worker1", "m1"}}}
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newValidV1alpha1ClusterForTest()
			tt.mutator(cfg)

			err := Validate_Cluster(cfg)
			if tt.wantErr == "" {
				assert.NoError(t, err, "Validate_Cluster() for %s expected no error", tt.name)
			} else {
				if assert.Error(t, err, "Validate_Cluster() expected error for %s, got nil", tt.name) {
					validationErrs, ok := err.(*validation.ValidationErrors)
					if assert.True(t, ok, "Validate_Cluster() error for %s is not *validation.ValidationErrors type: %T", tt.name, err) {
						assert.Contains(t, validationErrs.Error(), tt.wantErr, "Validate_Cluster() error for %s", tt.name)
					}
				}
			}
		})
	}
}

func TestValidate_Cluster_InvalidHostValues(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Port = 70000
	SetDefaults_Cluster(cfg)
	err := Validate_Cluster(cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "spec.hosts[0:m1].port: 70000 is invalid", "Expected port validation error")
	}

	cfg = newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts[0].Address = "not an ip or host!!"
	SetDefaults_Cluster(cfg)
	err = Validate_Cluster(cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "is not a valid IP address or hostname", "Expected host address validation error")
	}
}

func TestValidate_Cluster_DuplicateHostNames(t *testing.T) {
	cfg := newValidV1alpha1ClusterForTest()
	cfg.Spec.Hosts = append(cfg.Spec.Hosts, HostSpec{Name: "m1", Address: "1.1.1.2", Port:22, User:"u"})
	SetDefaults_Cluster(cfg)
	err := Validate_Cluster(cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), ".name: 'm1' is duplicated", "Expected duplicate hostname error")
	}
}

func TestValidate_RoleGroupsSpec(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *RoleGroupsSpec
		wantErrMsg string // Expect this substring in the error message
	}{
		{"nil_config", nil, ""},
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
			"spec.roleGroups.customRoles[1:metrics].name: custom role name 'metrics' is duplicated",
		},
		{
			"custom_role_host_empty",
			&RoleGroupsSpec{CustomRoles: []CustomRoleSpec{{Name: "db", Hosts: []string{"host1", " "}}}},
			"spec.roleGroups.customRoles[0:db].hosts.hosts[1]: hostname cannot be empty",
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
			verrs := &validation.ValidationErrors{}
			Validate_RoleGroupsSpec(tt.cfg, verrs, "spec.roleGroups")
			if tt.wantErrMsg == "" {
				assert.False(t, verrs.HasErrors(), "Validate_RoleGroupsSpec for %s expected no error, got %v", tt.name, verrs.Error())
			} else {
				if assert.True(t, verrs.HasErrors(), "Validate_RoleGroupsSpec for %s expected error, got none", tt.name) {
					assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Validate_RoleGroupsSpec error for %s", tt.name)
				}
			}
		})
	}
}
