package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	// "github.com/mensylisir/kubexm/pkg/common" // No longer directly needed for consts like DefaultEtcdDir here
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Aliased
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/util"
)

// mockTaskContextForResource provides a mock runtime.TaskContext.
type mockTaskContextForResource struct {
	// Embed an unexported/unimplemented interface to satisfy the compiler if methods are added to runtime.TaskContext
	// runtime.taskContextInternal // Example of such embedding pattern
	logger        *logger.Logger
	goCtx         context.Context
	controlHost   connector.Host
	clusterCfg    *v1alpha1.Cluster
	globalWorkDir string // This should be the root work dir, e.g., /tmp
	runner        *mockRunnerForResource
	// Caches - can be nil if not used by the specific methods being tested
	pCache runtime.PipelineCache
	mCache runtime.ModuleCache
	tCache runtime.TaskCache
	sCache runtime.StepCache
}

type mockRunnerForResource struct {
	// runner.Runner // Embed full interface if some methods are not mocked but might be called
	ExistsFunc   func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	GetSHA256Func func(ctx context.Context, conn connector.Connector, path string) (string, error)
}

// Implement only methods needed by the tests or by the code paths being tested.
func (m *mockRunnerForResource) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, nil
}
func (m *mockRunnerForResource) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) {
	if m.GetSHA256Func != nil {
		return m.GetSHA256Func(ctx, conn, path)
	}
	return "", fmt.Errorf("GetSHA256 not mocked")
}

// Implement other runner.Runner methods as needed, returning error or default values.
func (m *mockRunnerForResource) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForResource) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForResource) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForResource) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForResource) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) { return nil, nil, nil }
func (m *mockRunnerForResource) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForResource) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForResource) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForResource) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForResource) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForResource) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForResource) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForResource) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForResource) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForResource) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForResource) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForResource) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForResource) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForResource) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForResource) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForResource) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForResource) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForResource) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForResource) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForResource) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForResource) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForResource) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForResource) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForResource) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForResource) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForResource) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForResource) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForResource) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForResource) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForResource) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForResource) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForResource) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }


func newMockTaskContextForResource(t *testing.T, globalWD, clusterN string) *mockTaskContextForResource {
	l, _ := logger.NewLogger(logger.DefaultOptions()) // Use NewLogger for instance
	// Ensure control host spec has Arch
	ctrlHostSpec := v1alpha1.HostSpec{Name: "control-node", Type: "local", Address: "127.0.0.1", Roles: []string{"control-node"}, Arch: "amd64"}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockTaskContextForResource{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
		clusterCfg: &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterN},
			Spec:       v1alpha1.ClusterSpec{Hosts: []v1alpha1.HostSpec{ctrlHostSpec}},
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
func (m *mockTaskContextForResource) GetRunner() runner.Runner                  { return m.runner }
func (m *mockTaskContextForResource) GetConnectorForHost(h connector.Host) (connector.Connector, error) {
	if h.GetName() == "control-node" { // Updated to use constant if available or direct string
		return &connector.LocalConnector{}, nil
	}
	return nil, fmt.Errorf("no connector for host %s in mock", h.GetName())
}
func (m *mockTaskContextForResource) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if host.GetName() == "control-node" {
		return &runner.Facts{OS: &connector.OS{Arch: "amd64", ID: "linux"}}, nil // Mock facts for control node
	}
	return nil, fmt.Errorf("no facts for host %s in mock", host.GetName())
}

// Implement path helpers by delegating to a real Context instance (or re-implementing logic)
// This is a simplified GetFileDownloadPath for testing purposes.
// It constructs paths based on the new logic in util.GetBinaryInfo.
func (m *mockTaskContextForResource) GetFileDownloadPath(componentName, version, arch, filename string) string {
	// This mock needs to align with how util.GetBinaryInfo constructs paths.
	// util.GetBinaryInfo path structure: workdir/.kubexm/${cluster_name}/${type}/${resolved_comp_name_for_dir}/${version}/${arch}/${fileName}
	// or for non-container runtimes: workdir/.kubexm/${cluster_name}/${type}/${version}/${arch}/${fileName}

	// Determine type based on componentName (simplified)
	var typeDir string
	details, ok := util.KnownBinaryDetails[strings.ToLower(componentName)] // Access exported map
	if !ok {
		typeDir = "unknown_type" // Fallback
	} else {
		typeDir = string(details.BinaryType)
	}

	compDirPart := componentName // Default
	if details.ComponentNameForDir != "" {
		compDirPart = details.ComponentNameForDir
	}

	base := filepath.Join(m.globalWorkDir, ".kubexm", m.clusterCfg.Name, typeDir)
	var componentVersionArchPath string

	if details.BinaryType == util.CONTAINERD || details.BinaryType == util.DOCKER || details.BinaryType == util.RUNC || details.BinaryType == util.CRIDOCKERD {
		componentVersionArchPath = filepath.Join(base, compDirPart, version, arch)
	} else {
		componentVersionArchPath = filepath.Join(base, version, arch)
	}

	if filename == "" { // If filename is empty, it means we want the component's version/arch directory
		return componentVersionArchPath
	}
	return filepath.Join(componentVersionArchPath, filename)
}

