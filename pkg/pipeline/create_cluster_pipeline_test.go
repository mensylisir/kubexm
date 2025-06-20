package pipeline

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
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
			// Example: Check the IsEnabled string condition for Containerd module
			// This expected string comes from the NewContainerdModuleSpec factory
			expectedContainerdIsEnabled := "(cfg.Spec.ContainerRuntime == nil) || (cfg.Spec.ContainerRuntime.Type == '') || (cfg.Spec.ContainerRuntime.Type == 'containerd')"
			if moduleSpec.IsEnabled != expectedContainerdIsEnabled {
				t.Errorf("Containerd module IsEnabled string mismatch. Got: '%s', Want: '%s'",
					moduleSpec.IsEnabled, expectedContainerdIsEnabled)
			}
			if len(moduleSpec.Tasks) == 0 {
				t.Errorf("Containerd module spec has no tasks assembled by its factory.")
			}
		}
		if moduleSpec.Name == "Etcd Cluster Management" {
			// Example: Check the IsEnabled string condition for Etcd module
			// This expected string comes from the NewEtcdModuleSpec factory
			expectedEtcdIsEnabled := "cfg.Spec.Etcd != nil"
			if moduleSpec.IsEnabled != expectedEtcdIsEnabled {
				t.Errorf("Etcd module IsEnabled string mismatch. Got: '%s', Want: '%s'",
					moduleSpec.IsEnabled, expectedEtcdIsEnabled)
			}
			if len(moduleSpec.Tasks) == 0 {
				t.Errorf("Etcd module spec has no tasks assembled by its factory (even if placeholders).")
			}
		}
	}

	// 3. Test Pipeline PreRun and PostRun hooks (now string identifiers)
	if pipelineSpec.PreRunHook != "" { // Expecting empty string as per factory
		t.Errorf("PipelineSpec.PreRunHook should be empty, got %q", pipelineSpec.PreRunHook)
	}
	if pipelineSpec.PostRunHook != "" { // Expecting empty string as per factory
		t.Errorf("PipelineSpec.PostRunHook should be empty, got %q", pipelineSpec.PostRunHook)
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
