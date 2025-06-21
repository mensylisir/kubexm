package resource

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"io/ioutil"
	"os"


	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// mockTaskContextForResource provides a mock runtime.TaskContext.
type mockTaskContextForResource struct {
	runtime.TaskContext
	logger         *logger.Logger
	goCtx          context.Context
	clusterConfig  *v1alpha1.Cluster
	workDir        string
	controlHost    connector.Host
	mockStepOutput map[string]interface{} // To simulate outputs from steps like download path
}

func newMockTaskContextForResource(clusterName string) *mockTaskContextForResource {
	l, _ := logger.New(logger.DefaultConfig())
	tempWorkDir, _ := ioutil.TempDir("", "res-workdir-")

	// Create a mock control host
	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)


	return &mockTaskContextForResource{
		logger: l,
		goCtx:  context.Background(),
		clusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: clusterName},
			Spec:       v1alpha1.ClusterSpec{},
		},
		workDir: tempWorkDir,
		controlHost: ctrlHost,
		mockStepOutput: make(map[string]interface{}),
	}
}

// Implement runtime.TaskContext interface
func (m *mockTaskContextForResource) GetLogger() *logger.Logger          { return m.logger }
func (m *mockTaskContextForResource) GoContext() context.Context            { return m.goCtx }
func (m *mockTaskContextForResource) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockTaskContextForResource) GetWorkDir() string                  { return m.workDir } // This is $(pwd) effectively

func (m *mockTaskContextForResource) GetControlNode() (connector.Host, error) {
	if m.controlHost == nil {
		return nil, fmt.Errorf("control host not set in mock context")
	}
	return m.controlHost, nil
}

// --- Mock other TaskContext methods as needed, returning defaults/nils ---
func (m *mockTaskContextForResource) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockTaskContextForResource) GetHostFacts(host connector.Host) (*runtime.Facts, error) { return nil, nil }
func (m *mockTaskContextForResource) PipelineCache() runtime.PipelineCache { return nil }
func (m *mockTaskContextForResource) ModuleCache() runtime.ModuleCache   { return nil }
func (m *mockTaskContextForResource) TaskCache() runtime.TaskCache     { return nil }


func TestRemoteBinaryArchiveHandle_ID(t *testing.T) {
	handle := NewRemoteBinaryArchiveHandle("etcd", "v3.5.9", "amd64", "url", "fname", nil, "chk").(*RemoteBinaryArchiveHandle)
	assert.Equal(t, "remote-archive-etcd-v3.5.9-amd64", handle.ID())

	handleNoArch := NewRemoteBinaryArchiveHandle("etcd", "v3.5.9", "", "url", "fname", nil, "chk").(*RemoteBinaryArchiveHandle)
	assert.Equal(t, "remote-archive-etcd-v3.5.9-auto", handleNoArch.ID())
}

func TestRemoteBinaryArchiveHandle_PathCalculations(t *testing.T) {
	ctx := newMockTaskContextForResource("mycluster")
	defer os.RemoveAll(ctx.workDir)

	handle := NewRemoteBinaryArchiveHandle(
		"etcd", "3.5.9", "amd64", // Version without 'v' as per previous task refactor
		"http://example.com/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		"etcd-archive-{{.Version}}-{{.Arch}}.tar.gz",
		map[string]string{
			"etcd":    "etcd-{{.Version}}-linux-{{.Arch}}/etcd",
			"etcdctl": "etcd-{{.Version}}-linux-{{.Arch}}/etcdctl",
		},
		"checksum123",
	).(*RemoteBinaryArchiveHandle)

	expectedArch := "amd64"
	// $(pwd)/res-workdir-XXXX/.kubexm/mycluster/etcd/3.5.9/amd64/etcd-archive-3.5.9-amd64.tar.gz
	archivePath, err := handle.archiveDownloadPath(ctx)
	require.NoError(t, err)
	expectedArchiveName := fmt.Sprintf("etcd-archive-%s-%s.tar.gz", handle.Version, expectedArch)
	expectedArchivePathSuffix := filepath.Join(common.DefaultWorkDirName, "mycluster", "etcd", handle.Version, expectedArch, expectedArchiveName)
	assert.True(t, strings.HasSuffix(archivePath, expectedArchivePathSuffix), "archive path %s should end with %s", archivePath, expectedArchivePathSuffix)


	// $(pwd)/res-workdir-XXXX/.kubexm/mycluster/etcd/3.5.9/amd64/extracted
	extractedRoot, err := handle.extractedArchiveRootPath(ctx)
	require.NoError(t, err)
	expectedExtractedRootSuffix := filepath.Join(common.DefaultWorkDirName, "mycluster", "etcd", handle.Version, expectedArch, "extracted")
	assert.True(t, strings.HasSuffix(extractedRoot, expectedExtractedRootSuffix))

	// $(pwd)/res-workdir-XXXX/.kubexm/mycluster/etcd/3.5.9/amd64/extracted/etcd-3.5.9-linux-amd64/etcd
	etcdBinaryPath, err := handle.Path("etcd", ctx) // Using the specific binary path method
	require.NoError(t, err)
	expectedEtcdRelPath := fmt.Sprintf("etcd-%s-linux-%s/etcd", handle.Version, expectedArch)
	expectedEtcdPathSuffix := filepath.Join(expectedExtractedRootSuffix, expectedEtcdRelPath)
	assert.True(t, strings.HasSuffix(etcdBinaryPath, expectedEtcdPathSuffix))

	// Test Path() from Handle interface (should be extracted root)
	interfacePath := handle.Path(ctx)
	assert.Equal(t, extractedRoot, interfacePath)
}


