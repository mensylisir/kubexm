package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/util"
	// Mocking parser.ParseFromFile requires a bit more setup or an interface.
	// For this test, we can use BuildFromConfig and provide a config object directly.
)

// mockConnectorForBuilderTest is a simple mock.
type mockConnectorForBuilderTest struct {
	connector.Connector // Embed interface
	ConnectFunc func(ctx context.Context, cfg connector.ConnectionCfg) error
	GetOSFunc   func(ctx context.Context) (*connector.OS, error)
	CloseFunc   func() error
}

func (m *mockConnectorForBuilderTest) Connect(ctx context.Context, cfg connector.ConnectionCfg) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx, cfg)
	}
	return nil
}
func (m *mockConnectorForBuilderTest) GetOS(ctx context.Context) (*connector.OS, error) {
	if m.GetOSFunc != nil {
		return m.GetOSFunc(ctx)
	}
	return &connector.OS{ID: "linux", Arch: "amd64", PrettyName: "Mocked OS"}, nil
}
func (m *mockConnectorForBuilderTest) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
func (m *mockConnectorForBuilderTest) Exec(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil, nil, nil }
func (m *mockConnectorForBuilderTest) IsConnected() bool { return true }
func (m *mockConnectorForBuilderTest) CopyContent(ctx context.Context, content []byte, destPath string, options *connector.FileTransferOptions) error { return nil }
func (m *mockConnectorForBuilderTest) Stat(ctx context.Context, path string) (*connector.FileStat, error) { return nil, nil }
func (m *mockConnectorForBuilderTest) LookPath(ctx context.Context, file string) (string, error) { return "", nil }
func (m *mockConnectorForBuilderTest) ReadFile(ctx context.Context, path string) ([]byte, error) {return nil, nil}
func (m *mockConnectorForBuilderTest) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {return nil}


// mockRunnerForBuilderTest mocks the runner.Runner interface.
type mockRunnerForBuilderTest struct {
	runner.Runner // Embed for forward compatibility
	GatherFactsFunc func(ctx context.Context, conn connector.Connector) (*runner.Facts, error)
}
func (m *mockRunnerForBuilderTest) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) {
	if m.GatherFactsFunc != nil {
		return m.GatherFactsFunc(ctx, conn)
	}
	return &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64", PrettyName: "Mocked OS for Runner"}}, nil
}


func TestRuntimeBuilder_BuildFromConfig_Directories(t *testing.T) {
	// Setup: Create a temporary current working directory for the test
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
		ObjectMeta: v1alpha1.ObjectMeta{Name: clusterName},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []*v1alpha1.Host{
				{Name: "node1", Address: "10.0.0.1", Type: "ssh", Roles: []string{"worker"}},
			},
			// Global.WorkDir is intentionally left empty to test default path generation
		},
	}

	builder := &RuntimeBuilder{} // Not using NewRuntimeBuilder if we call BuildFromConfig directly

	// Override osReadFile for private key if any host needed it (not in this specific test case)
	// oldOsReadFile := osReadFile
	// osReadFile = func(name string) ([]byte, error) { return []byte("mock key data"), nil }
	// defer func() { osReadFile = oldOsReadFile }()

	// Mock connector behavior within the builder's initializeHost
	// This is tricky because initializeHost is internal.
	// A more common way is to mock the services (runner, connector factory) passed to builder,
	// or make builder accept them.
	// For now, we rely on LocalConnector for control-node and test directory creation.

	rtCtx, cleanup, err := builder.BuildFromConfig(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, rtCtx)
	defer cleanup()

	expectedGlobalWorkDir := filepath.Join(tmpCwd, common.KUBEXM)
	assert.Equal(t, expectedGlobalWorkDir, rtCtx.GlobalWorkDir)

	// Check GlobalWorkDir creation
	_, statErr := os.Stat(expectedGlobalWorkDir)
	assert.NoError(t, statErr, "GlobalWorkDir should be created")

	// Check HostWorkDir for control node
	controlNodeHostDir := filepath.Join(expectedGlobalWorkDir, common.ControlNodeHostName)
	_, statErr = os.Stat(controlNodeHostDir)
	assert.NoError(t, statErr, "ControlNodeHostDir should be created")

	// Check HostWorkDir for node1
	node1HostDir := filepath.Join(expectedGlobalWorkDir, "node1")
	_, statErr = os.Stat(node1HostDir)
	assert.NoError(t, statErr, "node1 HostDir should be created")


	// Check ClusterArtifactsDir
	expectedClusterArtifactsDir := filepath.Join(expectedGlobalWorkDir, clusterName)
	assert.Equal(t, expectedClusterArtifactsDir, rtCtx.ClusterArtifactsDir)
	_, statErr = os.Stat(expectedClusterArtifactsDir)
	assert.NoError(t, statErr, "ClusterArtifactsDir should be created")

	// Check sub-artifact directories
	dirsToTest := map[string]string{
		"logs":              filepath.Join(expectedClusterArtifactsDir, common.DefaultLogsDir),
		"certs_base":        filepath.Join(expectedClusterArtifactsDir, common.DefaultCertsDir),
		"etcd_certs":        filepath.Join(expectedClusterArtifactsDir, common.DefaultCertsDir, common.DefaultEtcdDir),
		"etcd_artifacts":    filepath.Join(expectedClusterArtifactsDir, common.DefaultEtcdDir),
		"crt_artifacts":     filepath.Join(expectedClusterArtifactsDir, common.DefaultContainerRuntimeDir),
		"k8s_artifacts":     filepath.Join(expectedClusterArtifactsDir, common.DefaultKubernetesDir),
	}

	for name, dirPath := range dirsToTest {
		_, statErr = os.Stat(dirPath)
		assert.NoError(t, statErr, fmt.Sprintf("%s directory should be created at %s", name, dirPath))
	}
}

