package containerd

import (
	"context"
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For v1alpha1.Cluster
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common" // For common constants if used by mock context
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/module" // For module.Module interface
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/task" // For task.TaskContext
	"github.com/stretchr/testify/assert"   // Added for assertions
	"github.com/stretchr/testify/require"  // Added for require
)

// Consolidated mock context definition
type mockContainerdTestModuleContext struct {
	clusterConfig *v1alpha1.Cluster
	realLogger    *logger.Logger
	pipelineCache cache.PipelineCache
	moduleCache   cache.ModuleCache
	taskCache     cache.TaskCache
	globalWorkDir string
	engine        engine.Engine
	controlHost   connector.Host
}

// newMockContainerdTestContext is a helper to create a mock context for containerd module tests.
// It implements both module.ModuleContext and task.TaskContext for convenience.
func newMockContainerdTestContext(t *testing.T, cfg *v1alpha1.Cluster) *mockContainerdTestModuleContext {
	l, _ := logger.New(logger.DefaultConfig())
	l.SetLogLevel(logger.DebugLevel)

	currentClusterCfg := cfg
	if currentClusterCfg == nil {
		currentClusterCfg = &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "default-test-cluster"}}
	}
	v1alpha1.SetDefaults_Cluster(currentClusterCfg) // Apply defaults

	defaultCtrlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Port: 22, User: "test", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(defaultCtrlHostSpec)

	return &mockContainerdTestModuleContext{
		clusterConfig: currentClusterCfg,
		realLogger:    l,
		pipelineCache: cache.NewMemoryCache(),
		moduleCache:   cache.NewMemoryCache(),
		taskCache:     cache.NewMemoryCache(),
		globalWorkDir: "/tmp/_containerd_mod_test",
		controlHost:   ctrlHost,
	}
}

func (m *mockContainerdTestModuleContext) GoContext() context.Context          { return context.Background() }
func (m *mockContainerdTestModuleContext) GetLogger() *logger.Logger           { return m.realLogger }
func (m *mockContainerdTestModuleContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockContainerdTestModuleContext) PipelineCache() cache.PipelineCache  { return m.pipelineCache }
func (m *mockContainerdTestModuleContext) GetGlobalWorkDir() string          { return m.globalWorkDir }
func (m *mockContainerdTestModuleContext) GetEngine() engine.Engine            { return m.engine }
func (m *mockContainerdTestModuleContext) ModuleCache() cache.ModuleCache    { return m.moduleCache }
func (m *mockContainerdTestModuleContext) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if m.clusterConfig != nil && m.clusterConfig.Spec.Hosts != nil {
		for _, hSpec := range m.clusterConfig.Spec.Hosts {
			isRolePresent := false
			for _, r := range hSpec.Roles {
				if r == role {
					isRolePresent = true
					break
				}
			}
			if isRolePresent {
				hosts = append(hosts, connector.NewHostFromSpec(hSpec))
			}
		}
	}
	if role == common.ControlNodeRole {
		foundCtrl := false
		for _, h := range hosts {
			if h.GetName() == m.controlHost.GetName() {
				foundCtrl = true
				break
			}
		}
		if !foundCtrl {
			hosts = append(hosts, m.controlHost)
		}
	}
	return hosts, nil
}
func (m *mockContainerdTestModuleContext) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil
}
func (m *mockContainerdTestModuleContext) TaskCache() cache.TaskCache          { return m.taskCache }
func (m *mockContainerdTestModuleContext) GetControlNode() (connector.Host, error) { return m.controlHost, nil }

var _ module.ModuleContext = (*mockContainerdTestModuleContext)(nil)
var _ task.TaskContext = (*mockContainerdTestModuleContext)(nil)

// TestContainerdModule_Factory_And_Plan_IsEnabledLogic tests NewContainerdModule factory and Plan's IsEnabled logic.
func TestContainerdModule_Factory_And_Plan_IsEnabledLogic(t *testing.T) {
	cfgContainerd := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ContainerRuntime: &v1alpha1.ContainerRuntimeConfig{Type: v1alpha1.ContainerdRuntime},
		},
	}
	cfgDocker := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ContainerRuntime: &v1alpha1.ContainerRuntimeConfig{Type: v1alpha1.DockerRuntime},
		},
	}
	cfgEmptyRuntimeType := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ContainerRuntime: &v1alpha1.ContainerRuntimeConfig{Type: ""}, // Will default to containerd
		},
	}
	cfgNilRuntimeSpec := &v1alpha1.Cluster{} // Will default to containerd

	modInstance := NewContainerdModule()
	require.NotNil(t, modInstance, "NewContainerdModule should return a non-nil instance")
	assert.Equal(t, "ContainerdRuntime", modInstance.Name(), "Module name mismatch")
	require.NotEmpty(t, modInstance.Tasks(), "Module should have tasks")
	// Can add more specific task checks if needed:
	// assert.Equal(t, "InstallAndConfigureContainerd", modInstance.Tasks()[0].Name())


	testCases := []struct {
		name           string
		config         *v1alpha1.Cluster
		expectPlan     bool // True if a non-empty plan is expected
		expectedErrMsg string
	}{
		{"ContainerdRuntime", cfgContainerd, true, ""},
		{"DockerRuntime", cfgDocker, false, ""},
		{"EmptyRuntimeType (defaults to containerd)", cfgEmptyRuntimeType, true, ""},
		{"NilRuntimeSpec (defaults to containerd)", cfgNilRuntimeSpec, true, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtx := newMockContainerdTestContext(t, tc.config)
			planResult, err := modInstance.Plan(mockCtx)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "Plan failed for config: %s", tc.name)
				require.NotNil(t, planResult, "Plan result should not be nil for config: %s", tc.name)
				if tc.expectPlan {
					assert.NotEmpty(t, planResult.Nodes, "Plan should be non-empty for config: %s. Actual type: %s", tc.name, mockCtx.GetClusterConfig().Spec.ContainerRuntime.Type)
				} else {
					assert.Empty(t, planResult.Nodes, "Plan should be empty for config: %s. Actual type: %s", tc.name, mockCtx.GetClusterConfig().Spec.ContainerRuntime.Type)
				}
			}
		})
	}
}
