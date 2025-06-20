package task

import (
	"reflect" // For DeepEqual
	"strings" // For string checks in names/types
	"testing"
	// "time" // Not directly needed for factory output tests

	"github.com/mensylisir/kubexm/pkg/config" // For dummy config
	"github.com/mensylisir/kubexm/pkg/spec"    // For spec.TaskSpec, spec.StepSpec
	"github.com/mensylisir/kubexm/pkg/step"    // For step.GetSpecTypeName

	// Import task factories
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
	taskPreflight "github.com/mensylisir/kubexm/pkg/task/preflight"

	// Import StepSpec types to check them
	stepSpecContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
	stepSpecPreflight "github.com/mensylisir/kubexm/pkg/step/preflight"
	// commandStepSpec "github.com/kubexms/kubexms/pkg/step/command" // If any task used it directly
)

// Note: MockStep and newTestRuntimeForTask helpers are removed as they were for testing Task.Run,
// which no longer exists in pkg/task/task.go. Tests now focus on factory output.

func TestNewSystemChecksTask_Factory_WithConfig(t *testing.T) {
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			PreflightConfig: config.PreflightConfigSpec{
				MinCPUCores: 3,
				MinMemoryMB: 3072,
				DisableSwap: true, // Explicitly request swap disable
			},
		},
	}
	taskSpec := taskPreflight.NewSystemChecksTask(dummyCfg)

	if taskSpec.Name != "Run System Preflight Checks" {
		t.Errorf("Name = '%s'", taskSpec.Name)
	}

	foundCPUSpec := false
	foundMemSpec := false
	foundSwapSpec := false

	for _, sSpec := range taskSpec.Steps {
		switch s := sSpec.(type) {
		case *stepSpecPreflight.CheckCPUStepSpec:
			foundCPUSpec = true
			if s.MinCores != 3 {
				t.Errorf("CheckCPUStepSpec.MinCores = %d, want 3", s.MinCores)
			}
		case *stepSpecPreflight.CheckMemoryStepSpec:
			foundMemSpec = true
			if s.MinMemoryMB != 3072 {
				t.Errorf("CheckMemoryStepSpec.MinMemoryMB = %d, want 3072", s.MinMemoryMB)
			}
		case *stepSpecPreflight.DisableSwapStepSpec:
			foundSwapSpec = true
		}
	}
	if !foundCPUSpec { t.Error("CheckCPUStepSpec not found") }
	if !foundMemSpec { t.Error("CheckMemoryStepSpec not found") }
	if !foundSwapSpec { t.Error("DisableSwapStepSpec not found when PreflightConfig.DisableSwap is true") }
	if len(taskSpec.Steps) != 3 { // Expect all three with DisableSwap: true
		t.Errorf("Expected 3 steps when DisableSwap is true, got %d", len(taskSpec.Steps))
	}
}

func TestNewSystemChecksTask_Factory_SkipDisableSwap(t *testing.T) {
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			PreflightConfig: config.PreflightConfigSpec{
				DisableSwap: false, // Explicitly request NOT to disable swap
			},
		},
	}
	taskSpec := taskPreflight.NewSystemChecksTask(dummyCfg)

	disableSwapStepFound := false
	for _, sSpec := range taskSpec.Steps {
		if _, ok := sSpec.(*stepSpecPreflight.DisableSwapStepSpec); ok {
			disableSwapStepFound = true
			break
		}
	}
	if disableSwapStepFound {
		t.Error("DisableSwapStepSpec found when PreflightConfig.DisableSwap is false")
	}
	if len(taskSpec.Steps) != 2 { // Expect CPU and Mem checks only
		t.Errorf("Expected 2 steps when DisableSwap is false, got %d", len(taskSpec.Steps))
	}
}