// Other TaskContext methods (returning nil or default values)
func (m *mockTaskContextForResource) PipelineCache() runtime.PipelineCache                 { return m.pCache }
func (m *mockTaskContextForResource) ModuleCache() runtime.ModuleCache                     { return m.mCache }
func (m *mockTaskContextForResource) TaskCache() runtime.TaskCache                       { return m.tCache }
func (m *mockTaskContextForResource) StepCache() runtime.StepCache                         { return m.sCache }
func (m *mockTaskContextForResource) GetHostsByRole(role string) ([]connector.Host, error) {
	if role == "control-node" { return []connector.Host{m.controlHost}, nil }
	var hosts []connector.Host
	for _, h := range m.clusterCfg.Spec.Hosts {
		for _, r := range h.Roles {
			if r == role {
				hosts = append(hosts, connector.NewHostFromSpec(h))
				break
			}
		}
	}
	return hosts, nil
}
func (m *mockTaskContextForResource) GetComponentArtifactsDir(componentName string) string { return "" }
func (m *mockTaskContextForResource) GetClusterArtifactsDir() string { return filepath.Join(m.globalWorkDir, ".kubexm", m.clusterCfg.Name) }
func (m *mockTaskContextForResource) GetCertsDir() string { return filepath.Join(m.GetClusterArtifactsDir(), "pki") } // Example
func (m *mockTaskContextForResource) GetEtcdCertsDir() string { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockTaskContextForResource) GetEtcdArtifactsDir() string { return filepath.Join(m.GetClusterArtifactsDir(), "etcd") }
func (m *mockTaskContextForResource) GetContainerRuntimeArtifactsDir() string { return filepath.Join(m.GetClusterArtifactsDir(), "container_runtime")}
func (m *mockTaskContextForResource) GetKubernetesArtifactsDir() string { return filepath.Join(m.GetClusterArtifactsDir(), "kubernetes")}
func (m *mockTaskContextForResource) GetHostDir(hostname string) string {return filepath.Join(m.GetClusterArtifactsDir(),"hosts",hostname)}
func (m *mockTaskContextForResource) IsVerbose() bool { return false}
func (m *mockTaskContextForResource) ShouldIgnoreErr() bool {return false}
func (m *mockTaskContextForResource) GetGlobalConnectionTimeout() time.Duration {return 30*time.Second}
func (m *mockTaskContextForResource) WithGoContext(goCtx context.Context) runtime.StepContext {
	newM := *m
	newM.goCtx = goCtx
	return &newM
}


func TestRemoteBinaryHandle_NewRemoteBinaryHandle_Success(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd", "testcluster")
	// Parameters for NewRemoteBinaryHandle: ctx, componentName, version, arch, osName, binaryNameInArchive, expectedChecksum, checksumAlgo
	handle, err := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "amd64", "linux", "etcd", "sha256:abcdef", "sha256")
	require.NoError(t, err)
	require.NotNil(t, handle)

	rbh, ok := handle.(*RemoteBinaryHandle)
	require.True(t, ok)
	assert.Equal(t, "etcd", rbh.ComponentName)
	assert.Equal(t, "v3.5.0", rbh.Version)
	assert.Equal(t, "amd64", rbh.Arch)
	assert.Equal(t, "linux", rbh.OS) // OS is resolved by GetBinaryInfo
	assert.Equal(t, "etcd", rbh.BinaryNameInArchive)
	assert.Equal(t, "sha256:abcdef", rbh.ExpectedChecksum)
	assert.Equal(t, "sha256", rbh.ChecksumAlgorithm)

	require.NotNil(t, rbh.binaryInfo)
	assert.Equal(t, "https://github.com/coreos/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz", rbh.binaryInfo.URL)
	assert.Equal(t, "etcd-v3.5.0-linux-amd64.tar.gz", rbh.binaryInfo.FileName)
	assert.True(t, rbh.binaryInfo.IsArchive)
}

