package parser

import (
	"reflect"
	"testing"

	// Replace {{MODULE_NAME}} with the actual module name of your project
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
	// "{{MODULE_NAME}}/pkg/config" // No longer needed as we use v1alpha1.Cluster directly
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseClusterYAML(t *testing.T) {
	// Valid comprehensive YAML
	validYAMLComprehensive := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-full
spec:
  global:
    user: ubuntu
    port: 2222
    workDir: /tmp/kubexms_test
  hosts:
    - name: master1
      address: 192.168.1.10
      roles: ["master", "etcd"]
    - name: worker1
      address: 192.168.1.11
      roles: ["worker"]
  roleGroups:
    master:
      hosts: ["master1"]
    worker:
      hosts: ["worker1"]
  controlPlaneEndpoint:
    address: 192.168.1.10 # Changed from host to address to match yaml tag
    port: 6443
  system:
    packageManager: apt
  kubernetes:
    version: "1.25.3"
    clusterName: "k8s-test-cluster"
  network:
    plugin: "calico"
    podsCIDR: "10.244.0.0/16"
  etcd:
    type: "internal" # or "external"
`

	// Minimal valid YAML
	validYAMLMinimal := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-min
spec:
  hosts:
    - name: node1
      address: 10.0.0.1
`
	// YAML with syntax error
	invalidYAMLSyntax := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: syntax-error
spec:
  hosts:
    - name: node1
      address: 10.0.0.1
  kubernetes:
    version: "1.23.0 # this comment is fine
    clusterName: "forgot-quote
`

	testCases := []struct {
		name        string
		yamlData    []byte
		expectError bool
		expectedCfg *v1alpha1.Cluster // Changed to v1alpha1.Cluster
	}{
		{
			name:        "valid comprehensive YAML",
			yamlData:    []byte(validYAMLComprehensive),
			expectError: false,
			expectedCfg: &v1alpha1.Cluster{ // Changed to v1alpha1.Cluster
				TypeMeta: metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-full"},
				Spec: v1alpha1.ClusterSpec{ // Changed to v1alpha1.ClusterSpec
					Global: &v1alpha1.GlobalSpec{User: "ubuntu", Port: 2222, WorkDir: "/tmp/kubexms_test"},
					Hosts:  []v1alpha1.HostSpec{{Name: "master1", Address: "192.168.1.10", Roles: []string{"master", "etcd"}}},
					RoleGroups: &v1alpha1.RoleGroupsSpec{
						Master: v1alpha1.MasterRoleSpec{Hosts: []string{"master1"}},
					},
					ControlPlaneEndpoint: &v1alpha1.ControlPlaneEndpointSpec{Host: "192.168.1.10", Port: 6443}, // Struct field is Host, YAML field 'address' maps to it
					System:               &v1alpha1.SystemSpec{PackageManager: "apt"},
					Kubernetes:           &v1alpha1.KubernetesConfig{Version: "1.25.3", ClusterName: "k8s-test-cluster"},
					Network:              &v1alpha1.NetworkConfig{Plugin: "calico", KubePodsCIDR: "10.244.0.0/16"}, // field name in v1alpha1 is KubePodsCIDR
					Etcd:                 &v1alpha1.EtcdConfig{Type: "internal"},
				},
			},
		},
		{
			name:        "valid minimal YAML",
			yamlData:    []byte(validYAMLMinimal),
			expectError: false,
			expectedCfg: &v1alpha1.Cluster{ // Changed
				TypeMeta:   metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-min"},
				Spec: v1alpha1.ClusterSpec{ // Changed
					Hosts: []v1alpha1.HostSpec{{Name: "node1", Address: "10.0.0.1"}},
				},
			},
		},
		{
			name:        "invalid YAML syntax",
			yamlData:    []byte(invalidYAMLSyntax),
			expectError: true,
		},
		{
			name:        "empty YAML data",
			yamlData:    []byte(""),
			expectError: true,
		},
		{
			name: "YAML with missing metadata name",
			yamlData: []byte(`
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata: {} # Name is missing
spec: {}
`),
			expectError: false,
			expectedCfg: &v1alpha1.Cluster{ // Changed
				TypeMeta:   metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: ""}, // Name will be empty
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedCfg, err := ParseClusterYAML(tc.yamlData) // Now returns *v1alpha1.Cluster

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if parsedCfg == nil {
					t.Fatalf("Expected a non-nil config, but got nil")
				}

				if tc.expectedCfg != nil {
					if parsedCfg.APIVersion != tc.expectedCfg.APIVersion {
						t.Errorf("Expected APIVersion %q, got %q", tc.expectedCfg.APIVersion, parsedCfg.APIVersion)
					}
					if parsedCfg.Kind != tc.expectedCfg.Kind {
						t.Errorf("Expected Kind %q, got %q", tc.expectedCfg.Kind, parsedCfg.Kind)
					}
					// Compare ObjectMeta.Name
					if parsedCfg.ObjectMeta.Name != tc.expectedCfg.ObjectMeta.Name {
						t.Errorf("Expected ObjectMeta.Name %q, got %q", tc.expectedCfg.ObjectMeta.Name, parsedCfg.ObjectMeta.Name)
					}

					// Selective deep comparisons for spec parts
					if !reflect.DeepEqual(parsedCfg.Spec.Global, tc.expectedCfg.Spec.Global) {
						t.Errorf("Spec.Global mismatch: got %+v, want %+v", parsedCfg.Spec.Global, tc.expectedCfg.Spec.Global)
					}

					if len(tc.expectedCfg.Spec.Hosts) > 0 { // Only check if expected hosts are defined
						if len(parsedCfg.Spec.Hosts) < 1 {
							t.Errorf("Expected at least one host, got %d", len(parsedCfg.Spec.Hosts))
						} else if !reflect.DeepEqual(parsedCfg.Spec.Hosts[0], tc.expectedCfg.Spec.Hosts[0]) {
							t.Errorf("Expected first Host spec to be %+v, got %+v", tc.expectedCfg.Spec.Hosts[0], parsedCfg.Spec.Hosts[0])
						}
					} else if len(parsedCfg.Spec.Hosts) != 0 {
						t.Errorf("Expected no hosts, got %d", len(parsedCfg.Spec.Hosts))
					}

					if !reflect.DeepEqual(parsedCfg.Spec.Kubernetes, tc.expectedCfg.Spec.Kubernetes) {
						t.Errorf("Spec.Kubernetes mismatch: got %+v, want %+v", parsedCfg.Spec.Kubernetes, tc.expectedCfg.Spec.Kubernetes)
					}
					if !reflect.DeepEqual(parsedCfg.Spec.RoleGroups, tc.expectedCfg.Spec.RoleGroups) {
						t.Errorf("Spec.RoleGroups mismatch: got %+v, want %+v", parsedCfg.Spec.RoleGroups, tc.expectedCfg.Spec.RoleGroups)
					}
					// Note: For ControlPlaneEndpoint, v1alpha1.ControlPlaneEndpointSpec.Host is used for yaml:"address"
					// The expectedCfg for comprehensive YAML uses Host field, ensure it aligns with the actual struct field name.
					// The YAML for controlPlaneEndpoint has "host:", which maps to ControlPlaneEndpointSpec.Host (json:"host", yaml:"address")
					// This means the test YAML for controlPlaneEndpoint should use "address:" or the tag on ControlPlaneEndpointSpec.Host should be yaml:"host"
					// Check for ControlPlaneEndpoint after YAML and expectedCfg are aligned
					if tc.name == "valid comprehensive YAML" {
						if parsedCfg.Spec.ControlPlaneEndpoint == nil {
							t.Errorf("Expected ControlPlaneEndpoint to be non-nil")
						} else {
							if parsedCfg.Spec.ControlPlaneEndpoint.Host != tc.expectedCfg.Spec.ControlPlaneEndpoint.Host {
								t.Errorf("Expected ControlPlaneEndpoint.Host %q, got %q", tc.expectedCfg.Spec.ControlPlaneEndpoint.Host, parsedCfg.Spec.ControlPlaneEndpoint.Host)
							}
							if parsedCfg.Spec.ControlPlaneEndpoint.Port != tc.expectedCfg.Spec.ControlPlaneEndpoint.Port {
								t.Errorf("Expected ControlPlaneEndpoint.Port %d, got %d", tc.expectedCfg.Spec.ControlPlaneEndpoint.Port, parsedCfg.Spec.ControlPlaneEndpoint.Port)
							}
						}
					} else if tc.expectedCfg.Spec.ControlPlaneEndpoint != nil { // For other tests, if CPE is expected, do a general check
						if !reflect.DeepEqual(parsedCfg.Spec.ControlPlaneEndpoint, tc.expectedCfg.Spec.ControlPlaneEndpoint) {
							t.Errorf("Spec.ControlPlaneEndpoint mismatch: got %+v, want %+v", parsedCfg.Spec.ControlPlaneEndpoint, tc.expectedCfg.Spec.ControlPlaneEndpoint)
						}
					}


					if !reflect.DeepEqual(parsedCfg.Spec.Network, tc.expectedCfg.Spec.Network) {
						t.Errorf("Spec.Network mismatch: got %+v, want %+v", parsedCfg.Spec.Network, tc.expectedCfg.Spec.Network)
					}
					if !reflect.DeepEqual(parsedCfg.Spec.Etcd, tc.expectedCfg.Spec.Etcd) {
						t.Errorf("Spec.Etcd mismatch: got %+v, want %+v", parsedCfg.Spec.Etcd, tc.expectedCfg.Spec.Etcd)
					}
				}
			}
		})
	}
}