func TestNewSetupKernelTask_Factory_WithConfig(t *testing.T) {
	customModules := []string{"custom_mod1", "custom_mod2"}
	customSysctl := map[string]string{"custom.param": "1", "another.param": "2"}
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			KernelConfig: config.KernelConfigSpec{
				Modules:      customModules,
				SysctlParams: customSysctl,
				// SysctlConfigFilePath: "/my/custom/sysctl.conf", // Example for path override
			},
		},
	}
	taskSpec := taskPreflight.NewSetupKernelTask(dummyCfg)

	if taskSpec.Name != "Setup Kernel Parameters and Modules" {
		t.Errorf("Name = '%s'", taskSpec.Name)
	}

	var loadModulesSpec *stepSpecPreflight.LoadKernelModulesStepSpec
	var setSysctlSpec *stepSpecPreflight.SetSystemConfigStepSpec

	for _, sSpec := range taskSpec.Steps {
		if s, ok := sSpec.(*stepSpecPreflight.LoadKernelModulesStepSpec); ok { loadModulesSpec = s }
		if s, ok := sSpec.(*stepSpecPreflight.SetSystemConfigStepSpec); ok { setSysctlSpec = s }
	}

	if loadModulesSpec == nil { t.Fatal("LoadKernelModulesStepSpec not found") }
	if !reflect.DeepEqual(loadModulesSpec.Modules, customModules) {
		t.Errorf("LoadKernelModulesStepSpec.Modules = %v, want %v", loadModulesSpec.Modules, customModules)
	}

	if setSysctlSpec == nil { t.Fatal("SetSystemConfigStepSpec not found") }
	if !reflect.DeepEqual(setSysctlSpec.Params, customSysctl) {
		t.Errorf("SetSystemConfigStepSpec.Params = %v, want %v", setSysctlSpec.Params, customSysctl)
	}
	// Example if checking path override
	// if setSysctlSpec.ConfigFilePath != "/my/custom/sysctl.conf" {
	// 	t.Errorf("SetSystemConfigStepSpec.ConfigFilePath = %s, want /my/custom/sysctl.conf", setSysctlSpec.ConfigFilePath)
	// }
	if setSysctlSpec.Reload == nil || !*setSysctlSpec.Reload { // Factory sets Reload to true by default
		t.Error("SetSystemConfigStepSpec.Reload should be true by default from factory")
	}
}

func TestNewInstallContainerdTask_Factory_WithConfig(t *testing.T) {
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Version: "1.7.1"},
			Containerd: &config.ContainerdSpec{
				// Version: "1.7.0", // This would be overridden by ContainerRuntime.Version
				RegistryMirrors: map[string][]string{ // Renamed from RegistryMirrorsConfig
					"docker.io": {"https://my.mirror.com", "https://another.mirror.com"},
				},
				UseSystemdCgroup:   true,
				InsecureRegistries: []string{"insecure.reg:5000"},
				ExtraTomlConfig:   "[plugins.\"io.containerd.toto.v1\"]\n  extra_config = true",
				ConfigPath: "/custom/containerd.toml",
			},
		},
	}
	taskSpec := taskContainerd.NewInstallContainerdTask(dummyCfg)

	if taskSpec.Name != "Install and Configure Containerd" {
		t.Errorf("Name = '%s'", taskSpec.Name)
	}
	if len(taskSpec.Steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(taskSpec.Steps))
	}

	var installSpec *stepSpecContainerd.InstallContainerdStepSpec
	var configSpec *stepSpecContainerd.ConfigureContainerdStepSpec
	// var manageSpec *stepSpecContainerd.EnableAndStartContainerdStepSpec // Not checked in detail here

	for _, sSpec := range taskSpec.Steps {
		if s, ok := sSpec.(*stepSpecContainerd.InstallContainerdStepSpec); ok { installSpec = s }
		if s, ok := sSpec.(*stepSpecContainerd.ConfigureContainerdStepSpec); ok { configSpec = s }
		// if s, ok := sSpec.(*stepSpecContainerd.EnableAndStartContainerdStepSpec); ok { manageSpec = s }
	}

	if installSpec == nil { t.Fatal("InstallContainerdStepSpec not found") }
	if configSpec == nil { t.Fatal("ConfigureContainerdStepSpec not found") }
	// if manageSpec == nil { t.Fatal("EnableAndStartContainerdStepSpec not found") }


	if installSpec.Version != "1.7.1" { // From ContainerRuntime.Version
		t.Errorf("InstallContainerdStepSpec.Version = %s, want '1.7.1'", installSpec.Version)
	}
	if configSpec.RegistryMirrors["docker.io"] != "https://my.mirror.com" {
		t.Errorf("ConfigureContainerdStepSpec.RegistryMirrors['docker.io'] = %s, want 'https://my.mirror.com'", configSpec.RegistryMirrors["docker.io"])
	}
	if !configSpec.UseSystemdCgroup {
		t.Error("ConfigureContainerdStepSpec.UseSystemdCgroup = false, want true")
	}
	if len(configSpec.InsecureRegistries) != 1 || configSpec.InsecureRegistries[0] != "insecure.reg:5000" {
		t.Errorf("ConfigureContainerdStepSpec.InsecureRegistries = %v, want ['insecure.reg:5000']", configSpec.InsecureRegistries)
	}
	if configSpec.ExtraTomlContent != "[plugins.\"io.containerd.toto.v1\"]\n  extra_config = true" {
		t.Errorf("ConfigureContainerdStepSpec.ExtraTomlContent mismatch")
	}
	if configSpec.ConfigFilePath != "/custom/containerd.toml" {
		t.Errorf("ConfigureContainerdStepSpec.ConfigFilePath = %s, want /custom/containerd.toml", configSpec.ConfigFilePath)
	}
}

// Ensure step.GetSpecTypeName is available for tests that might need it for dynamic checks.
// This line just ensures the import is used to avoid compile errors if other tests were removed.
var _ = step.GetSpecTypeName
