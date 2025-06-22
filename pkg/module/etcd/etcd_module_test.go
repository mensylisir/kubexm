package etcd

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/module" // For module.Module interface
	"context" // Required by mock context
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/task"
)

// This test is adapted from the old pkg/module/module_test.go
// Similar to containerd, NewEtcdModule() no longer takes *v1alpha1.Cluster
// and IsEnabled logic is now within the Plan method.
func TestNewEtcdModule_Factory_And_Plan_IsEnabledLogic(t *testing.T) {
	cfgManagedEtcd := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			Etcd: &v1alpha1.EtcdConfig{Type: "kubexm", Managed: true}, // Assuming Managed controls this module now
		},
	}
	v1alpha1.SetDefaults_Cluster(cfgManagedEtcd)

	cfgUnmanagedEtcd := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			Etcd: &v1alpha1.EtcdConfig{Type: "external", Managed: false},
		},
	}
	v1alpha1.SetDefaults_Cluster(cfgUnmanagedEtcd)

	cfgNilEtcdSpec := &v1alpha1.Cluster{}
	v1alpha1.SetDefaults_Cluster(cfgNilEtcdSpec) // Etcd.Managed defaults to false

	// Test module creation
	modInstance := NewEtcdModule() // NewEtcdModule now has no params
	if modInstance.Name() != "EtcdClusterManagement" { // Name from NewEtcdModule
		t.Errorf("Expected module name 'EtcdClusterManagement', got '%s'", modInstance.Name())
	}
	// Check tasks if necessary, e.g., if len(modInstance.Tasks()) ...

	// Mock context for Plan
	mockCtxManaged := newMockEtcdTestContext(cfgManagedEtcd)
	mockCtxUnmanaged := newMockEtcdTestContext(cfgUnmanagedEtcd)
	mockCtxNil := newMockEtcdTestContext(cfgNilEtcdSpec)

	// Test Plan with managed etcd
	planResultManaged, err := modInstance.Plan(mockCtxManaged)
	if err != nil {
		t.Fatalf("Plan failed for managed etcd config: %v", err)
	}
	if planResultManaged == nil || len(planResultManaged.Nodes) == 0 {
		t.Error("Etcd module should be enabled and produce a non-empty plan when Etcd.Managed is true")
	}

	// Test Plan with unmanaged etcd
	planResultUnmanaged, err := modInstance.Plan(mockCtxUnmanaged)
	if err != nil {
		t.Fatalf("Plan failed for unmanaged etcd config: %v", err)
	}
	if planResultUnmanaged == nil || len(planResultUnmanaged.Nodes) != 0 {
		t.Error("Etcd module should be disabled and produce an empty plan when Etcd.Managed is false")
	}

	// Test Plan with nil etcd spec (defaults to Managed=false)
	planResultNil, err := modInstance.Plan(mockCtxNil)
	if err != nil {
		t.Fatalf("Plan failed for nil etcd spec config: %v", err)
	}
	if planResultNil == nil || len(planResultNil.Nodes) != 0 {
	    t.Logf("Etcd.Managed after default for nil spec: %v", cfgNilEtcdSpec.Spec.Etcd.Managed)
		t.Error("Etcd module should be disabled (defaulted to Managed=false from nil spec), but plan is not empty")
	}
}


// mockModuleContextRealLogger from containerd_module_test.go can be reused or adapted.
// For simplicity, copying and renaming for clarity.
type mockEtcdModuleTestContext struct {
	clusterConfig *v1alpha1.Cluster
	realLogger    *logger.Logger
}

func (m *mockEtcdModuleTestContext) GoContext() context.Context                               { return context.Background() }
func (m *mockEtcdModuleTestContext) GetLogger() *logger.Logger                                  {
	if m.realLogger == nil {
		l, _ := logger.New(logger.DefaultConfig())
		m.realLogger = l
	}
	return m.realLogger
}
func (m *mockEtcdModuleTestContext) GetClusterConfig() *v1alpha1.Cluster                      { return m.clusterConfig }
func (m *mockEtcdModuleTestContext) PipelineCache() cache.PipelineCache                       { return cache.NewMemoryCache() }
func (m *mockEtcdModuleTestContext) GetGlobalWorkDir() string                                 { return "/tmp" }
func (m *mockEtcdModuleTestContext) GetEngine() engine.Engine                                 { return nil }
func (m *mockEtcdModuleTestContext) ModuleCache() cache.ModuleCache                         { return cache.NewMemoryCache() }
func (m *mockEtcdModuleTestContext) GetHostsByRole(role string) ([]connector.Host, error)     { return nil, nil }
func (m *mockEtcdModuleTestContext) GetHostFacts(host connector.Host) (*runner.Facts, error)  { return nil, nil }
func (m *mockEtcdModuleTestContext) TaskCache() cache.TaskCache                               { return cache.NewMemoryCache() }
func (m *mockEtcdModuleTestContext) GetControlNode() (connector.Host, error)                  { return nil, nil }

var _ module.ModuleContext = (*mockEtcdModuleTestContext)(nil)
var _ task.TaskContext = (*mockEtcdModuleTestContext)(nil) // Ensure it also implements TaskContext

// Helper to create mock context for etcd tests
func newMockEtcdTestContext(cfg *v1alpha1.Cluster) *mockEtcdModuleTestContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockEtcdModuleTestContext{clusterConfig: cfg, realLogger: l}
}
