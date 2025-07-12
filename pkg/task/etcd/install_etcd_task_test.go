package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common" // Alias to avoid clash with package 'common'
	stepetcd "github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

// TestInstallETCDTask_Plan_BinaryInstall_SingleNode tests the Plan method for a single Etcd node, binary install.
func TestInstallETCDTask_Plan_BinaryInstall_SingleNode(t *testing.T) {
	// --- Test Setup ---
	mockTaskCtx := new(taskmocks.MockTaskContext)
	mockLogger := new(loggermocks.MockLogger)
	mockCtrlNode := new(connectormocks.MockHost)
	mockEtcdNode1 := new(connectormocks.MockHost)
	mockResourceHandle := new(resourcemocks.MockHandle)

	// Cluster Configuration
	etcdVersion := "v3.5.13"
	clusterName := "test-cluster"
	etcdNode1Name := "etcd-node-1"
	etcdNode1IP := "192.168.1.10"

	clusterConfig := &v1alpha1.Cluster{
		ObjectMeta: v1alpha1.ObjectMeta{Name: clusterName},
		Spec: v1alpha1.ClusterSpec{
			Etcd: &v1alpha1.EtcdConfig{
				Type:    v1alpha1.EtcdTypeKubeXM,
				Version: etcdVersion,
				Arch:    "amd64",
				DataDir: "/var/lib/kubexm/etcd", // Using the getter GetDatadir() in real code
				ClusterToken: "test-etcd-token",
				ClientPort: func(i int) *int { return &i }(2379),
				PeerPort:   func(i int) *int { return &i }(2380),
			},
		},
	}
	// Apply defaults to EtcdConfig part for getters
	v1alpha1.SetDefaults_EtcdConfig(clusterConfig.Spec.Etcd)


	// Mock Host behaviors
	mockCtrlNode.On("GetName").Return(common.ControlNodeHostName)
	mockEtcdNode1.On("GetName").Return(etcdNode1Name)
	mockEtcdNode1.On("GetAddress").Return(etcdNode1IP)
	mockEtcdNode1.On("GetArch").Return("amd64") // Assuming etcd node arch

	// Mock TaskContext methods
	mockTaskCtx.On("GetLogger").Return(mockLogger)
	mockLogger.On("With", mock.Anything, mock.Anything).Return(mockLogger)
	mockLogger.On("Info", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockTaskCtx.On("GetClusterConfig").Return(clusterConfig)
	mockTaskCtx.On("GetHostsByRole", v1alpha1.ETCDRole).Return([]connector.Host{mockEtcdNode1}, nil)
	mockTaskCtx.On("GetHostsByRole", v1alpha1.MasterRole).Return([]connector.Host{}, nil) // No masters for this simple test
	mockTaskCtx.On("GetControlNode").Return(mockCtrlNode, nil)
	mockTaskCtx.On("GetEtcdCertsDir").Return(filepath.Join("/tmp/kubexm-test", clusterName, "pki", "etcd"))


	// Mock Resource Handle for Etcd Archive
	localEtcdArchivePath := filepath.Join("/tmp/kubexm-test", clusterName, "etcd", etcdVersion, "amd64", "etcd-"+etcdVersion+"-linux-amd64.tar.gz")
	resourcePrepFragment := task.NewExecutionFragment("EtcdArchivePrep")
	downloadNodeID := plan.NodeID("download-etcd-archive-resource")
	resourcePrepFragment.AddNode(&plan.ExecutionNode{ // Simplified node for testing
		Name: "DownloadEtcdArchive", Step: &step.NoOpStep{}, Hosts: []connector.Host{mockCtrlNode},
	}, downloadNodeID)
	resourcePrepFragment.EntryNodes = []plan.NodeID{downloadNodeID}
	resourcePrepFragment.ExitNodes = []plan.NodeID{downloadNodeID}

	// This mock setup is simplified. A real test would involve:
	// 1. Mocking resource.NewRemoteBinaryHandle to return our mockResourceHandle.
	// 2. Mocking mockResourceHandle.EnsurePlan() and mockResourceHandle.Path().
	// For now, we'll assume NewRemoteBinaryHandle works and mock what its EnsurePlan would produce.
	// We can't directly mock NewRemoteBinaryHandle without DI or interface for its constructor.
	// So, this test will be more of an integration test for the Plan method's logic flow.
	// To truly unit test, NewRemoteBinaryHandle would need to be injectable.
	// Let's assume this test is verifying the orchestration logic *given* a resource handle.
	// For now, this part of the test is more conceptual due to constructor call.
	// The actual `resource.NewRemoteBinaryHandle` will be called. We'll mock its output.
	// This means we need a way to inject the mockResourceHandle or test the real one.
	// Let's assume the real resource.NewRemoteBinaryHandle is used, and its EnsurePlan returns a known simple fragment.
	// This makes the test an integration test of InstallEtcdTask with resource.RemoteBinaryHandle.
	// To make it a unit test, resource.Handle creation would need to be injectable.

	// Given the current structure, we can't easily mock NewRemoteBinaryHandle.
	// So, this test will rely on the real NewRemoteBinaryHandle and its EnsurePlan.
	// We will focus on verifying the structure of the fragment *after* resource prep.

	// --- Create and Run Task ---
	etcdTask := NewInstallETCDTask()
	mainFragment, err := etcdTask.Plan(mockTaskCtx)

	// --- Assertions ---
	require.NoError(t, err)
	require.NotNil(t, mainFragment)
	assert.NotEmpty(t, mainFragment.Nodes)

	// Verify resource prep node (download) is present (from real resource.NewRemoteBinaryHandle)
	// The exact ID will be generated by the handle. We check for a step type.
	foundDownloadEtcdArchive := false
	var downloadEtcdArchiveNodeID plan.NodeID
	for id, node := range mainFragment.Nodes {
		if strings.HasPrefix(string(id), "download-etcd-"+etcdVersion) {
			if _, ok := node.Step.(*commonsteps.DownloadFileStep); ok {
				foundDownloadEtcdArchive = true
				downloadEtcdArchiveNodeID = id
				assert.Equal(t, []connector.Host{mockCtrlNode}, node.Hosts)
				break
			}
		}
	}
	assert.True(t, foundDownloadEtcdArchive, "Expected a DownloadFileStep for etcd archive")
	require.NotEmpty(t, downloadEtcdArchiveNodeID, "Download Etcd Archive Node ID should not be empty")


	// Verify PKI distribution nodes (at least CA upload for etcd node)
	expectedCaUploadNodeID := plan.NodeID(fmt.Sprintf("upload-pki-%s-upload-ca_pem", strings.ReplaceAll(etcdNode1Name, ".", "-")))
	caUploadNode, ok := mainFragment.Nodes[expectedCaUploadNodeID]
	require.True(t, ok, "CA upload node for etcd node not found")
	assert.Equal(t, []connector.Host{mockEtcdNode1}, caUploadNode.Hosts)
	if downloadEtcdArchiveNodeID != "" { // If resource prep happened
		assert.Contains(t, caUploadNode.Dependencies, downloadEtcdArchiveNodeID, "CA upload should depend on archive download/prep")
	}

	// Verify per-node installation steps for etcdNode1
	nodePrefix := fmt.Sprintf("etcd-%s-", strings.ReplaceAll(etcdNode1Name, ".", "-"))

	uploadArchiveNodeID := plan.NodeID(nodePrefix + "upload-archive")
	uploadArchiveNode, ok := mainFragment.Nodes[uploadArchiveNodeID]
	require.True(t, ok, "Upload archive node for etcd node not found")
	assert.Equal(t, []connector.Host{mockEtcdNode1}, uploadArchiveNode.Hosts)
	assert.Contains(t, uploadArchiveNode.Dependencies, downloadEtcdArchiveNodeID) // Depends on local archive ready

	extractArchiveNodeID := plan.NodeID(nodePrefix + "extract-archive")
	extractArchiveNode, ok := mainFragment.Nodes[extractArchiveNodeID]
	require.True(t, ok, "Extract archive node for etcd node not found")
	assert.Contains(t, extractArchiveNode.Dependencies, uploadArchiveNodeID)

	copyEtcdNodeID := plan.NodeID(nodePrefix + "copy-etcd")
	copyEtcdNode, ok := mainFragment.Nodes[copyEtcdNodeID]
	require.True(t, ok, "Copy etcd binary node not found")
	assert.Contains(t, copyEtcdNode.Dependencies, extractArchiveNodeID)

	copyEtcdctlNodeID := plan.NodeID(nodePrefix + "copy-etcdctl")
	copyEtcdctlNode, ok := mainFragment.Nodes[copyEtcdctlNodeID]
	require.True(t, ok, "Copy etcdctl binary node not found")
	assert.Contains(t, copyEtcdctlNode.Dependencies, extractArchiveNodeID) // Parallel to etcd copy

	generateConfigNodeID := plan.NodeID(nodePrefix + "generate-config")
	generateConfigNode, ok := mainFragment.Nodes[generateConfigNodeID]
	require.True(t, ok, "Generate etcd config node not found")
	assert.Contains(t, generateConfigNode.Dependencies, copyEtcdNodeID)
	assert.Contains(t, generateConfigNode.Dependencies, copyEtcdctlNodeID)
	// Also check dependency on this host's last PKI cert upload
	expectedPkiDepForConfigNode := plan.NodeID(fmt.Sprintf("upload-pki-%s-upload-peer-%s-key_pem", etcdNode1Name, strings.ReplaceAll(fmt.Sprintf("peer-%s-key.pem",etcdNode1Name),".","_")))
	assert.Contains(t, generateConfigNode.Dependencies, expectedPkiDepForConfigNode)


	bootstrapNodeID := plan.NodeID(nodePrefix + "bootstrap") // Since it's the first (and only) etcd node
	bootstrapNode, ok := mainFragment.Nodes[bootstrapNodeID]
	require.True(t, ok, "Bootstrap etcd node not found")

	// Check that bootstrap depends on daemon-reload, which depends on service gen, which depends on config gen
	daemonReloadNodeID := plan.NodeID(nodePrefix + "daemon-reload")
	assert.Contains(t, bootstrapNode.Dependencies, daemonReloadNodeID)
	generateServiceNodeID := plan.NodeID(nodePrefix + "generate-service")
	daemonReloadNode, _ := mainFragment.Nodes[daemonReloadNodeID]
	assert.Contains(t, daemonReloadNode.Dependencies, generateServiceNodeID)
	generateServiceNode, _ := mainFragment.Nodes[generateServiceNodeID]
	assert.Contains(t, generateServiceNode.Dependencies, generateConfigNodeID)


	// Check Entry and Exit nodes
	// Entry node should be the initial resource download node (or nodes if pki prep is parallel)
	assert.Contains(t, mainFragment.EntryNodes, downloadEtcdArchiveNodeID)

	// Exit node should be the bootstrap node for the single etcd node
	assert.Contains(t, mainFragment.ExitNodes, bootstrapNodeID)

	// Further checks:
	// - Correct step types used (e.g., common.UploadFileStep, common.ExtractArchiveStep, common.CommandStep, etcd.GenerateEtcdConfigStep etc.)
	// - Correct parameters passed to step constructors (e.g. paths, commands, sudo flags)
	// - Correct target hosts for each node

	// Example: Check step type for upload archive node
	require.IsType(t, &commonsteps.UploadFileStep{}, uploadArchiveNode.Step)
	uploadStep := uploadArchiveNode.Step.(*commonsteps.UploadFileStep)
	assert.Equal(t, localEtcdArchivePath, uploadStep.LocalSrcPath) // Verify source path is correct
	assert.True(t, uploadStep.Sudo) // Verify sudo for upload

	// Example: Check step type and command for copy etcd node
	require.IsType(t, &commonsteps.CommandStep{}, copyEtcdNode.Step)
	copyCmdStep := copyEtcdNode.Step.(*commonsteps.CommandStep)
	expectedCopyEtcdCmd := fmt.Sprintf("cp -fp %s/%s/etcd %s/etcd && chmod +x %s/etcd",
		"/opt/kubexm/etcd-extracted", // etcdExtractDirOnHost
		archiveInternalDirName,
		DefaultEtcdUsrLocalBin, DefaultEtcdUsrLocalBin,
	)
	assert.Equal(t, expectedCopyEtcdCmd, copyCmdStep.Cmd)
	assert.True(t, copyCmdStep.Sudo)


	// Verify EtcdNodeConfigData in GenerateEtcdConfigStep
	generateConfigStep, ok := generateConfigNode.Step.(*stepetcd.GenerateEtcdConfigStep)
	require.True(t, ok, "Generate config step is not of type GenerateEtcdConfigStep")
	assert.Equal(t, etcdNode1Name, generateConfigStep.ConfigData.Name)
	assert.Contains(t, generateConfigStep.ConfigData.ListenPeerURLs, etcdNode1IP)
	assert.Equal(t, "new", generateConfigStep.ConfigData.InitialClusterState)


	mockTaskCtx.AssertExpectations(t)
	// mockResourceHandle.AssertExpectations(t) // If we were able to mock it
}
