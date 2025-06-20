package module

import (
	"reflect" // For reflect.DeepEqual in helper, if used.
	"sort"    // For sorting string slices in helper, if used.
	"strings" // For string checks in names/types
	"testing"
	// "context" // No longer needed for these factory tests
	// "errors" // No longer needed
	// "fmt" // No longer needed
	// "sync/atomic" // No longer needed
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/config"    // For dummy config
	"github.com/mensylisir/kubexm/pkg/spec"     // For spec.ModuleSpec, spec.TaskSpec
	// "github.com/kubexms/kubexms/pkg/runtime" // Not directly needed for factory assembly tests
	// "github.com/kubexms/kubexms/pkg/step"      // For step.GetSpecTypeName if checking hook step types

	// Import module factories
	moduleContainerd "github.com/mensylisir/kubexm/pkg/module/containerd"
	modulePreflight "github.com/mensylisir/kubexm/pkg/module/preflight"
	moduleEtcd "github.com/mensylisir/kubexm/pkg/module/etcd"
)

// Note: MockTask and newTestRuntimeForModule helpers are removed as they were for testing Module.Run,
// which no longer exists in pkg/module/module.go. Tests now focus on factory output (*spec.ModuleSpec).

func TestNewPreflightModule_Factory_IsEnabledLogic(t *testing.T) {
	cfgSkipPreflight := &config.Cluster{
		Spec: config.ClusterSpec{
			Global: config.GlobalSpec{SkipPreflight: true},
		},
	}
	// Apply defaults to ensure full config structure is present if IsEnabled accesses deeper fields
	config.SetDefaults(cfgSkipPreflight)


	cfgDoPreflight := &config.Cluster{
		Spec: config.ClusterSpec{
			Global: config.GlobalSpec{SkipPreflight: false},
		},
	}
	config.SetDefaults(cfgDoPreflight)

	cfgDefaultPreflight := &config.Cluster{}
	config.SetDefaults(cfgDefaultPreflight) // SkipPreflight defaults to false in GlobalSpec

	modSpecSkip := modulePreflight.NewPreflightModuleSpec(cfgSkipPreflight)
	expectedSkipCondition := "(cfg.Spec.Global == nil) || (cfg.Spec.Global.SkipPreflight == false)" // This actually enables if SkipPreflight is true due to original module logic
	// The IsEnabled string for PreflightModuleSpec is "(cfg.Spec.Global == nil) || (cfg.Spec.Global.SkipPreflight == false)"
	// This means it's enabled if Global is nil OR SkipPreflight is false.
	// So if SkipPreflight is true (and Global is not nil), the condition is false.
	// If SkipPreflight is false (and Global is not nil), the condition is true.
	// If Global is nil, the condition is true.

	// Let's verify the generated IsEnabled string
	// The factory NewPreflightModuleSpec generates: "(cfg.Spec.Global == nil) || (cfg.Spec.Global.SkipPreflight == false)"
	// If cfg.Spec.Global.SkipPreflight is true, IsEnabled should effectively be false.
	// If cfg.Spec.Global.SkipPreflight is false, IsEnabled should effectively be true.
	expectedCondition := "(cfg.Spec.Global == nil) || (cfg.Spec.Global.SkipPreflight == false)"
	if modSpecSkip.IsEnabled != expectedCondition {
		t.Errorf("Preflight IsEnabled string mismatch. Got: '%s', Want: '%s'", modSpecSkip.IsEnabled, expectedCondition)
	}
	// Test evaluation of this condition would be Executor's role. Here we test string construction.
	// For cfgSkipPreflight (SkipPreflight = true), the string means it *should* be disabled.

	modSpecDo := modulePreflight.NewPreflightModuleSpec(cfgDoPreflight)
	if modSpecDo.IsEnabled != expectedCondition {
		t.Errorf("Preflight IsEnabled string mismatch for DoPreflight. Got: '%s', Want: '%s'", modSpecDo.IsEnabled, expectedCondition)
	}
	// For cfgDoPreflight (SkipPreflight = false), the string means it *should* be enabled.

	modSpecDefault := modulePreflight.NewPreflightModuleSpec(cfgDefaultPreflight)
	if modSpecDefault.IsEnabled != expectedCondition {
		t.Errorf("Preflight IsEnabled string mismatch for Default. Got: '%s', Want: '%s'", modSpecDefault.IsEnabled, expectedCondition)
	}
	// For cfgDefaultPreflight (SkipPreflight = false by default), the string means it *should* be enabled.

	// Basic assembly check
	if modSpecDefault.Name != "Preflight Checks and Setup" {
		t.Errorf("Name = %s", modSpecDefault.Name)
	}
	if len(modSpecDefault.Tasks) < 2 { // SystemChecks, SetupKernel
		t.Errorf("Expected at least 2 tasks, got %d", len(modSpecDefault.Tasks))
	}
}

