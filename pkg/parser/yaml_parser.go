package parser

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// ParseFromFile is a stub implementation.
// TODO: Implement actual YAML parsing from file.
func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	fmt.Printf("INFO: [STUB] pkg/parser.ParseFromFile called for path: %s\n", filePath)
	// Return a minimal valid Cluster object for testing purposes
	return &v1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.Group + "/" + v1alpha1.SchemeGroupVersion.Version,
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "stub-cluster",
		},
		Spec: v1alpha1.ClusterSpec{
			Global: &v1alpha1.GlobalSpec{
				User:              "stubuser",
				Port:              22,
				ConnectionTimeout: 30 * time.Second,
				WorkDir:           "/tmp/kubexm_work_stub",
			},
			Hosts: []v1alpha1.HostSpec{
				{
					Name:    "stub-host1",
					Address: "127.0.0.1", // Make it a local connection for easier stub testing
					User:    "stubuser",
					Port:    22,
					Roles:   []string{"control-plane", "etcd", "worker", "web-server"}, // Add web-server for InstallNginxTask
					Type:    "local", // Specify as local to use LocalConnector
				},
			},
		},
	}, nil
}