func TestRemoteBinaryHandle_NewRemoteBinaryHandle_ArchResolution(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd", "testclusterarch")
	// Arch is empty, should be resolved from mockCtx.GetHostFacts(controlNode) -> "amd64"
	handle, err := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "", "linux", "etcd", "", "")
	require.NoError(t, err)
	rbh := handle.(*RemoteBinaryHandle)
	assert.Equal(t, "amd64", rbh.Arch)
	require.NotNil(t, rbh.binaryInfo)
	assert.Equal(t, "https://github.com/coreos/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz", rbh.binaryInfo.URL)
	assert.Equal(t, "etcd-v3.5.0-linux-amd64.tar.gz", rbh.binaryInfo.FileName)
}

func TestRemoteBinaryHandle_ID(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd", "testidcluster")
	// componentName, version, arch, osName, binaryNameInArchive
	handle, _ := NewRemoteBinaryHandle(mockCtx, "my-comp", "1.0.0", "arm64", "linux", "my-comp-bin", "", "")
	// Expected format: component-version-os-arch-filename-target-binaryname
	// binaryInfo.FileName for "my-comp" might be "my-comp-1.0.0-linux-arm64. Hypothetical.
	// Let's test with a known component for predictable FileName.
	etcdHandle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.4.0", "amd64", "linux", "etcd", "", "")
	expectedID := "etcd-v3.4.0-linux-amd64-etcd-v3.4.0-linux-amd64.tar.gz-target-etcd"
	assert.Equal(t, expectedID, etcdHandle.ID())

	runcHandle, _ := NewRemoteBinaryHandle(mockCtx, "runc", "v1.1.0", "amd64", "linux", "", "", "") // BinaryNameInArchive is empty, uses FileName from BinaryInfo
	expectedRuncID := "runc-v1.1.0-linux-amd64-runc.amd64" // No -target- part
	assert.Equal(t, expectedRuncID, runcHandle.ID())
}

func TestRemoteBinaryHandle_Path(t *testing.T) {
	workDir := "/testworkdir"
	clusterName := "testpathcluster"
	mockCtx := newMockTaskContextForResource(t, workDir, clusterName)

	// Scenario 1: Archive with specific binary
	etcdHandle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "amd64", "linux", "etcd", "", "")
	expectedPathEtcd := filepath.Join(workDir, ".kubexm", clusterName, "etcd", "v3.5.0", "amd64", "extracted_etcd-v3.5.0-linux-amd64", "etcd")
	actualPathEtcd, err := etcdHandle.Path(mockCtx)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expectedPathEtcd), filepath.Clean(actualPathEtcd))

	// Scenario 2: Direct binary download (not an archive)
	runcHandle, _ := NewRemoteBinaryHandle(mockCtx, "runc", "v1.1.0", "amd64", "linux", "", "", "") // BinaryNameInArchive is empty
	rbhRunc := runcHandle.(*RemoteBinaryHandle)
	expectedPathRunc := rbhRunc.binaryInfo.FilePath // Should be /testworkdir/.kubexm/testpathcluster/runc/runc/v1.1.0/amd64/runc.amd64
	actualPathRunc, err := runcHandle.Path(mockCtx)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expectedPathRunc), filepath.Clean(actualPathRunc))

	// Scenario 3: Archive, but BinaryNameInArchive is empty (Path should point to the archive itself)
	etcdArchiveHandle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.1", "arm64", "linux", "", "", "")
	rbhEtcdArchive := etcdArchiveHandle.(*RemoteBinaryHandle)
	expectedPathEtcdArchive := rbhEtcdArchive.binaryInfo.FilePath // Should be path to .tar.gz
	actualPathEtcdArchive, err := etcdArchiveHandle.Path(mockCtx)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expectedPathEtcdArchive), filepath.Clean(actualPathEtcdArchive))
}