func TestNewContainerdModule_Factory_IsEnabledLogic(t *testing.T) {
	cfgContainerd := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "containerd"},
		},
	}
	config.SetDefaults(cfgContainerd)

	cfgDocker := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "docker"},
		},
	}
	config.SetDefaults(cfgDocker)

	cfgEmptyRuntimeType := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: ""}, // Explicitly empty
		},
	}
	config.SetDefaults(cfgEmptyRuntimeType) // SetDefaults makes it "containerd"

	cfgNilRuntimeSpec := &config.Cluster{}
	config.SetDefaults(cfgNilRuntimeSpec) // SetDefaults makes ContainerRuntime non-nil and type "containerd"

	// Expected IsEnabled condition string from NewContainerdModuleSpec
	expectedCondition := "(cfg.Spec.ContainerRuntime == nil) || (cfg.Spec.ContainerRuntime.Type == '') || (cfg.Spec.ContainerRuntime.Type == 'containerd')"

	modSpecContainerd := moduleContainerd.NewContainerdModuleSpec(cfgContainerd)
	if modSpecContainerd.IsEnabled != expectedCondition {
		t.Errorf("Containerd IsEnabled string for 'containerd' type. Got: '%s', Want: '%s'", modSpecContainerd.IsEnabled, expectedCondition)
	}

	modSpecDocker := moduleContainerd.NewContainerdModuleSpec(cfgDocker)
	if modSpecDocker.IsEnabled != expectedCondition { // The string is the same, but its evaluation with cfgDocker would be false.
		t.Errorf("Containerd IsEnabled string for 'docker' type. Got: '%s', Want: '%s'", modSpecDocker.IsEnabled, expectedCondition)
	}

	modSpecEmpty := moduleContainerd.NewContainerdModuleSpec(cfgEmptyRuntimeType)
	if modSpecEmpty.IsEnabled != expectedCondition {
		t.Errorf("Containerd IsEnabled string for empty type. Got: '%s', Want: '%s'", modSpecEmpty.IsEnabled, expectedCondition)
	}

	modSpecNil := moduleContainerd.NewContainerdModuleSpec(cfgNilRuntime)
	if modSpecNil.IsEnabled != expectedCondition {
		t.Errorf("Containerd IsEnabled string for nil runtime spec. Got: '%s', Want: '%s'", modSpecNil.IsEnabled, expectedCondition)
	}

	if modSpecContainerd.Name != "Containerd Runtime" { t.Errorf("Name = %s", modSpecContainerd.Name) }
	if len(modSpecContainerd.Tasks) < 1 { // Expect InstallContainerdTaskSpec
		t.Errorf("Expected at least 1 task, got %d", len(modSpecContainerd.Tasks))
	}
	// Check the name of the first task, which should be the install task.
	// The actual name is defined within NewInstallContainerdTaskSpec.
	if !strings.Contains(modSpecContainerd.Tasks[0].Name, "InstallAndConfigureContainerd") {
		t.Errorf("Task name = '%s', expected to contain 'InstallAndConfigureContainerd'", modSpecContainerd.Tasks[0].Name)
	}
}

func TestNewEtcdModule_Factory_IsEnabledLogic(t *testing.T) {
	cfgManagedEtcd := &config.Cluster{
		Spec: config.ClusterSpec{
			Etcd: &config.EtcdSpec{Managed: true},
		},
	}
	config.SetDefaults(cfgManagedEtcd)

	cfgUnmanagedEtcd := &config.Cluster{
		Spec: config.ClusterSpec{
			Etcd: &config.EtcdSpec{Managed: false},
		},
	}
	config.SetDefaults(cfgUnmanagedEtcd)

	cfgNilEtcdSpec := &config.Cluster{}
	config.SetDefaults(cfgNilEtcdSpec) // SetDefaults initializes EtcdSpec

	// Expected IsEnabled condition string from NewEtcdModuleSpec
	expectedCondition := "cfg.Spec.Etcd != nil"

	modSpecManaged := moduleEtcd.NewEtcdModuleSpec(cfgManagedEtcd)
	if modSpecManaged.IsEnabled != expectedCondition {
		t.Errorf("Etcd IsEnabled string for managed Etcd. Got: '%s', Want: '%s'", modSpecManaged.IsEnabled, expectedCondition)
	}
	// For cfgManagedEtcd (Etcd spec is present), the condition is true.

	modSpecUnmanaged := moduleEtcd.NewEtcdModuleSpec(cfgUnmanagedEtcd)
	if modSpecUnmanaged.IsEnabled != expectedCondition {
		t.Errorf("Etcd IsEnabled string for unmanaged Etcd. Got: '%s', Want: '%s'", modSpecUnmanaged.IsEnabled, expectedCondition)
	}
	// For cfgUnmanagedEtcd (Etcd spec is present), the condition is true.
	// The original test checked Etcd.Managed, but the new IsEnabled string is "cfg.Spec.Etcd != nil".
	// This means the module is enabled if the Etcd spec exists, tasks inside will differ.
	// This aligns with the refactored NewEtcdModuleSpec's IsEnabled logic.

	modSpecNil := moduleEtcd.NewEtcdModuleSpec(cfgNilEtcdSpec)
	// After SetDefaults, cfgNilEtcdSpec.Spec.Etcd IS NOT nil. It's an empty EtcdSpec.
	// So, cfg.Spec.Etcd != nil is true.
	if modSpecNil.IsEnabled != expectedCondition {
		t.Errorf("Etcd IsEnabled string for nil Etcd spec initially. Got: '%s', Want: '%s'", modSpecNil.IsEnabled, expectedCondition)
	}
	// The original test implied it should be disabled if EtcdSpec was initially nil.
	// However, SetDefaults initializes cfg.Spec.Etcd, so cfg.Spec.Etcd != nil becomes true.
	// The new IsEnabled string "cfg.Spec.Etcd != nil" accurately reflects this.

	if modSpecManaged.Name != "Etcd Cluster Management" {t.Errorf("Name = %s", modSpecManaged.Name)}
	if len(modSpecManaged.Tasks) == 0 && cfgManagedEtcd.Spec.Etcd != nil {
		// If Etcd spec is present (as in cfgManagedEtcd), we expect some tasks.
		t.Errorf("Expected tasks for etcd module when Etcd spec is present, got %d", len(modSpecManaged.Tasks))
	}
}

// reflectDeepEqual helper can be removed if not used.
// func reflectDeepEqual(a, b interface{}) bool { return reflect.DeepEqual(a, b) }