func TestRemoteBinaryArchiveHandle_EnsurePlan(t *testing.T) {
	ctx := newMockTaskContextForResource("plancluster")
	defer os.RemoveAll(ctx.workDir)

	controlHost, _ := ctx.GetControlNode()

	handle := NewRemoteBinaryArchiveHandle(
		"mycomp", "1.2.3", "amd64",
		"http://cache.example.com/mycomp-{{.Version}}-{{.Arch}}.tar.gz",
		"mycomp-{{.Version}}-{{.Arch}}.tar.gz",
		map[string]string{"mybinary": "mycomp-{{.Version}}-{{.Arch}}/mybinary"},
		"sha256checksum",
	).(*RemoteBinaryArchiveHandle)

	// --- Simulate that the resource is NOT present ---
	// (Precheck logic in RemoteBinaryArchiveHandle.EnsurePlan is currently basic and relies on steps' prechecks)

	fragment, err := handle.EnsurePlan(ctx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	require.Len(t, fragment.Nodes, 2, "EnsurePlan should generate 2 nodes (download + extract)")

	// Node 1: Download
	downloadNodeID := plan.NodeID("Download-mycomp-1.2.3-amd64")
	downloadNode, ok := fragment.Nodes[downloadNodeID]
	require.True(t, ok, "Download node not found")
	assert.Equal(t, []connector.Host{controlHost}, downloadNode.Hosts)
	assert.Empty(t, downloadNode.Dependencies)
	require.IsType(t, &commonsteps.DownloadFileStep{}, downloadNode.Step)
	downloadStep := downloadNode.Step.(*commonsteps.DownloadFileStep)
	assert.Equal(t, "http://cache.example.com/mycomp-1.2.3-amd64.tar.gz", downloadStep.URL)
	expectedDownloadPath, _ := handle.archiveDownloadPath(ctx)
	assert.Equal(t, expectedDownloadPath, downloadStep.DestPath)
	assert.Equal(t, "sha256checksum", downloadStep.Checksum)

	// Node 2: Extract
	extractNodeID := plan.NodeID("Extract-mycomp-1.2.3-amd64")
	extractNode, ok := fragment.Nodes[extractNodeID]
	require.True(t, ok, "Extract node not found")
	assert.Equal(t, []connector.Host{controlHost}, extractNode.Hosts)
	require.Len(t, extractNode.Dependencies, 1)
	assert.Equal(t, downloadNodeID, extractNode.Dependencies[0])
	require.IsType(t, &commonsteps.ExtractArchiveStep{}, extractNode.Step)
	extractStep := extractNode.Step.(*commonsteps.ExtractArchiveStep)
	assert.Equal(t, expectedDownloadPath, extractStep.SourceArchivePath) // Source is the downloaded archive
	expectedExtractPath, _ := handle.extractedArchiveRootPath(ctx)
	assert.Equal(t, expectedExtractPath, extractStep.DestinationDir)

	// Check Entry and Exit nodes
	require.ElementsMatch(t, []plan.NodeID{downloadNodeID}, fragment.EntryNodes)
	require.ElementsMatch(t, []plan.NodeID{extractNodeID}, fragment.ExitNodes)


	// --- Simulate that the resource IS present (by making precheck of steps return true) ---
	// This part is harder to test at the handle level without actually executing steps or deeply mocking them.
	// The current EnsurePlan relies on the steps' Precheck methods.
	// If we wanted to test the handle's own precheck logic (the commented out allBinariesExist part),
	// we would need to:
	// 1. Create the expected final binary file(s) on disk.
	// 2. Implement a way for the handle's precheck to use a local runner/os.Stat on those files.
	// This is more of an integration test for the handle's precheck.
	// For now, the unit test focuses on the plan generation logic.
}

// TODO: Add test for GetEffectiveArch if it becomes more complex (e.g., reads from context/facts)
// TODO: Add test for URL/ArchiveFileName rendering errors
// TODO: Add test for Path (binary key) not found in BinariesInArchive
