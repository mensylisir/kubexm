package task

import (
	"strings" // For string checks in names/types
	"testing"
	// "time" // No longer needed for these factory tests
	// "context" // No longer needed
	// "errors" // No longer needed
	// "fmt" // No longer needed
	// "sync/atomic" // No longer needed

	"github.com/kubexms/kubexms/pkg/config" // For dummy config
	"github.com/kubexms/kubexms/pkg/spec"    // For spec.TaskSpec, spec.StepSpec
	"github.com/kubexms/kubexms/pkg/step"    // For step.GetSpecTypeName

	// Import task factories
	taskContainerd "github.com/kubexms/kubexms/pkg/task/containerd"
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"

	// Import StepSpec types to check them
	// It's good practice to alias if names might clash or for clarity
	stepSpecPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
	stepSpecContainerd "github.com/kubexms/kubexms/pkg/step/containerd"
	// commandStepSpec "github.com/kubexms/kubexms/pkg/step/command" // If any task used it directly
)

// Note: MockStep and newTestRuntimeForTask helpers are removed as they were for testing Task.Run,
// which no longer exists in pkg/task/task.go (it's now in pkg/executor).
// Tests now focus on factory output (*spec.TaskSpec).

func TestNewSystemChecksTask_Factory(t *testing.T) {
	dummyCfg := &config.Cluster{}
	taskSpec := taskPreflight.NewSystemChecksTask(dummyCfg)

	if taskSpec.Name != "Run System Preflight Checks" {
		t.Errorf("NewSystemChecksTask name = '%s', want 'Run System Preflight Checks'", taskSpec.Name)
	}
	if len(taskSpec.RunOnRoles) != 0 { // Expecting empty, meaning all hosts by default from orchestrator
		t.Errorf("NewSystemChecksTask RunOnRoles = %v, want empty []string{}", taskSpec.RunOnRoles)
	}
	if taskSpec.IgnoreError { // Preflight checks are typically critical
		t.Error("NewSystemChecksTask IgnoreError = true, want false")
	}

	// Check for expected steps and their types
	expectedStepSpecTypeNames := []string{ // Based on TypeName from step.GetSpecTypeName
		step.GetSpecTypeName(&stepSpecPreflight.CheckCPUStepSpec{}),
		step.GetSpecTypeName(&stepSpecPreflight.CheckMemoryStepSpec{}),
		step.GetSpecTypeName(&stepSpecPreflight.DisableSwapStepSpec{}),
	}
	if len(taskSpec.Steps) != len(expectedStepSpecTypeNames) {
		t.Fatalf("NewSystemChecksTask expected %d steps, got %d", len(expectedStepSpecTypeNames), len(taskSpec.Steps))
	}

	for i, sSpec := range taskSpec.Steps {
		typeName := step.GetSpecTypeName(sSpec)
		if typeName != expectedStepSpecTypeNames[i] {
			t.Errorf("Step %d type = %s, want %s", i, typeName, expectedStepSpecTypeNames[i])
		}
		// Optionally, type assert and check specific fields of the spec
		if cs, ok := sSpec.(*stepSpecPreflight.CheckCPUStepSpec); ok {
			if cs.MinCores != 2 { // Default from factory
				t.Errorf("CheckCPUStepSpec.MinCores = %d, want 2", cs.MinCores)
			}
		}
	}
}

