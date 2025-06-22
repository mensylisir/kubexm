package preflight

import (
	"reflect" // For DeepEqual
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Changed from config.Cluster
	// "github.com/mensylisir/kubexm/pkg/config" // Old import
	// "github.com/mensylisir/kubexm/pkg/spec"    // For spec.TaskSpec, spec.StepSpec - No longer used directly by new task structure
	// "github.com/mensylisir/kubexm/pkg/step"    // For step.GetSpecTypeName - No longer used directly

	// Import Step types to check them by type assertion if needed, or check their properties
	// stepPreflight "github.com/mensylisir/kubexm/pkg/step/preflight" // Specific step types for assertion

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mensylisir/kubexm/pkg/task" // For task.Task interface
	// For mock context - will define a local one or use a shared one if available
	"context"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"

)

// mockPreflightTaskTestContext implements task.TaskContext for preflight task tests.
type mockPreflightTaskTestContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	controlHost   connector.Host
	clusterCfg    *v1alpha1.Cluster
	taskCache     cache.TaskCache
	moduleCache   cache.ModuleCache
	pipelineCache cache.PipelineCache
}

func newMockPreflightTaskTestContext(t *testing.T, cfg *v1alpha1.Cluster) task.TaskContext {
	l, _ := logger.New(logger.DefaultConfig())
	defaultCtrlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Port: 22, User:"test", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(defaultCtrlHostSpec)

	currentClusterCfg := cfg
	if currentClusterCfg == nil {
		currentClusterCfg = &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "default-test-cluster"}}
	}
	v1alpha1.SetDefaults_Cluster(currentClusterCfg) // Apply defaults

	return &mockPreflightTaskTestContext{
		logger:        l,
		goCtx:         context.Background(),
		controlHost:   ctrlHost,
		clusterCfg:    currentClusterCfg,
		taskCache:     cache.NewMemoryCache(),
		moduleCache:   cache.NewMemoryCache(),
		pipelineCache: cache.NewMemoryCache(),
	}
}
func (m *mockPreflightTaskTestContext) GoContext() context.Context          { return m.goCtx }
func (m *mockPreflightTaskTestContext) GetLogger() *logger.Logger           { return m.logger }
func (m *mockPreflightTaskTestContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterCfg }
func (m *mockPreflightTaskTestContext) PipelineCache() cache.PipelineCache  { return m.pipelineCache }
func (m *mockPreflightTaskTestContext) GetGlobalWorkDir() string          { return "/tmp/_preflight_task_test" }
func (m *mockPreflightTaskTestContext) ModuleCache() cache.ModuleCache    { return m.moduleCache }
func (m *mockPreflightTaskTestContext) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if m.clusterCfg != nil {
		for _, hSpec := range m.clusterCfg.Spec.Hosts {
			for _, r := range hSpec.Roles {
				if r == role {hosts = append(hosts, connector.NewHostFromSpec(hSpec)); break }
			}
		}
	}
	if role == common.ControlNodeRole {
		found := false
		for _, h := range hosts {
			if h.GetName() == m.controlHost.GetName() {
				found = true
				break
			}
		}
		if !found {
			hosts = append(hosts, m.controlHost)
		}
	}
	return hosts, nil
}
func (m *mockPreflightTaskTestContext) GetHostFacts(host connector.Host) (*runner.Facts, error) { return &runner.Facts{OS: &connector.OS{Arch:"amd64"}}, nil }
func (m *mockPreflightTaskTestContext) TaskCache() cache.TaskCache          { return m.taskCache }
func (m *mockPreflightTaskTestContext) GetControlNode() (connector.Host, error) { return m.controlHost, nil }

var _ task.TaskContext = (*mockPreflightTaskTestContext)(nil)


// Test logic from old task_test.go, adapted for new Task structure (Plan method, ExecutionFragment)
// The original tests were checking spec.TaskSpec and spec.StepSpec which are no longer directly built by task factories.
// Tasks now return ExecutionFragments. Tests need to verify the structure of these fragments.

func TestNewSystemChecksTask_Plan_WithConfig(t *testing.T) {
	cfg := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			System: &v1alpha1.SystemSpec{}, // Assuming PreflightConfig was merged into SystemSpec
			// If PreflightConfig is still separate and used by NewSystemChecksTask:
			// Preflight: &v1alpha1.PreflightConfig{
			// MinCPUCores: 3,
			// MinMemoryMB: 3072,
			// DisableSwap: true,
			// },
		},
	}
	// Assuming SystemChecksTask uses fields that might be in PreflightConfig, need to set them.
	// For now, let's assume it checks some defaults or has internal logic.
	// If NewSystemChecksTask takes cfg, it's an old pattern. The new pattern is for Plan to use ctx.GetClusterConfig().

	taskInstance := NewSystemChecksTask() // New factory might not take config directly
	mockCtx := newMockPreflightTaskTestContext(t, cfg)

	fragment, err := taskInstance.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	assert.NotEmpty(t, fragment.Nodes, "SystemChecksTask Plan should generate nodes")

	// TODO: Inspect fragment.Nodes to verify that CheckCPUStep, CheckMemoryStep, DisableSwapStep
	// (or their equivalents) are present and configured based on cfg.
	// This requires knowing the NodeIDs and Step types generated by SystemChecksTask.Plan().
	// Example (conceptual):
	// foundCPU, foundMem, foundSwap := false, false, false
	// for _, node := range fragment.Nodes {
	//  if _, ok := node.Step.(*preflight.CheckCPUStep); ok { foundCPU = true }
	//  // ... similar checks
	// }
	// assert.True(t, foundCPU && foundMem && foundSwap)
}

func TestNewSetupKernelTask_Plan_WithConfig(t *testing.T) {
	customModules := []string{"custom_mod1", "custom_mod2"}
	customSysctl := map[string]string{"custom.param": "1", "another.param": "2"}
	cfg := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			System: &v1alpha1.SystemSpec{ // Assuming KernelConfig was merged into SystemSpec
				Modules:      customModules,
				SysctlParams: customSysctl,
			},
		},
	}
	taskInstance := NewSetupKernelTask() // New factory might not take config
	mockCtx := newMockPreflightTaskTestContext(t, cfg)

	fragment, err := taskInstance.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	assert.NotEmpty(t, fragment.Nodes, "SetupKernelTask Plan should generate nodes")

	// TODO: Inspect fragment.Nodes to verify that LoadKernelModulesStep and SetSystemConfigStep
	// are present and configured with customModules and customSysctl.
}
