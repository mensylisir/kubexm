package pipeline

import (
	"testing"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	// No direct need to import specific module or task factories here,
	// as we are testing the pipeline factory's assembly of module *specs*.
	// We will check module spec names.
	// Module factories are called by NewCreateClusterPipelineSpec internally.
)

func TestNewCreateClusterPipelineSpec_Assembly(t *testing.T) {
	// Create a dummy config. The factory might use it to enable/disable modules
	// or pass it down to module/task factories which might use it.
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			// Initialize sub-specs to avoid nil pointer issues if factories access them,
			// even if they don't use specific fields from them in this test.
			Global:           config.GlobalSpec{},
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "containerd"}, // For ContainerdModule IsEnabled
			Etcd:             &config.EtcdSpec{Managed: true},                   // For EtcdModule IsEnabled
			// Kubernetes: &config.KubernetesSpec{}, // Example
			// Network:    &config.NetworkSpec{},    // Example
		},
	}

	pipelineSpec := NewCreateClusterPipelineSpec(dummyCfg)

	// 1. Test Pipeline Name
	expectedPipelineName := "Create New Kubernetes Cluster"
	if pipelineSpec.Name != expectedPipelineName {
		t.Errorf("PipelineSpec.Name = %q, want %q", pipelineSpec.Name, expectedPipelineName)
	}

	// 2. Test Assembled Modules (by name and order)
	// Based on the NewCreateClusterPipelineSpec implementation (Preflight, Containerd, Etcd currently)
	expectedModuleSpecNames := []string{
		"Preflight Checks and Setup",
		"Containerd Runtime",
		"Etcd Cluster Management",
		// Add names of other modules as they are uncommented/added in the factory
	}

	if len(pipelineSpec.Modules) != len(expectedModuleSpecNames) {
		t.Fatalf("Expected %d modules in the pipeline, got %d. Modules: %v",
			len(expectedModuleSpecNames), len(pipelineSpec.Modules), moduleSpecNames(pipelineSpec.Modules))
	}

	for i, moduleSpec := range pipelineSpec.Modules {
		if moduleSpec == nil {
			t.Errorf("Module %d is nil", i)
			continue
		}
		if moduleSpec.Name != expectedModuleSpecNames[i] {
			t.Errorf("Module %d: Name = %q, want %q", i, moduleSpec.Name, expectedModuleSpecNames[i])
		}

		// Optional: perform basic checks on the module spec itself
		if moduleSpec.Name == "Containerd Runtime" {
			if moduleSpec.IsEnabled == nil {
				t.Errorf("Containerd module IsEnabled function is nil")
			} else {
				// Test IsEnabled with the dummyCfg to ensure it behaves as expected by factory's logic
				if !moduleSpec.IsEnabled(dummyCfg) {
					t.Errorf("Containerd module IsEnabled returned false for config type '%s', factory should enable it.",
						dummyCfg.Spec.ContainerRuntime.Type)
				}
			}
			if len(moduleSpec.Tasks) == 0 {
				t.Errorf("Containerd module spec has no tasks assembled by its factory.")
			}
		}
		if moduleSpec.Name == "Etcd Cluster Management" {
			if moduleSpec.IsEnabled == nil {
				t.Errorf("Etcd module IsEnabled function is nil")
			} else {
				if !moduleSpec.IsEnabled(dummyCfg) {
					t.Errorf("Etcd module IsEnabled returned false, but factory default is true or based on Etcd.Managed=true")
				}
			}
			if len(moduleSpec.Tasks) == 0 {
				t.Errorf("Etcd module spec has no tasks assembled by its factory (even if placeholders).")
			}
		}
	}

	// 3. Test Pipeline PreRun and PostRun hooks
	if pipelineSpec.PreRun != nil {
		t.Errorf("PipelineSpec.PreRun is not nil, got %T (%s)", pipelineSpec.PreRun, pipelineSpec.PreRun.GetName())
	}
	if pipelineSpec.PostRun != nil {
		t.Errorf("PipelineSpec.PostRun is not nil, got %T (%s)", pipelineSpec.PostRun, pipelineSpec.PostRun.GetName())
	}

	// TODO: Add more test cases for NewCreateClusterPipelineSpec:
	// - Test with different configurations in dummyCfg that might alter which modules are
	//   enabled or how they are configured (if factories use cfg for more than just IsEnabled).
	// - Test if pipeline PreRun/PostRun steps are correctly assembled if the factory adds them.
}

// Helper to get module spec names for logging/debugging
func moduleSpecNames(modules []*spec.ModuleSpec) []string {
	names := make([]string, len(modules))
	for i, m := range modules {
		if m != nil {
			names[i] = m.Name
		} else {
			names[i] = "<nil>"
		}
	}
	return names
}
