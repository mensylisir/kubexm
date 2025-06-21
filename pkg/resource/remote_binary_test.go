package resource

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// mockTaskContextForResource provides a mock runtime.TaskContext.
type mockTaskContextForResource struct {
	runtime.TaskContext // Embed for forward compatibility
	logger              *logger.Logger
	goCtx               context.Context
	controlHost         connector.Host
	clusterCfg          *v1alpha1.Cluster
	globalWorkDir       string
	runner              *mockRunnerForResource // For Exists check in EnsurePlan pre-check
}

type mockRunnerForResource struct {
	runner.Runner
	ExistsFunc func(ctx context.Context, conn connector.Connector, path string) (bool, error)
}

func (m *mockRunnerForResource) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, nil // Default to not exists
}


func newMockTaskContextForResource(t *testing.T, globalWD string) *mockTaskContextForResource {
	l, _ := logger.New(logger.DefaultConfig())
	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockTaskContextForResource{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
		clusterCfg: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "resource-test-cluster"},
			Spec:       v1alpha1.ClusterSpec{Hosts: []*v1alpha1.Host{&ctrlHostSpec}},
		},
		globalWorkDir: globalWD,
		runner:        &mockRunnerForResource{},
	}
}
func (m *mockTaskContextForResource) GetLogger() *logger.Logger          { return m.logger }
func (m *mockTaskContextForResource) GoContext() context.Context            { return m.goCtx }
func (m *mockTaskContextForResource) GetControlNode() (connector.Host, error) { return m.controlHost, nil }
func (m *mockTaskContextForResource) GetClusterConfig() *v1alpha1.Cluster { return m.clusterCfg }
func (m *mockTaskContextForResource) GetGlobalWorkDir() string                  { return m.globalWorkDir }
func (m *mockTaskContextForResource) GetRunner() runner.Runner { return m.runner}
func (m *mockTaskContextForResource) GetConnectorForHost(h connector.Host) (connector.Connector, error) {
	if h.GetName() == common.ControlNodeHostName {
		return &connector.LocalConnector{}, nil
	}
	return nil, fmt.Errorf("no connector for host %s in mock", h.GetName())
}
func (m *mockTaskContextForResource) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if host.GetName() == common.ControlNodeHostName {
		return &runner.Facts{OS: &connector.OS{Arch: "amd64", ID: "linux"}}, nil // Mock facts for control node
	}
	return nil, fmt.Errorf("no facts for host %s in mock", host.GetName())
}
// Implement path helpers by delegating to a real Context instance (or re-implementing logic)
func (m *mockTaskContextForResource) GetFileDownloadPath(componentName, version, arch, filename string) string {
	baseCtx := &runtime.Context{GlobalWorkDir: m.globalWorkDir, ClusterConfig: m.clusterCfg}
	baseCtx.ClusterArtifactsDir = filepath.Join(m.globalWorkDir, m.clusterCfg.Name) // Set as builder would
	return baseCtx.GetFileDownloadPath(componentName, version, arch, filename)
}
// Other TaskContext methods
func (m *mockTaskContextForResource) PipelineCache() runtime.PipelineCache                 { return nil }
func (m *mockTaskContextForResource) ModuleCache() runtime.ModuleCache                     { return nil }
func (m *mockTaskContextForResource) TaskCache() runtime.TaskCache                       { return nil }
func (m *mockTaskContextForResource) GetHostsByRole(role string) ([]connector.Host, error) {
	if role == common.ControlNodeRole { return []connector.Host{m.controlHost}, nil }
	return nil, nil
}


func TestRemoteBinaryHandle_NewRemoteBinaryHandle_Success(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd")
	handle, err := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "amd64", "linux",
		"http://example.com/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		"etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		"etcd-{{.Version}}-{{.OS}}-{{.Arch}}/etcd",
		"sha256:abcdef", "")
	require.NoError(t, err)
	require.NotNil(t, handle)

	rbh, ok := handle.(*RemoteBinaryHandle)
	require.True(t, ok)
	assert.Equal(t, "etcd", rbh.ComponentName)
	assert.Equal(t, "v3.5.0", rbh.Version)
	assert.Equal(t, "amd64", rbh.Arch)
	assert.Equal(t, "linux", rbh.OS)
	assert.Equal(t, "http://example.com/etcd-v3.5.0-linux-amd64.tar.gz", rbh.DownloadURL)
	assert.Equal(t, "etcd-v3.5.0-linux-amd64.tar.gz", rbh.ArchiveFilename)
	assert.Equal(t, "etcd-v3.5.0-linux-amd64/etcd", rbh.BinaryPathInArchive)
	assert.Equal(t, "sha256:abcdef", rbh.ExpectedChecksum) // Checksum is stored as is
	assert.Equal(t, "sha256", rbh.ChecksumAlgorithm)      // Defaulted
}

func TestRemoteBinaryHandle_NewRemoteBinaryHandle_ArchResolution(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd")
	// Arch is empty, should be resolved from mockCtx.GetHostFacts(controlNode) -> "amd64"
	handle, err := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "", "linux", // Arch is ""
		"http://example.com/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		"etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		"etcd-{{.Version}}-{{.OS}}-{{.Arch}}/etcd",
		"", "")
	require.NoError(t, err)
	rbh := handle.(*RemoteBinaryHandle)
	assert.Equal(t, "amd64", rbh.Arch)
	assert.Equal(t, "http://example.com/etcd-v3.5.0-linux-amd64.tar.gz", rbh.DownloadURL)
	assert.Equal(t, "etcd-v3.5.0-linux-amd64.tar.gz", rbh.ArchiveFilename)
}