func TestRemoteBinaryHandle_EnsurePlan_Success_Archive(t *testing.T) {
	workDir := "/tmp/ensureplanwd_archive"
	clusterName := "testplancluster"
	mockCtx := newMockTaskContextForResource(t, workDir, clusterName)
	_ = os.MkdirAll(filepath.Join(workDir, ".kubexm", clusterName), 0755)
	defer os.RemoveAll(workDir)

	// componentName, version, arch, osName, binaryNameInArchive, expectedChecksum, checksumAlgo
	handle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.9", "amd64", "linux", "etcd", "checksum123", "sha256")
	rbh := handle.(*RemoteBinaryHandle)

	mockCtx.runner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return false, nil // Simulate nothing exists initially
	}
	mockCtx.runner.GetSHA256Func = func(ctx context.Context, conn connector.Connector, path string) (string, error) {
		return "checksum123", nil // Simulate checksum matches if file were to exist
	}


	fragment, err := handle.EnsurePlan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	// Expect 4 steps: download, (precheck checksum for archive happens before plan or inside download), extract, ensure-finaldir, finalize
	assert.Len(t, fragment.Nodes, 4, "Should plan 4 steps for archive: download, extract, ensure-finaldir, finalize")

	// Check download node
	downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s", handle.ID()))
	downloadNode, ok := fragment.Nodes[downloadNodeID]
	require.True(t, ok, "Download node not found")
	downloadStep, ok := downloadNode.Step.(*commonstep.DownloadFileStep)
	require.True(t, ok)
	assert.Equal(t, rbh.binaryInfo.URL, downloadStep.URL)
	assert.Equal(t, rbh.binaryInfo.FilePath, downloadStep.DestPath) // DestPath is the full path to the downloaded item
	assert.Equal(t, "checksum123", downloadStep.Checksum)

	// Check extract node
	extractNodeID := plan.NodeID(fmt.Sprintf("extract-%s", handle.ID()))
	extractNode, ok := fragment.Nodes[extractNodeID]
	require.True(t, ok)
	extractStep, ok := extractNode.Step.(*commonstep.ExtractArchiveStep)
	require.True(t, ok)
	assert.Equal(t, rbh.binaryInfo.FilePath, extractStep.SourceArchivePath) // Source for extract is the downloaded archive

	archiveBase := strings.TrimSuffix(rbh.binaryInfo.FileName, filepath.Ext(rbh.binaryInfo.FileName))
	if strings.HasSuffix(archiveBase, ".tar") { archiveBase = strings.TrimSuffix(archiveBase, ".tar")}
	expectedExtractDir := filepath.Join(rbh.binaryInfo.ComponentDir, "extracted_"+archiveBase)
	assert.Equal(t, expectedExtractDir, extractStep.DestinationDir)
	assert.Contains(t, extractNode.Dependencies, downloadNodeID)

	// Check ensure-finaldir node
	ensureDirNodeID := plan.NodeID(fmt.Sprintf("ensure-finaldir-%s", handle.ID()))
	ensureDirNode, ok := fragment.Nodes[ensureDirNodeID]
	require.True(t, ok, "Ensure final directory node not found")
	assert.Contains(t, ensureDirNode.Dependencies, extractNodeID)

	// Check finalize node
	finalizeNodeID := plan.NodeID(fmt.Sprintf("finalize-binary-%s", handle.ID()))
	finalizeNode, ok := fragment.Nodes[finalizeNodeID]
	require.True(t, ok, "Finalize node not found")
	finalBinPathTarget, _ := handle.Path(mockCtx) // This is the path to the target binary in the end
	expectedCmd := fmt.Sprintf("cp -fp %s %s && chmod +x %s",
		filepath.Join(expectedExtractDir, rbh.BinaryNameInArchive), // Source from extraction dir
		finalBinPathTarget, // Final destination
		finalBinPathTarget,
	)
	finalizeCmdStep, ok := finalizeNode.Step.(*command.CommandStep)
	require.True(t, ok)
	assert.Equal(t, expectedCmd, finalizeCmdStep.Cmd)
	assert.Contains(t, finalizeNode.Dependencies, ensureDirNodeID)


	assert.ElementsMatch(t, []plan.NodeID{downloadNodeID}, fragment.EntryNodes)
	assert.ElementsMatch(t, []plan.NodeID{finalizeNodeID}, fragment.ExitNodes)
}

