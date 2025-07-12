package kubernetes

import (
	"context"
	"testing"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

// MockModuleContext implements runtime.ModuleContext for testing
type MockModuleContext struct {
	clusterConfig interface{}
	hosts        []string
	currentHost  string
	cache        map[string]interface{}
}

func (m *MockModuleContext) GetClusterConfig() interface{} {
	return m.clusterConfig
}

func (m *MockModuleContext) GetAvailableHosts() []string {
	return m.hosts
}

func (m *MockModuleContext) GetCurrentHost() string {
	return m.currentHost
}

func (m *MockModuleContext) GetCache() runtime.Cache {
	return &MockCache{data: m.cache}
}

func (m *MockModuleContext) GetLogger() runtime.Logger {
	return &MockLogger{}
}

func (m *MockModuleContext) GetContext() context.Context {
	return context.Background()
}

// MockCache implements runtime.Cache for testing
type MockCache struct {
	data map[string]interface{}
}

func (c *MockCache) Set(key string, value interface{}) error {
	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	c.data[key] = value
	return nil
}

func (c *MockCache) Get(key string) (interface{}, bool) {
	if c.data == nil {
		return nil, false
	}
	value, exists := c.data[key]
	return value, exists
}

func (c *MockCache) Delete(key string) error {
	if c.data != nil {
		delete(c.data, key)
	}
	return nil
}

func (c *MockCache) Clear() error {
	c.data = make(map[string]interface{})
	return nil
}

// MockLogger implements runtime.Logger for testing
type MockLogger struct {
	logs []string
}

func (l *MockLogger) Debug(args ...interface{}) {
	l.logs = append(l.logs, "DEBUG")
}

func (l *MockLogger) Debugf(format string, args ...interface{}) {
	l.logs = append(l.logs, "DEBUG")
}

func (l *MockLogger) Info(args ...interface{}) {
	l.logs = append(l.logs, "INFO")
}

func (l *MockLogger) Infof(format string, args ...interface{}) {
	l.logs = append(l.logs, "INFO")
}

func (l *MockLogger) Warn(args ...interface{}) {
	l.logs = append(l.logs, "WARN")
}

func (l *MockLogger) Warnf(format string, args ...interface{}) {
	l.logs = append(l.logs, "WARN")
}

func (l *MockLogger) Error(args ...interface{}) {
	l.logs = append(l.logs, "ERROR")
}

func (l *MockLogger) Errorf(format string, args ...interface{}) {
	l.logs = append(l.logs, "ERROR")
}

func (l *MockLogger) Fatal(args ...interface{}) {
	l.logs = append(l.logs, "FATAL")
}

func (l *MockLogger) Fatalf(format string, args ...interface{}) {
	l.logs = append(l.logs, "FATAL")
}

func TestNewCoreComponentsModule(t *testing.T) {
	module := NewCoreComponentsModule()
	
	if module == nil {
		t.Fatal("NewCoreComponentsModule() returned nil")
	}

	// Verify it implements the Module interface
	if _, ok := module.(module.Module); !ok {
		t.Error("CoreComponentsModule does not implement module.Module interface")
	}

	// Verify it's the correct type
	if _, ok := module.(*CoreComponentsModule); !ok {
		t.Errorf("NewCoreComponentsModule() returned wrong type: %T", module)
	}
}

func TestCoreComponentsModule_GetName(t *testing.T) {
	module := NewCoreComponentsModule()
	
	// This assumes the BaseModule implements GetName() method
	if nameGetter, ok := module.(interface{ GetName() string }); ok {
		name := nameGetter.GetName()
		expectedName := "CoreComponentsInstallation"
		if name != expectedName {
			t.Errorf("GetName() returned %s, want %s", name, expectedName)
		}
	} else {
		t.Skip("Module does not have GetName() method")
	}
}

func TestCoreComponentsModule_Plan(t *testing.T) {
	module := NewCoreComponentsModule().(*CoreComponentsModule)
	
	// Create mock context
	mockCtx := &MockModuleContext{
		clusterConfig: map[string]interface{}{
			"containerRuntime": "containerd",
		},
		hosts:       []string{"master1", "worker1", "worker2"},
		currentHost: "master1",
		cache:       make(map[string]interface{}),
	}

	fragment, err := module.Plan(mockCtx)
	
	if err != nil {
		t.Errorf("Plan() returned error: %v", err)
	}

	if fragment == nil {
		t.Fatal("Plan() returned nil fragment")
	}

	// Since current implementation returns empty fragment, we can test that
	// In a complete implementation, we would test for proper task creation and dependencies
}

func TestCoreComponentsModule_GetTasks(t *testing.T) {
	module := NewCoreComponentsModule()
	
	// This assumes the BaseModule implements GetTasks() method
	if taskGetter, ok := module.(interface{ GetTasks() []task.Task }); ok {
		tasks := taskGetter.GetTasks()
		
		// Currently expects empty tasks due to placeholder implementation
		if len(tasks) != 0 {
			t.Errorf("GetTasks() returned %d tasks, expected 0 (placeholder implementation)", len(tasks))
		}
	} else {
		t.Skip("Module does not have GetTasks() method")
	}
}

func TestCoreComponentsModule_InterfaceCompliance(t *testing.T) {
	module := NewCoreComponentsModule()
	
	// Test that the module implements the Module interface completely
	var _ module.Module = module
	
	// Test Plan method exists and is callable
	mockCtx := &MockModuleContext{
		cache: make(map[string]interface{}),
	}
	
	_, err := module.Plan(mockCtx)
	if err != nil {
		t.Errorf("Module.Plan() failed: %v", err)
	}
}

func TestCoreComponentsModule_PlanWithDifferentConfigs(t *testing.T) {
	module := NewCoreComponentsModule().(*CoreComponentsModule)
	
	tests := []struct {
		name          string
		clusterConfig interface{}
		expectError   bool
	}{
		{
			name: "containerd runtime",
			clusterConfig: map[string]interface{}{
				"containerRuntime": "containerd",
			},
			expectError: false,
		},
		{
			name: "docker runtime",
			clusterConfig: map[string]interface{}{
				"containerRuntime": "docker",
			},
			expectError: false,
		},
		{
			name:          "nil config",
			clusterConfig: nil,
			expectError:   false, // Current implementation doesn't fail on nil config
		},
		{
			name:          "empty config",
			clusterConfig: map[string]interface{}{},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtx := &MockModuleContext{
				clusterConfig: tt.clusterConfig,
				hosts:         []string{"host1"},
				cache:         make(map[string]interface{}),
			}

			fragment, err := module.Plan(mockCtx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if fragment == nil && err == nil {
				t.Error("Expected fragment but got nil (without error)")
			}
		})
	}
}

func TestCoreComponentsModule_PlanWithNoHosts(t *testing.T) {
	module := NewCoreComponentsModule().(*CoreComponentsModule)
	
	mockCtx := &MockModuleContext{
		clusterConfig: map[string]interface{}{},
		hosts:         []string{}, // No hosts
		cache:         make(map[string]interface{}),
	}

	fragment, err := module.Plan(mockCtx)
	
	// This should not fail in current implementation
	if err != nil {
		t.Errorf("Plan() with no hosts returned error: %v", err)
	}
	if fragment == nil {
		t.Error("Plan() with no hosts returned nil fragment")
	}
}

// Benchmark test for module creation
func BenchmarkNewCoreComponentsModule(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCoreComponentsModule()
	}
}

// Benchmark test for planning
func BenchmarkCoreComponentsModule_Plan(b *testing.B) {
	module := NewCoreComponentsModule().(*CoreComponentsModule)
	mockCtx := &MockModuleContext{
		clusterConfig: map[string]interface{}{
			"containerRuntime": "containerd",
		},
		hosts: []string{"master1", "worker1"},
		cache: make(map[string]interface{}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = module.Plan(mockCtx)
	}
}