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
			// Assuming arch and zone are typically global or derived elsewhere for tests
			Kubernetes: config.KubernetesSpec{Arch: "amd64"},
			ImageStore: config.ImageStoreSpec{Zone: "us-central1"},
			// ContainerRuntime.Version is the primary source for version
			ContainerRuntime: &config.ContainerRuntimeSpec{Version: "1.7.1"},
			Containerd: &config.ContainerdSpec{
				// Version: "1.7.0", // This would be overridden by ContainerRuntime.Version if logic existed, but factory takes version directly
				RegistryMirrors: map[string][]string{
					"docker.io": {"https://my.mirror.com", "https://another.mirror.com"},
				},
				UseSystemdCgroup:   true,
				InsecureRegistries: []string{"insecure.reg:5000"},
				ExtraTomlConfig:    "[plugins.\"io.containerd.toto.v1\"]\n  extra_config = true",
				ConfigPath:         "/custom/containerd.toml",
				// DownloadDir and Checksum are not in config, passed directly to factory.
			},
		},
	}

	// Parameters for NewInstallContainerdTaskSpec
	version := dummyCfg.Spec.ContainerRuntime.Version
	arch := dummyCfg.Spec.Kubernetes.Arch
	zone := dummyCfg.Spec.ImageStore.Zone
	downloadDir := "" // Use factory default
	checksum := ""    // No checksum in this test

	// Convert map[string][]string to map[string]string for the step, taking the first mirror.
	// This matches the assumption that ConfigureContainerdStepSpec might only handle one URL per registry.
	// If ConfigureContainerdStepSpec is updated to handle []string, this conversion is not needed.
	registryMirrorsForStep := make(map[string]string)
	if dummyCfg.Spec.Containerd != nil && dummyCfg.Spec.Containerd.RegistryMirrors != nil {
		for k, v := range dummyCfg.Spec.Containerd.RegistryMirrors {
			if len(v) > 0 {
				registryMirrorsForStep[k] = v[0]
			}
		}
	}

	insecureRegistries := dummyCfg.Spec.Containerd.InsecureRegistries
	useSystemdCgroup := dummyCfg.Spec.Containerd.UseSystemdCgroup
	extraTomlContent := dummyCfg.Spec.Containerd.ExtraTomlConfig
	containerdConfigPath := dummyCfg.Spec.Containerd.ConfigPath
	runOnRoles := []string{"all"} // Example role
	globalWorkDir := "/tmp/kubexm_test_workdir"

	taskSpec := taskContainerd.NewInstallContainerdTaskSpec(
		version, arch, zone, downloadDir, checksum,
		registryMirrorsForStep, insecureRegistries,
		useSystemdCgroup, extraTomlContent, containerdConfigPath,
		runOnRoles, globalWorkDir,
	)

	expectedName := "InstallAndConfigureContainerd"
	if taskSpec.Name != expectedName {
		t.Errorf("Name = '%s', want '%s'", taskSpec.Name, expectedName)
	}
	// Expect 5 steps: Download, Extract, Install, Configure, DaemonReload, Enable, Start
	if len(taskSpec.Steps) != 7 {
		t.Fatalf("Expected 7 steps, got %d", len(taskSpec.Steps))
	}

	var downloadStep *component_downloads.DownloadContainerdStepSpec
	var configureStep *stepSpecContainerd.ConfigureContainerdStepSpec
	// Other steps can be similarly checked if needed.

	for _, sSpec := range taskSpec.Steps {
		if s, ok := sSpec.(*component_downloads.DownloadContainerdStepSpec); ok {
			downloadStep = s
		}
		if s, ok := sSpec.(*stepSpecContainerd.ConfigureContainerdStepSpec); ok {
			configureStep = s
		}
	}

	if downloadStep == nil { t.Fatal("DownloadContainerdStepSpec not found") }
	if configureStep == nil { t.Fatal("ConfigureContainerdStepSpec not found") }

	if downloadStep.Version != version {
		t.Errorf("DownloadContainerdStepSpec.Version = %s, want %s", downloadStep.Version, version)
	}
	if !reflect.DeepEqual(configureStep.RegistryMirrors, registryMirrorsForStep) {
		t.Errorf("ConfigureContainerdStepSpec.RegistryMirrors = %v, want %v", configureStep.RegistryMirrors, registryMirrorsForStep)
	}
	if configureStep.UseSystemdCgroup != useSystemdCgroup {
		t.Errorf("ConfigureContainerdStepSpec.UseSystemdCgroup = %v, want %v", configureStep.UseSystemdCgroup, useSystemdCgroup)
	}
	if !reflect.DeepEqual(configureStep.InsecureRegistries, insecureRegistries) {
		t.Errorf("ConfigureContainerdStepSpec.InsecureRegistries = %v, want %v", configureStep.InsecureRegistries, insecureRegistries)
	}
	if configureStep.ExtraTomlContent != extraTomlContent {
		t.Errorf("ConfigureContainerdStepSpec.ExtraTomlContent mismatch")
	}
	if configureStep.ConfigFilePath != containerdConfigPath {
		t.Errorf("ConfigureContainerdStepSpec.ConfigFilePath = %s, want %s", configureStep.ConfigFilePath, containerdConfigPath)
	}
}

// Ensure step.GetSpecTypeName is available for tests that might need it for dynamic checks.
// This line just ensures the import is used to avoid compile errors if other tests were removed.
var _ = step.GetSpecTypeName