func TestRemoteBinaryHandle_EnsurePlan_DirectBinary(t *testing.T) {
	workDir := "/tmp/ensureplanwd_direct"
	clusterName := "testplandirect"
	mockCtx := newMockTaskContextForResource(t, workDir, clusterName)
	_ = os.MkdirAll(filepath.Join(workDir, ".kubexm", clusterName), 0755)
	defer os.RemoveAll(workDir)

	handle, _ := NewRemoteBinaryHandle(mockCtx, "runc", "v1.1.0", "amd64", "linux", "", "checksum-runc", "sha256")
	rbh := handle.(*RemoteBinaryHandle)

	mockCtx.runner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
	mockCtx.runner.GetSHA256Func = func(ctx context.Context, conn connector.Connector, path string) (string, error) { return "checksum-runc", nil }

	fragment, err := handle.EnsurePlan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	// Expect 2 steps for direct binary: download, chmod (no copy if downloadedItemPath == finalBinaryTargetPath)
	// If BinaryNameInArchive was set for a non-archive, it might add a copy. Here it's empty.
	// Path() for non-archive returns binaryInfo.FilePath.
	// downloadedItemPath is binaryInfo.FilePath. So no copy step.
	assert.Len(t, fragment.Nodes, 2, "Should plan 2 steps for direct binary: download, chmod")

	downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s", handle.ID()))
	downloadNode, _ := fragment.Nodes[downloadNodeID]
	downloadStep := downloadNode.Step.(*commonstep.DownloadFileStep)
	assert.Equal(t, rbh.binaryInfo.URL, downloadStep.URL)
	assert.Equal(t, rbh.binaryInfo.FilePath, downloadStep.DestPath)

	chmodNodeID := plan.NodeID(fmt.Sprintf("chmod-direct-binary-%s", handle.ID()))
	chmodNode, _ := fragment.Nodes[chmodNodeID]
	chmodCmdStep := chmodNode.Step.(*command.CommandStep)
	expectedChmodCmd := fmt.Sprintf("chmod +x %s", rbh.binaryInfo.FilePath)
	assert.Equal(t, expectedChmodCmd, chmodCmdStep.Cmd)
	assert.Contains(t, chmodNode.Dependencies, downloadNodeID)

	assert.ElementsMatch(t, []plan.NodeID{downloadNodeID}, fragment.EntryNodes)
	assert.ElementsMatch(t, []plan.NodeID{chmodNodeID}, fragment.ExitNodes)
}


func TestRemoteBinaryHandle_EnsurePlan_BinaryAlreadyExists(t *testing.T) {
	workDir := "/tmp/ensureplanwd_exists"
	clusterName := "testplanexists"
	mockCtx := newMockTaskContextForResource(t, workDir, clusterName)
	_ = os.MkdirAll(filepath.Join(workDir, ".kubexm", clusterName), 0755)
	defer os.RemoveAll(workDir)

	handle, _ := NewRemoteBinaryHandle(mockCtx, "etcd", "v3.5.0", "amd64", "linux", "etcd", "", "")

	// Simulate final binary path exists
	mockCtx.runner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		finalBinPath, _ := handle.Path(mockCtx)
		if path == finalBinPath {
			return true, nil
		}
		return false, nil
	}

	fragment, err := handle.EnsurePlan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	assert.Empty(t, fragment.Nodes, "Plan should be empty if binary already exists")
}

// Example of adding a test for a specific component from knownBinaryDetails
func TestRemoteBinaryHandle_KnownComponent_Containerd(t *testing.T) {
	mockCtx := newMockTaskContextForResource(t, "/tmp/reswd", "testcluster_containerd")
	handle, err := NewRemoteBinaryHandle(mockCtx, "containerd", "1.6.8", "amd64", "linux", "containerd", "", "")
	require.NoError(t, err)
	rbh := handle.(*RemoteBinaryHandle)

	assert.Equal(t, "containerd", rbh.ComponentName)
	assert.Equal(t, "1.6.8", rbh.Version) // Retains original version passed
	assert.Equal(t, "containerd", rbh.BinaryNameInArchive) // Explicitly passed

	require.NotNil(t, rbh.binaryInfo)
	assert.Equal(t, "linux", rbh.binaryInfo.OS)
	assert.Equal(t, "amd64", rbh.binaryInfo.Arch)
	assert.Equal(t, "containerd-1.6.8-linux-amd64.tar.gz", rbh.binaryInfo.FileName)
	assert.True(t, rbh.binaryInfo.IsArchive)
	assert.Contains(t, rbh.binaryInfo.URL, "https://github.com/containerd/containerd/releases/download/v1.6.8/containerd-1.6.8-linux-amd64.tar.gz")

	expectedPath := filepath.Join("/tmp/reswd", ".kubexm", "testcluster_containerd", "containerd", "containerd", "1.6.8", "amd64", "extracted_containerd-1.6.8-linux-amd64", "containerd")
	actualPath, _ := handle.Path(mockCtx)
	assert.Equal(t, filepath.Clean(expectedPath), filepath.Clean(actualPath))
}