func TestRuntimeBuilder_BuildFromConfig_HostInitialization(t *testing.T) {
	clusterName := "test-cluster-hosts"
	node1Spec := &v1alpha1.Host{Name: "node1", Address: "10.0.0.1", Type: "ssh", User: "testuser", Port: 22}
	cfg := &v1alpha1.Cluster{
		ObjectMeta: v1alpha1.ObjectMeta{Name: clusterName},
		Spec:       v1alpha1.ClusterSpec{Hosts: []*v1alpha1.Host{node1Spec}},
	}

	builder := &RuntimeBuilder{}

	// Store original functions to restore them later
    origNewSSHConnector := connector.NewSSHConnector
    origNewLocalConnector := func() connector.Connector { return &connector.LocalConnector{} } // Assuming LocalConnector is simple
    origRunnerNew := runner.New

	// Mock NewSSHConnector to return our mock connector
    connector.NewSSHConnector = func(pool *connector.ConnectionPool) connector.Connector {
        mockConn := &mockConnectorForBuilderTest{}
        mockConn.ConnectFunc = func(ctx context.Context, cfg connector.ConnectionCfg) error {
            // Assertions about cfg can be made here if needed
            assert.Equal(t, node1Spec.Address, cfg.Host)
            return nil
        }
        return mockConn
    }
    // We also need to control the LocalConnector for the control-node part of BuildFromConfig
    // This is harder if it's directly instantiated.
    // For now, let's assume LocalConnector works as is, or its interactions are minimal in this test's scope.

    // Mock runner.New to return our mock runner
    mockRun := &mockRunnerForBuilderTest{}
    mockRun.GatherFactsFunc = func(ctx context.Context, conn connector.Connector) (*runner.Facts, error) {
        return &runner.Facts{Hostname: conn.(*mockConnectorForBuilderTest).ConnectFunc // Hacky way to see which host this is for
			OS: &connector.OS{ID: "mocklinux", Arch: "amd64", PrettyName: "Mocked OS via Runner"},
		}, nil
    }
    runner.New = func() runner.Runner { return mockRun }


	rtCtx, cleanup, err := builder.BuildFromConfig(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, rtCtx)
	defer cleanup()

	// Restore original functions
    connector.NewSSHConnector = origNewSSHConnector
    runner.New = origRunnerNew


	assert.NotNil(t, rtCtx.ControlNode, "ControlNode should be initialized")
	assert.Equal(t, common.ControlNodeHostName, rtCtx.ControlNode.GetName())

	require.Contains(t, rtCtx.HostRuntimes, common.ControlNodeHostName)
	_, ok := rtCtx.HostRuntimes[common.ControlNodeHostName].Conn.(*connector.LocalConnector)
	assert.True(t, ok, "Control node should use LocalConnector")
	assert.NotNil(t, rtCtx.HostRuntimes[common.ControlNodeHostName].Facts)


	require.Contains(t, rtCtx.HostRuntimes, "node1")
	_, ok = rtCtx.HostRuntimes["node1"].Conn.(*mockConnectorForBuilderTest)
	assert.True(t, ok, "node1 should use the mocked SSH connector")
	assert.NotNil(t, rtCtx.HostRuntimes["node1"].Facts)
	// Facts for node1 would come from mockRun.GatherFactsFunc
	assert.Equal(t, "Mocked OS via Runner", rtCtx.HostRuntimes["node1"].Facts.OS.PrettyName)
}

// TODO: Test for BuildFromFile (requires mocking parser.ParseFromFile or using a temp config file)
// TODO: Test private key loading (mocking osReadFile)
// TODO: Test connection timeout propagation
// TODO: Test error handling during host initialization
// TODO: Test global config propagation (Verbose, IgnoreErr, etc.)

func TestUtilCreateDir(t *testing.T) { // Testing the utility function directly as an example
	tmpDir, err := os.MkdirTemp("", "test-create-dir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	newPath := filepath.Join(tmpDir, "a", "b", "c")
	err = util.CreateDir(newPath)
	require.NoError(t, err)

	_, statErr := os.Stat(newPath)
	assert.NoError(t, statErr)
}