func TestRemoteBinaryHandle_ID(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd")
	handle, _ := NewRemoteBinaryHandle(mockCtx, "my-comp", "1.0", "arm64", "darwin", "url", "archive.zip", "bin/mybin", "", "")
	expectedID := "my-comp-binary-1.0-darwin-arm64-archive.zip"
	assert.Equal(t, expectedID, handle.ID())
}

func TestRemoteBinaryHandle_Path(t *testing.T) {
	workDir := "/testworkdir" // Mock global work dir
	mockCtx := newMockTaskContextForResource(t, workDir)

	handle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "amd64", "linux", "url", "etcd.tar.gz", "etcd-v3.5.0-linux-amd64/etcd", "", "")

	expectedPath := filepath.Join(workDir, mockCtx.GetClusterConfig().Name, "etcd", "v3.5.0", "amd64", "etcd")
	actualPath, err := handle.Path(mockCtx)
	require.NoError(t, err)
	assert.Equal(t, expectedPath, actualPath)
}

func TestRemoteBinaryHandle_EnsurePlan_Success(t *testing.T) {
	workDir := "/tmp/ensureplanwd"
	mockCtx := newMockTaskContextForResource(t, workDir)
	os.MkdirAll(filepath.Join(workDir, mockCtx.GetClusterConfig().Name), 0755) // Ensure base cluster dir exists for GetFileDownloadPath
	defer os.RemoveAll(workDir)

	handle, _ := NewRemoteBinaryHandle(mockCtx, "tool", "1.2.3", "amd64", "linux",
		"http://site.com/tool.tar.gz", "tool-v1.2.3.tar.gz", "tool-1.2.3/bin/tool", "sha256:checksum123", "")

	// Ensure final binary path does not exist for full plan generation
	mockCtx.runner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		finalBinPath, _ := handle.Path(mockCtx)
		if path == finalBinPath { return false, nil }
		return false, nil
	}

	fragment, err := handle.EnsurePlan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	assert.Len(t, fragment.Nodes, 3, "Should plan 3 steps: download, extract, finalize")

	// Check download node
	downloadNodeID := plan.NodeID("download-" + handle.ID())
	downloadNode, ok := fragment.Nodes[downloadNodeID]
	require.True(t, ok, "Download node not found")
	assert.Equal(t, fmt.Sprintf("Download tool archive (1.2.3 linux amd64)"), downloadNode.Name)
	downloadStep, ok := downloadNode.Step.(*commonsteps.DownloadFileStep)
	require.True(t, ok)
	assert.Equal(t, "http://site.com/tool.tar.gz", downloadStep.URL)
	expectedArchivePath := mockCtx.GetFileDownloadPath("tool", "1.2.3", "amd64", "tool-v1.2.3.tar.gz")
	assert.Equal(t, filepath.Dir(expectedArchivePath), downloadStep.DestDir) // DestDir is parent
	assert.Equal(t, "tool-v1.2.3.tar.gz", downloadStep.DestFilename)
	assert.Equal(t, "sha256:checksum123", downloadStep.Checksum)

	// Check extract node
	extractNodeID := plan.NodeID("extract-" + handle.ID())
	extractNode, ok := fragment.Nodes[extractNodeID]
	require.True(t, ok)
	extractStep, ok := extractNode.Step.(*commonsteps.ExtractArchiveStep)
	require.True(t, ok)
	assert.Equal(t, expectedArchivePath, extractStep.SourceArchivePath)
	expectedExtractDir := filepath.Join(filepath.Dir(expectedArchivePath), "extracted_tool-v1.2.3")
	assert.Equal(t, expectedExtractDir, extractStep.DestinationDir)
	assert.Contains(t, extractNode.Dependencies, downloadNodeID)

	// Check finalize node
	finalizeNodeID := plan.NodeID("finalize-" + handle.ID())
	finalizeNode, ok := fragment.Nodes[finalizeNodeID]
	require.True(t, ok)
	finalizeCmdStep, ok := finalizeNode.Step.(*commonsteps.CommandStep)
	require.True(t, ok)
	finalBinPath, _ := handle.Path(mockCtx)
	expectedCmd := fmt.Sprintf("mkdir -p %s && cp -f %s %s && chmod +x %s",
		filepath.Dir(finalBinPath),
		filepath.Join(expectedExtractDir, "tool-1.2.3/bin/tool"),
		finalBinPath,
		finalBinPath,
	)
	assert.Equal(t, expectedCmd, finalizeCmdStep.Cmd)
	assert.Contains(t, finalizeNode.Dependencies, extractNodeID)

	assert.ElementsMatch(t, []plan.NodeID{downloadNodeID}, fragment.EntryNodes)
	assert.ElementsMatch(t, []plan.NodeID{finalizeNodeID}, fragment.ExitNodes)
}

func TestRemoteBinaryHandle_EnsurePlan_BinaryAlreadyExists(t *testing.T) {
	workDir := "/tmp/ensureplanwd_exists"
	mockCtx := newMockTaskContextForResource(t, workDir)
	os.MkdirAll(filepath.Join(workDir, mockCtx.GetClusterConfig().Name), 0755)
	defer os.RemoveAll(workDir)

	handle, _ := NewRemoteBinaryHandle(mockCtx, "tool", "1.2.3", "amd64", "linux", "url", "archive.tar.gz", "bin/tool", "", "")

	// Simulate final binary path exists
	mockCtx.runner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		finalBinPath, _ := handle.Path(mockCtx)
		if path == finalBinPath { return true, nil } // Mark as existing
		return false, nil
	}

	fragment, err := handle.EnsurePlan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	assert.Empty(t, fragment.Nodes, "Plan should be empty if binary already exists")
}
