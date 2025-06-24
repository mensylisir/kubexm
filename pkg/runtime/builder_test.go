package runtime

import (
	"context"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/plan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// --- Mock Implementations ---

// mockEngine is a simple mock for the engine.Engine interface.
type mockEngine struct{}

func (m *mockEngine) Execute(ctx engine.EngineExecuteContext, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	return nil, nil
}
func (m *mockEngine) Plan(ctx context.Context, modules ...interface{}) (interface{}, error) {
	return nil, nil
}

// mockConnector is a mock for the connector.Connector interface.
type mockConnector struct {
	connector.Connector // Embed interface for forward compatibility
	ConnectFunc         func(ctx context.Context, cfg connector.ConnectionCfg) error
}

func (m *mockConnector) Connect(ctx context.Context, cfg connector.ConnectionCfg) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx, cfg)
	}
	return nil
}

// mockRunner is a mock for the runner.Runner interface.
type mockRunner struct {
	runner.Runner
	GatherFactsFunc func(ctx context.Context, conn connector.Connector) (*runner.Facts, error)
}

func (m *mockRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) {
	if m.GatherFactsFunc != nil {
		return m.GatherFactsFunc(ctx, conn)
	}
	return &runner.Facts{Hostname: "default-mock-hostname", OS: &connector.OS{ID: "linux", Arch: "amd64"}}, nil
}

// mockConnectorFactory is a mock for the connector.Factory interface.
type mockConnectorFactory struct {
	connector.Factory
	NewSSHConnectorFunc   func(pool *connector.ConnectionPool) connector.Connector
	NewLocalConnectorFunc func() connector.Connector
}

func (m *mockConnectorFactory) NewSSHConnector(pool *connector.ConnectionPool) connector.Connector {
	if m.NewSSHConnectorFunc != nil {
		return m.NewSSHConnectorFunc(pool)
	}
	return &mockConnector{}
}
func (m *mockConnectorFactory) NewLocalConnector() connector.Connector {
	if m.NewLocalConnectorFunc != nil {
		return m.NewLocalConnectorFunc()
	}
	return &mockConnector{}
}

// --- Test Functions ---

func TestRuntimeBuilder_Build_HostInitialization(t *testing.T) {
	// --- 1. Test Setup ---
	clusterName := "test-cluster-hosts"
	node1Spec := v1alpha1.HostSpec{Name: "node1", Address: "10.0.0.1", Type: "ssh", User: "testuser", Port: 22, Roles: []string{"worker"}}
	cfg := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []v1alpha1.HostSpec{node1Spec},
		},
	}
	v1alpha1.SetDefaults_Cluster(cfg)

	// --- 2. Create and Configure Mock Dependencies ---

	mockRunnerSvc := &mockRunner{
		GatherFactsFunc: func(ctx context.Context, conn connector.Connector) (*runner.Facts, error) {
			return &runner.Facts{Hostname: "facts-hostname", OS: &connector.OS{ID: "mockOS"}}, nil
		},
	}

	mockPool := connector.NewConnectionPool(connector.DefaultPoolConfig()) // A real pool is fine

	mockFactory := &mockConnectorFactory{
		NewSSHConnectorFunc: func(pool *connector.ConnectionPool) connector.Connector {
			// This will be called for "node1"
			return &mockConnector{
				ConnectFunc: func(ctx context.Context, connCfg connector.ConnectionCfg) error {
					assert.Equal(t, "10.0.0.1", connCfg.Host)
					return nil
				},
			}
		},
		NewLocalConnectorFunc: func() connector.Connector {
			// This will be called for the control-node
			return &connector.LocalConnector{} // Use a real LocalConnector as it's simple
		},
	}

	mockEng := &mockEngine{}

	// --- 3. Create the Builder with Injected Mocks ---
	builder := NewRuntimeBuilderFromConfig(cfg, mockRunnerSvc, mockPool, mockFactory)

	// --- 4. Execute the method under test ---
	rtCtx, cleanup, err := builder.Build(context.Background(), mockEng)

	// --- 5. Assertions ---
	require.NoError(t, err)
	require.NotNil(t, rtCtx)
	defer cleanup()

	// Assert control node initialization
	assert.NotNil(t, rtCtx.controlNode, "ControlNode should be initialized")
	require.Contains(t, rtCtx.hostInfoMap, common.ControlNodeHostName)
	controlNodeInfo := rtCtx.hostInfoMap[common.ControlNodeHostName]
	_, ok := controlNodeInfo.Conn.(*connector.LocalConnector)
	assert.True(t, ok, "Control node should use LocalConnector created by factory")
	assert.NotNil(t, controlNodeInfo.Facts)

	// Assert node1 initialization
	require.Contains(t, rtCtx.hostInfoMap, "node1")
	node1Info := rtCtx.hostInfoMap["node1"]
	_, ok = node1Info.Conn.(*mockConnector)
	assert.True(t, ok, "node1 should use the mocked SSH connector from factory")
	assert.NotNil(t, node1Info.Facts)
	assert.Equal(t, "facts-hostname", node1Info.Facts.Hostname)
}

func TestRuntimeBuilder_Build_Directories(t *testing.T) {
	// --- 1. Test Setup ---
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	tmpCwd, err := os.MkdirTemp("", "kubexm-builder-test-cwd")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpCwd))
	defer func() {
		os.Chdir(originalWd)
		os.RemoveAll(tmpCwd)
	}()

	clusterName := "test-cluster-dirs"
	cfg := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []v1alpha1.HostSpec{
				{Name: "node1", Address: "10.0.0.1", Type: "ssh", Roles: []string{"worker"}},
			},
		},
	}
	v1alpha1.SetDefaults_Cluster(cfg)

	// --- 2. Create Mocks ---
	mockRunnerSvc := &mockRunner{}
	mockPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
	mockFactory := &mockConnectorFactory{
		NewSSHConnectorFunc:   func(pool *connector.ConnectionPool) connector.Connector { return &mockConnector{} },
		NewLocalConnectorFunc: func() connector.Connector { return &connector.LocalConnector{} },
	}
	mockEng := &mockEngine{}

	// --- 3. Create Builder ---
	builder := NewRuntimeBuilderFromConfig(cfg, mockRunnerSvc, mockPool, mockFactory)

	// --- 4. Execute ---
	rtCtx, cleanup, err := builder.Build(context.Background(), mockEng)

	// --- 5. Assertions ---
	require.NoError(t, err)
	require.NotNil(t, rtCtx)
	defer cleanup()

	expectedGlobalWorkDir := filepath.Join(tmpCwd, common.KUBEXM, clusterName)
	assert.Equal(t, expectedGlobalWorkDir, rtCtx.GlobalWorkDir)

	// Check directories
	dirsToTest := []string{
		rtCtx.GlobalWorkDir,
		rtCtx.GetHostDir(common.ControlNodeHostName),
		rtCtx.GetHostDir("node1"),
		rtCtx.GetCertsDir(),
		rtCtx.GetEtcdCertsDir(),
		rtCtx.GetEtcdArtifactsDir(),
		rtCtx.GetContainerRuntimeArtifactsDir(),
		rtCtx.GetKubernetesArtifactsDir(),
		filepath.Join(rtCtx.GlobalWorkDir, common.DefaultLogsDir),
	}

	for _, dirPath := range dirsToTest {
		_, statErr := os.Stat(dirPath)
		assert.NoError(t, statErr, fmt.Sprintf("Directory should be created: %s", dirPath))
	}
}
