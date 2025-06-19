package config

import (
	"reflect"
	"testing"
	"time"

	// Replace {{MODULE_NAME}} with the actual module name of your project
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToV1Alpha1Cluster(t *testing.T) {
	testCases := []struct {
		name        string
		inputCfg    *Cluster // config.Cluster
		expectedV1  *v1alpha1.Cluster
		expectError bool
	}{
		{
			name: "nil input config",
			inputCfg:    nil,
			expectError: true,
		},
		{
			name: "basic conversion with minimal spec",
			inputCfg: &Cluster{
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   Metadata{Name: "test-min"},
				Spec: ClusterSpec{ // This ClusterSpec now uses v1alpha1 types directly
					Hosts: []v1alpha1.HostSpec{
						{Name: "node1", Address: "10.0.0.1"},
					},
				},
			},
			expectedV1: &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-min"},
				Spec: v1alpha1.ClusterSpec{
					Hosts: []v1alpha1.HostSpec{
						{Name: "node1", Address: "10.0.0.1"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "conversion with comprehensive spec",
			inputCfg: &Cluster{
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   Metadata{Name: "test-full"},
				Spec: ClusterSpec{
					Global: &v1alpha1.GlobalSpec{User: "testuser", Port: 2202, ConnectionTimeout: 30 * time.Second},
					Hosts: []v1alpha1.HostSpec{
						{Name: "master-0", Address: "192.168.0.10", Roles: []string{"master"}},
						{Name: "worker-0", Address: "192.168.0.20", Roles: []string{"worker"}},
					},
					RoleGroups: &v1alpha1.RoleGroupsSpec{
						Master: v1alpha1.MasterRoleSpec{Hosts: []string{"master-0"}},
					},
					Kubernetes: &v1alpha1.KubernetesConfig{Version: "1.26.0"},
					Network:    &v1alpha1.NetworkConfig{Plugin: "cilium"},
					System:     &v1alpha1.SystemSpec{PackageManager: "yum"},
					ControlPlaneEndpoint: &v1alpha1.ControlPlaneEndpointSpec{Host: "vip.example.com", Port: 6443},
				},
			},
			expectedV1: &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-full"},
				Spec: v1alpha1.ClusterSpec{
					Global: &v1alpha1.GlobalSpec{User: "testuser", Port: 2202, ConnectionTimeout: 30 * time.Second},
					Hosts: []v1alpha1.HostSpec{
						{Name: "master-0", Address: "192.168.0.10", Roles: []string{"master"}},
						{Name: "worker-0", Address: "192.168.0.20", Roles: []string{"worker"}},
					},
					RoleGroups: &v1alpha1.RoleGroupsSpec{
						Master: v1alpha1.MasterRoleSpec{Hosts: []string{"master-0"}},
					},
					Kubernetes: &v1alpha1.KubernetesConfig{Version: "1.26.0"},
					Network:    &v1alpha1.NetworkConfig{Plugin: "cilium"},
					System:     &v1alpha1.SystemSpec{PackageManager: "yum"},
					ControlPlaneEndpoint: &v1alpha1.ControlPlaneEndpointSpec{Host: "vip.example.com", Port: 6443},
				},
			},
			expectError: false,
		},
		{
			name: "conversion with some spec fields nil",
			inputCfg: &Cluster{
				APIVersion: "kubexms.io/v1alpha1",
				Kind:       "Cluster",
				Metadata:   Metadata{Name: "test-partial"},
				Spec: ClusterSpec{
					Hosts: []v1alpha1.HostSpec{
						{Name: "node1", Address: "10.0.0.1"},
					},
					Kubernetes: nil, // Kubernetes config is nil
					Network:    &v1alpha1.NetworkConfig{Plugin: "flannel"},
				},
			},
			expectedV1: &v1alpha1.Cluster{
				TypeMeta: metav1.TypeMeta{APIVersion: "kubexms.io/v1alpha1", Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-partial"},
				Spec: v1alpha1.ClusterSpec{
					Hosts: []v1alpha1.HostSpec{
						{Name: "node1", Address: "10.0.0.1"},
					},
					Kubernetes: nil, // Expect Kubernetes to be nil in v1alpha1.ClusterSpec
					Network:    &v1alpha1.NetworkConfig{Plugin: "flannel"},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v1Cluster, err := ToV1Alpha1Cluster(tc.inputCfg)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				return // Don't proceed further if error was expected
			}

			if err != nil {
				t.Fatalf("Expected no error, but got: %v", err)
			}

			if v1Cluster == nil {
				t.Fatalf("Expected a non-nil v1alpha1.Cluster, but got nil")
			}

			// Compare TypeMeta
			if !reflect.DeepEqual(v1Cluster.TypeMeta, tc.expectedV1.TypeMeta) {
				t.Errorf("TypeMeta mismatch: got %+v, want %+v", v1Cluster.TypeMeta, tc.expectedV1.TypeMeta)
			}

			// Compare ObjectMeta
			if !reflect.DeepEqual(v1Cluster.ObjectMeta, tc.expectedV1.ObjectMeta) {
				t.Errorf("ObjectMeta mismatch: got %+v, want %+v", v1Cluster.ObjectMeta, tc.expectedV1.ObjectMeta)
			}

			// Compare Spec (DeepEqual should work well here due to direct type usage)
			if !reflect.DeepEqual(v1Cluster.Spec, tc.expectedV1.Spec) {
				t.Errorf("Spec mismatch: got %+v, want %+v", v1Cluster.Spec, tc.expectedV1.Spec)
			}
		})
	}
}
