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

	modSpecSkip := modulePreflight.NewPreflightModule(cfgSkipPreflight)
	if modSpecSkip.IsEnabled == nil {t.Fatal("Preflight IsEnabled is nil")}
	if modSpecSkip.IsEnabled(cfgSkipPreflight) {
		t.Error("Preflight module should be disabled if cfg.Spec.Global.SkipPreflight is true")
	}

	modSpecDo := modulePreflight.NewPreflightModule(cfgDoPreflight)
	if modSpecDo.IsEnabled == nil {t.Fatal("Preflight IsEnabled is nil")}
	if !modSpecDo.IsEnabled(cfgDoPreflight) {
		t.Error("Preflight module should be enabled if cfg.Spec.Global.SkipPreflight is false")
	}

	modSpecDefault := modulePreflight.NewPreflightModule(cfgDefaultPreflight)
	if modSpecDefault.IsEnabled == nil {t.Fatal("Preflight IsEnabled is nil")}
	if !modSpecDefault.IsEnabled(cfgDefaultPreflight) { // Default for SkipPreflight (bool) is false
		t.Error("Preflight module should be enabled by default if SkipPreflight is not set (defaults to false)")
	}

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

	modSpecContainerd := moduleContainerd.NewContainerdModule(cfgContainerd)
	if modSpecContainerd.IsEnabled == nil {t.Fatal("Containerd IsEnabled is nil")}
	if !modSpecContainerd.IsEnabled(cfgContainerd) {
		t.Error("Containerd module should be enabled when type is 'containerd'")
	}

	modSpecDocker := moduleContainerd.NewContainerdModule(cfgDocker)
	if modSpecDocker.IsEnabled == nil {t.Fatal("Containerd IsEnabled is nil")}
	if modSpecDocker.IsEnabled(cfgDocker) { // cfgDocker has type "docker"
		t.Error("Containerd module should be disabled when type is 'docker'")
	}

	modSpecEmpty := moduleContainerd.NewContainerdModule(cfgEmptyRuntimeType)
	if modSpecEmpty.IsEnabled == nil {t.Fatal("Containerd IsEnabled is nil")}
	if !modSpecEmpty.IsEnabled(cfgEmptyRuntimeType) { // After defaults, type is "containerd"
		t.Errorf("Containerd module should be enabled when type is defaulted to 'containerd', got type: %s", cfgEmptyRuntimeType.Spec.ContainerRuntime.Type)
	}

	modSpecNil := moduleContainerd.NewContainerdModule(cfgNilRuntime)
	if modSpecNil.IsEnabled == nil {t.Fatal("Containerd IsEnabled is nil")}
	if !modSpecNil.IsEnabled(cfgNilRuntime) { // After defaults, type is "containerd"
	    t.Logf("Runtime type after default for nil spec: %s", cfgNilRuntime.Spec.ContainerRuntime.Type)
		t.Error("Containerd module should be enabled when ContainerRuntimeSpec is nil (defaults to containerd type)")
	}

	if modSpecContainerd.Name != "Containerd Runtime" { t.Errorf("Name = %s", modSpecContainerd.Name) }
	if len(modSpecContainerd.Tasks) < 1 {
		t.Errorf("Expected at least 1 task, got %d", len(modSpecContainerd.Tasks))
	}
	if modSpecContainerd.Tasks[0].Name != "Install and Configure Containerd" {
		t.Errorf("Task name = '%s', want 'Install and Configure Containerd'", modSpecContainerd.Tasks[0].Name)
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
	config.SetDefaults(cfgNilEtcdSpec) // SetDefaults initializes EtcdSpec, Managed defaults to false

	modSpecManaged := moduleEtcd.NewEtcdModule(cfgManagedEtcd)
	if modSpecManaged.IsEnabled == nil {t.Fatal("Etcd IsEnabled is nil")}
	if !modSpecManaged.IsEnabled(cfgManagedEtcd) {
		t.Error("Etcd module should be enabled when Etcd.Managed is true")
	}

	modSpecUnmanaged := moduleEtcd.NewEtcdModule(cfgUnmanagedEtcd)
	if modSpecUnmanaged.IsEnabled == nil {t.Fatal("Etcd IsEnabled is nil")}
	if modSpecUnmanaged.IsEnabled(cfgUnmanagedEtcd) {
		t.Error("Etcd module should be disabled when Etcd.Managed is false")
	}

	modSpecNil := moduleEtcd.NewEtcdModule(cfgNilEtcdSpec)
	if modSpecNil.IsEnabled == nil {t.Fatal("Etcd IsEnabled is nil")}
	if modSpecNil.IsEnabled(cfgNilEtcdSpec) { // EtcdSpec.Managed defaults to false
		t.Errorf("Etcd module should be disabled when EtcdSpec is nil (Managed defaults to false), got Etcd.Managed: %v", cfgNilEtcdSpec.Spec.Etcd.Managed)
	}

	if modSpecManaged.Name != "Etcd Cluster Management" {t.Errorf("Name = %s", modSpecManaged.Name)}
	if len(modSpecManaged.Tasks) < 2 { // Based on placeholder tasks in factory
		t.Errorf("Expected at least 2 tasks for etcd, got %d", len(modSpecManaged.Tasks))
	}
}

// reflectDeepEqual helper can be removed if not used.
// func reflectDeepEqual(a, b interface{}) bool { return reflect.DeepEqual(a, b) }
