package parser

import (
	"reflect"
	"testing"

	// Replace {{MODULE_NAME}} with the actual module name of your project
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
	"{{MODULE_NAME}}/pkg/config"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Not strictly needed for comparing parsed values unless we construct expected v1alpha1.ObjectMeta
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
    host: 192.168.1.10
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
		expectedCfg *config.Cluster // Only check a few key fields for non-error cases
	}{
		{
			name:        "valid comprehensive YAML",
			yamlData:    []byte(validYAMLComprehensive),
			expectError: false,
			expectedCfg: &config.Cluster{
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   config.Metadata{Name: "test-cluster-full"},
				Spec: config.ClusterSpec{
					Global: &v1alpha1.GlobalSpec{User: "ubuntu", Port: 2222, WorkDir: "/tmp/kubexms_test"},
					Hosts:  []v1alpha1.HostSpec{{Name: "master1", Address: "192.168.1.10", Roles: []string{"master", "etcd"}}}, // Just check first host for brevity
					RoleGroups: &v1alpha1.RoleGroupsSpec{
						Master: v1alpha1.MasterRoleSpec{Hosts: []string{"master1"}},
					},
					ControlPlaneEndpoint: &v1alpha1.ControlPlaneEndpointSpec{Host: "192.168.1.10", Port: 6443},
					System:               &v1alpha1.SystemSpec{PackageManager: "apt"},
					Kubernetes:           &v1alpha1.KubernetesConfig{Version: "1.25.3", ClusterName: "k8s-test-cluster"},
					Network:              &v1alpha1.NetworkConfig{Plugin: "calico", PodsCIDR: "10.244.0.0/16"},
					Etcd:                 &v1alpha1.EtcdConfig{Type: "internal"},
				},
			},
		},
		{
			name:        "valid minimal YAML",
			yamlData:    []byte(validYAMLMinimal),
			expectError: false,
			expectedCfg: &config.Cluster{
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   config.Metadata{Name: "test-cluster-min"},
				Spec: config.ClusterSpec{
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
			expectError: true, // yaml.Unmarshal returns error for empty input when unmarshaling into a struct
		},
		{
			name: "YAML with missing metadata name",
			yamlData: []byte(`
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata: {} # Name is missing
spec: {}
`),
			expectError: false, // Parser itself doesn't validate required fields, that's a higher level concern
			expectedCfg: &config.Cluster{ // Name will be empty
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   config.Metadata{Name: ""},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedCfg, err := ParseClusterYAML(tc.yamlData)

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

				// Perform selective field checks for brevity and focus
				if tc.expectedCfg != nil {
					if parsedCfg.APIVersion != tc.expectedCfg.APIVersion {
						t.Errorf("Expected APIVersion %q, got %q", tc.expectedCfg.APIVersion, parsedCfg.APIVersion)
					}
					if parsedCfg.Kind != tc.expectedCfg.Kind {
						t.Errorf("Expected Kind %q, got %q", tc.expectedCfg.Kind, parsedCfg.Kind)
					}
					if parsedCfg.Metadata.Name != tc.expectedCfg.Metadata.Name {
						t.Errorf("Expected Metadata.Name %q, got %q", tc.expectedCfg.Metadata.Name, parsedCfg.Metadata.Name)
					}
					if tc.expectedCfg.Spec.Global != nil && parsedCfg.Spec.Global != nil {
						if parsedCfg.Spec.Global.User != tc.expectedCfg.Spec.Global.User {
							t.Errorf("Expected Global.User %q, got %q", tc.expectedCfg.Spec.Global.User, parsedCfg.Spec.Global.User)
						}
					} else if tc.expectedCfg.Spec.Global != parsedCfg.Spec.Global { // one is nil, other not
						t.Errorf("Expected Global spec to be %+v, got %+v", tc.expectedCfg.Spec.Global, parsedCfg.Spec.Global)
					}

					if len(tc.expectedCfg.Spec.Hosts) > 0 && len(parsedCfg.Spec.Hosts) > 0 {
						if !reflect.DeepEqual(parsedCfg.Spec.Hosts[0], tc.expectedCfg.Spec.Hosts[0]) {
							t.Errorf("Expected first Host spec to be %+v, got %+v", tc.expectedCfg.Spec.Hosts[0], parsedCfg.Spec.Hosts[0])
						}
					} else if len(tc.expectedCfg.Spec.Hosts) != len(parsedCfg.Spec.Hosts) {
						t.Errorf("Expected %d hosts, got %d", len(tc.expectedCfg.Spec.Hosts), len(parsedCfg.Spec.Hosts))
					}

					// Check Kubernetes version if expected
					if tc.expectedCfg.Spec.Kubernetes != nil && parsedCfg.Spec.Kubernetes != nil {
						if parsedCfg.Spec.Kubernetes.Version != tc.expectedCfg.Spec.Kubernetes.Version {
							t.Errorf("Expected Kubernetes.Version %q, got %q", tc.expectedCfg.Spec.Kubernetes.Version, parsedCfg.Spec.Kubernetes.Version)
						}
					} else if tc.expectedCfg.Spec.Kubernetes != parsedCfg.Spec.Kubernetes {
						t.Errorf("Expected Kubernetes spec to be %+v, got %+v", tc.expectedCfg.Spec.Kubernetes, parsedCfg.Spec.Kubernetes)
					}

					// Check RoleGroups (basic check on master)
					if tc.expectedCfg.Spec.RoleGroups != nil && parsedCfg.Spec.RoleGroups != nil {
						if !reflect.DeepEqual(parsedCfg.Spec.RoleGroups.Master, tc.expectedCfg.Spec.RoleGroups.Master) {
							t.Errorf("Expected RoleGroups.Master to be %+v, got %+v", tc.expectedCfg.Spec.RoleGroups.Master, parsedCfg.Spec.RoleGroups.Master)
						}
					} else if tc.expectedCfg.Spec.RoleGroups != parsedCfg.Spec.RoleGroups {
						t.Errorf("Expected RoleGroups to be %+v, got %+v", tc.expectedCfg.Spec.RoleGroups, parsedCfg.Spec.RoleGroups)
					}
					// Add more checks as needed for other fields...
				}
			}
		})
	}
}