func TestNewSetupKernelTask_Factory(t *testing.T) {
	dummyCfg := &config.Cluster{}
	taskSpec := taskPreflight.NewSetupKernelTask(dummyCfg)

	if taskSpec.Name != "Setup Kernel Parameters and Modules" {
		t.Errorf("Name = '%s', want 'Setup Kernel Parameters and Modules'", taskSpec.Name)
	}

	expectedStepSpecTypeNames := []string{
		step.GetSpecTypeName(&stepSpecPreflight.LoadKernelModulesStepSpec{}),
		step.GetSpecTypeName(&stepSpecPreflight.SetSystemConfigStepSpec{}),
	}
	if len(taskSpec.Steps) != len(expectedStepSpecTypeNames) {
		t.Fatalf("Expected %d steps, got %d", len(expectedStepSpecTypeNames), len(taskSpec.Steps))
	}
	for i, sSpec := range taskSpec.Steps {
		typeName := step.GetSpecTypeName(sSpec)
		if typeName != expectedStepSpecTypeNames[i] {
			t.Errorf("Step %d type = %s, want %s", i, typeName, expectedStepSpecTypeNames[i])
		}
		if ks, ok := sSpec.(*stepSpecPreflight.LoadKernelModulesStepSpec); ok {
			// Check default modules from factory
			expectedModules := []string{"br_netfilter", "overlay", "ip_vs"}
			if len(ks.Modules) != len(expectedModules) {
				t.Errorf("LoadKernelModulesStepSpec.Modules count = %d, want %d", len(ks.Modules), len(expectedModules))
			}
		}
		if ss, ok := sSpec.(*stepSpecPreflight.SetSystemConfigStepSpec); ok {
			if ss.Params["net.ipv4.ip_forward"] != "1" {
				t.Error("SetSystemConfigStepSpec.Params missing or incorrect net.ipv4.ip_forward")
			}
			if ss.Reload == nil || !*ss.Reload { // Reload defaults to true in factory (via *bool)
				t.Error("SetSystemConfigStepSpec.Reload is not true or nil (expecting explicit true from factory)")
			}
		}
	}
}


func TestNewInstallContainerdTask_Factory(t *testing.T) {
	// Example of providing some config to the factory
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			// This assumes Containerd field exists in ClusterSpec and ContainerdSpec in config package
			// For this test, we'll assume it's nil or default, and factory handles it.
			// If config.ContainerdSpec was defined and used:
			// Containerd: &config.ContainerdSpec{ Version: "1.7.1" },
		},
	}
	taskSpec := taskContainerd.NewInstallContainerdTask(dummyCfg)

	if taskSpec.Name != "Install and Configure Containerd" {
		t.Errorf("Name = '%s'", taskSpec.Name)
	}
	if len(taskSpec.Steps) != 3 { // Install, Configure, EnableAndStart
		t.Fatalf("Expected 3 steps, got %d", len(taskSpec.Steps))
	}

	expectedStepSpecTypeNames := []string{
		step.GetSpecTypeName(&stepSpecContainerd.InstallContainerdStepSpec{}),
		step.GetSpecTypeName(&stepSpecContainerd.ConfigureContainerdStepSpec{}),
		step.GetSpecTypeName(&stepSpecContainerd.EnableAndStartContainerdStepSpec{}),
	}
	for i, sSpec := range taskSpec.Steps {
		typeName := step.GetSpecTypeName(sSpec)
		if typeName != expectedStepSpecTypeNames[i] {
			t.Errorf("Step %d type = %s, want %s", i, typeName, expectedStepSpecTypeNames[i])
		}
	}

	// Example: Check if InstallContainerdStepSpec received a default version (empty if latest)
	if installSpec, ok := taskSpec.Steps[0].(*stepSpecContainerd.InstallContainerdStepSpec); ok {
		// The factory currently sets version to "" if not in config.
		// If a specific version was set in dummyCfg and read by factory, test that here.
		// Example: if dummyCfg.Spec.Containerd.Version was "1.7.1", check installSpec.Version == "1.7.1"
		if installSpec.Version != "" { // Default in factory is ""
			t.Logf("InstallContainerdStepSpec.Version = %s (expected empty for latest, or specific if from cfg)", installSpec.Version)
		}
	} else {
		t.Error("First step is not *stepSpecContainerd.InstallContainerdStepSpec")
	}

	if configSpec, ok := taskSpec.Steps[1].(*stepSpecContainerd.ConfigureContainerdStepSpec); ok {
		if !configSpec.UseSystemdCgroup { // Default in factory is true
			t.Error("ConfigureContainerdStepSpec.UseSystemdCgroup = false, want true")
		}
	} else {
		t.Error("Second step is not *stepSpecContainerd.ConfigureContainerdStepSpec")
	}
}
