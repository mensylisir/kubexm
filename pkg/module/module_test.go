package module

import (
	"strings" // For string checks in names/types
	"testing"
	// "context" // No longer needed for these factory tests
	// "errors" // No longer needed
	// "fmt" // No longer needed
	// "sync/atomic" // No longer needed
	// "time" // No longer needed

	"github.com/kubexms/kubexms/pkg/config"    // For dummy config
	"github.com/kubexms/kubexms/pkg/spec"     // For spec.ModuleSpec, spec.TaskSpec
	"github.com/kubexms/kubexms/pkg/step"     // For step.GetSpecTypeName
	// "github.com/kubexms/kubexms/pkg/runtime" // Not directly needed for factory assembly tests

	// Import module factories
	moduleContainerd "github.com/kubexms/kubexms/pkg/module/containerd"
	modulePreflight "github.com/kubexms/kubexms/pkg/module/preflight"
	// moduleEtcd "github.com/kubexms/kubexms/pkg/module/etcd" // If testing etcd module factory

	// Import StepSpec types to check them if necessary
	// stepSpecPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
	// stepSpecContainerd "github.com/kubexms/kubexms/pkg/step/containerd"
)

// Note: MockTask and newTestRuntimeForModule helpers are removed as they were for testing Module.Run,
// which no longer exists in pkg/module/module.go (its logic is now in pkg/executor).
// Tests now focus on factory output (*spec.ModuleSpec).

func TestNewPreflightModule_Factory(t *testing.T) {
	dummyCfg := &config.Cluster{ Spec: config.ClusterSpec{ Global: config.GlobalSpec{} }} // Ensure Spec.Global exists if IsEnabled checks it
	moduleSpec := modulePreflight.NewPreflightModule(dummyCfg)

	if moduleSpec.Name != "Preflight Checks and Setup" {
		t.Errorf("NewPreflightModule name = '%s', want 'Preflight Checks and Setup'", moduleSpec.Name)
	}

	if moduleSpec.IsEnabled == nil {
		t.Fatal("NewPreflightModule IsEnabled function is nil")
	}
	if !moduleSpec.IsEnabled(dummyCfg) { // Preflight is usually always enabled by default in its factory
		t.Error("NewPreflightModule IsEnabled() returned false, want true")
	}

	expectedTaskNames := []string{
		"Run System Preflight Checks",
		"Setup Kernel Parameters and Modules",
	}
	if len(moduleSpec.Tasks) != len(expectedTaskNames) {
		t.Fatalf("NewPreflightModule expected %d tasks, got %d", len(expectedTaskNames), len(moduleSpec.Tasks))
	}
	for i, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil {
			t.Errorf("Task %d is nil", i); continue
		}
		if taskSpec.Name != expectedTaskNames[i] {
			t.Errorf("Task %d name = '%s', want '%s'", i, taskSpec.Name, expectedTaskNames[i])
		}
		// Example: Deeper check for a specific task's step count
		if taskSpec.Name == "Run System Preflight Checks" && len(taskSpec.Steps) < 3 {
			t.Errorf("Task 'Run System Preflight Checks' should have at least 3 steps, got %d", len(taskSpec.Steps))
		}
	}

	if moduleSpec.PreRun != nil {
		t.Errorf("NewPreflightModule PreRun spec is not nil, got %T", moduleSpec.PreRun)
	}
	if moduleSpec.PostRun != nil {
		t.Errorf("NewPreflightModule PostRun spec is not nil, got %T", moduleSpec.PostRun)
	}
}

func TestNewContainerdModule_Factory(t *testing.T) {
	// Test case 1: IsEnabled should be true with specific config for "containerd"
	cfgContainerdEnabled := &config.Cluster{
		Spec: config.ClusterSpec{
			// This assumes config.ContainerRuntimeSpec is defined and populated in pkg/config
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "containerd"},
		},
	}
	moduleSpecEnabled := moduleContainerd.NewContainerdModule(cfgContainerdEnabled)

	if moduleSpecEnabled.Name != "Containerd Runtime" {
		t.Errorf("Name = '%s'", moduleSpecEnabled.Name)
	}
	if moduleSpecEnabled.IsEnabled == nil {
		t.Fatal("IsEnabled function is nil")
	}
	if !moduleSpecEnabled.IsEnabled(cfgContainerdEnabled) {
		t.Error("IsEnabled() returned false with type 'containerd', want true")
	}
	if len(moduleSpecEnabled.Tasks) != 1 { // Expecting NewInstallContainerdTask
		t.Fatalf("Expected 1 task for containerd, got %d", len(moduleSpecEnabled.Tasks))
	}
	if moduleSpecEnabled.Tasks[0] == nil {
		t.Fatal("Task 0 in Containerd module is nil")
	}
	if moduleSpecEnabled.Tasks[0].Name != "Install and Configure Containerd" {
		t.Errorf("Task name = '%s', want 'Install and Configure Containerd'", moduleSpecEnabled.Tasks[0].Name)
	}

	// Test case 2: IsEnabled behavior with nil ContainerRuntime spec (factory defaults to true)
	cfgNilRuntime := &config.Cluster{ Spec: config.ClusterSpec{} }
	moduleSpecNil := moduleContainerd.NewContainerdModule(cfgNilRuntime)
	if !moduleSpecNil.IsEnabled(cfgNilRuntime) {
		t.Error("IsEnabled() returned false for nil ContainerRuntime spec, but factory defaults to true")
	}

	// Test case 3: IsEnabled should be false if runtime type is different
	cfgDockerRuntime := &config.Cluster{
	    Spec: config.ClusterSpec{
	        ContainerRuntime: &config.ContainerRuntimeSpec{Type: "docker"},
	    },
	}
	moduleSpecDocker := moduleContainerd.NewContainerdModule(cfgDockerRuntime)
	if moduleSpecDocker.IsEnabled(cfgDockerRuntime) {
	    t.Error("IsEnabled() returned true when runtime type is 'docker', want false")
	}

	if moduleSpecEnabled.PreRun != nil || moduleSpecEnabled.PostRun != nil {
		t.Error("Containerd module PreRun or PostRun spec is not nil")
	}
}

// Note: Tests for NewEtcdModule would follow a similar pattern.
// They would involve creating various dummy *config.Cluster instances
// to simulate different etcd configurations (e.g., number of nodes, external vs. managed)
// and asserting that the NewEtcdModule factory assembles the correct list of *spec.TaskSpec
// and sets other ModuleSpec fields appropriately.
// Since NewEtcdModule currently uses placeholder task specs, detailed tests for its
// task assembly would wait until those task factories are also refactored.
// A basic assembly test could check the module name and the names of the placeholder tasks.
