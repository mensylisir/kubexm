package containerd

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/task" // For task.Task interface
	// stepSpecContainerd "github.com/mensylisir/kubexm/pkg/step/containerd" // For checking step types if needed
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "github.com/mensylisir/kubexm/pkg/module/preflight" // Cannot use this
	"context"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// mockTaskTestContext implements task.TaskContext for containerd task tests.
type mockTaskTestContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	controlHost   connector.Host
	clusterCfg    *v1alpha1.Cluster
	// Caches and other fields as needed by TaskContext
	pipelineCacheOverride cache.PipelineCache
	moduleCacheOverride   cache.ModuleCache
	taskCacheOverride     cache.TaskCache
}

func newMockTaskTestContext(t *testing.T, cfg *v1alpha1.Cluster) task.TaskContext {
	l, _ := logger.New(logger.DefaultConfig())
	defaultCtrlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Port: 22, User:"test", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(defaultCtrlHostSpec)

	currentClusterCfg := cfg
	if currentClusterCfg == nil {
		currentClusterCfg = &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "default-test-cluster"}}
	}
	v1alpha1.SetDefaults_Cluster(currentClusterCfg)

	return &mockTaskTestContext{
		logger:                l,
		goCtx:                 context.Background(),
		controlHost:           ctrlHost,
		clusterCfg:            currentClusterCfg,
		pipelineCacheOverride: cache.NewMemoryCache(),
		moduleCacheOverride:   cache.NewMemoryCache(),
		taskCacheOverride:     cache.NewMemoryCache(),
	}
}

func (m *mockTaskTestContext) GoContext() context.Context                               { return m.goCtx }
func (m *mockTaskTestContext) GetLogger() *logger.Logger                                  { return m.logger }
func (m *mockTaskTestContext) GetClusterConfig() *v1alpha1.Cluster                      { return m.clusterCfg }
func (m *mockTaskTestContext) PipelineCache() cache.PipelineCache                       { return m.pipelineCacheOverride }
func (m *mockTaskTestContext) GetGlobalWorkDir() string                                 { return "/tmp/_task_test" }
func (m *mockTaskTestContext) ModuleCache() cache.ModuleCache                         { return m.moduleCacheOverride }
func (m *mockTaskTestContext) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if m.clusterCfg != nil {
		for _, hSpec := range m.clusterCfg.Spec.Hosts {
			for _, r := range hSpec.Roles {
				if r == role {
					hosts = append(hosts, connector.NewHostFromSpec(hSpec))
					break
				}
			}
		}
	}
	if role == common.ControlNodeRole && len(hosts) == 0 { // Ensure control node can be fetched
		 for _,h := range m.clusterCfg.Spec.Hosts { // Check if control node is already in hosts
			isCtrl := false
			for _, r := range h.Roles { if r == common.ControlNodeRole {isCtrl = true; break}}
			if h.Name == common.ControlNodeHostName && isCtrl { return hosts, nil}
		 }
		 hosts = append(hosts, m.controlHost)
	}
	return hosts, nil
}
func (m *mockTaskTestContext) GetHostFacts(host connector.Host) (*runner.Facts, error)  { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockTaskTestContext) TaskCache() cache.TaskCache                               { return m.taskCacheOverride }
func (m *mockTaskTestContext) GetControlNode() (connector.Host, error)                  { return m.controlHost, nil }

var _ task.TaskContext = (*mockTaskTestContext)(nil)


// This test is adapted from the old pkg/task/task_test.go
// It needs to be adjusted because NewInstallContainerdTask now takes roles,
// and configurations are fetched from the context during Plan.
func TestNewInstallContainerdTask_Factory_And_Plan_Configuration(t *testing.T) {
	roles := []string{common.MasterRole, common.WorkerRole}

	// Test Factory part
	taskInstance := NewInstallContainerdTask(roles)
	require.NotNil(t, taskInstance)
	assert.Equal(t, "InstallAndConfigureContainerd", taskInstance.Name())

	if concreteTask, ok := taskInstance.(*InstallContainerdTask); ok {
		assert.Equal(t, roles, concreteTask.RunOnRoles)
	} else {
		t.Fatalf("NewInstallContainerdTask did not return *InstallContainerdTask")
	}

	// Test Plan part - for checking if config is picked up correctly
	// This requires a mock TaskContext that can provide a ClusterConfig.
	clusterCfg := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ContainerRuntime: &v1alpha1.ContainerRuntimeConfig{
				Type:    v1alpha1.ContainerdRuntime,
				Version: "1.7.1",
				Containerd: &v1alpha1.ContainerdConfig{
					SandboxImage: "my.reg/pause:custom",
					RegistryMirrors: map[string]v1alpha1.RegistryMirror{
						"docker.io": {Endpoints: []string{"https://my.mirror.com"}},
					},
					InsecureRegistries: []string{"insecure.reg:5000"},
					ConfigPath:         "/custom/containerd.toml",
					ExtraTomlContent:   "[plugins.\"io.containerd.myplugin\"]\n  debug = true",
				},
			},
			// Minimal hosts for the mock context to function
			Hosts: []v1alpha1.HostSpec{
				{Name: "test-master", Roles: []string{common.MasterRole}, Address: "1.2.3.4", User:"root", Port:22},
			},
		},
	}
	v1alpha1.SetDefaults_Cluster(clusterCfg) // Apply defaults

	// Using a mock context similar to the one in preflight_module_test.go
	// Ideally, there should be a shared test utility for these mock contexts.
	mockTaskCtx := newMockTaskTestContext(t, clusterCfg)

	// Check IsRequired
	isRequired, err := taskInstance.IsRequired(mockTaskCtx)
	require.NoError(t, err)
	assert.True(t, isRequired, "InstallContainerdTask should be required for containerd runtime type")

	// Execute Plan to populate task fields from config
	_, err = taskInstance.Plan(mockTaskCtx) // We are interested in side effects on taskInstance for this test part
	require.NoError(t, err, "Plan should not fail for valid config")

	concreteTaskForPlan, ok := taskInstance.(*InstallContainerdTask)
	require.True(t, ok, "Could not assert to *InstallContainerdTask after plan")

	assert.Equal(t, "my.reg/pause:custom", concreteTaskForPlan.SandboxImage)
	assert.NotNil(t, concreteTaskForPlan.RegistryMirrors)
	assert.Contains(t, concreteTaskForPlan.RegistryMirrors, "docker.io")
	assert.Equal(t, []string{"https://my.mirror.com"}, concreteTaskForPlan.RegistryMirrors["docker.io"].Endpoints)
	assert.Equal(t, []string{"insecure.reg:5000"}, concreteTaskForPlan.InsecureRegistries)
	assert.Equal(t, "/custom/containerd.toml", concreteTaskForPlan.ContainerdConfigPath)
	assert.Equal(t, "[plugins.\"io.containerd.myplugin\"]\n  debug = true", concreteTaskForPlan.ExtraTomlContent)

	// Further tests could inspect the generated ExecutionFragment from Plan,
	// similar to how TestNewSystemChecksTask_Factory_WithConfig checked steps.
	// For now, this focuses on config propagation into the task struct.
}
