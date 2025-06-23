package containerd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForInstallContainerd is a helper to create a StepContext for testing.
func mockStepContextForInstallContainerd(t *testing.T, mockRunner runner.Runner, hostName string, taskCacheValues map[string]interface{}) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-install-ctd"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		TaskCache:     cache.NewTaskCache(),
		GlobalWorkDir: "/tmp/kubexm_install_ctd_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-install-ctd"
	}
	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "127.0.0.1", Type: "local"}
	currentHost := connector.NewHostFromSpec(hostSpec)
	mainCtx.GetHostInfoMap()[hostName] = &runtime.HostRuntimeInfo{
		Host:  currentHost,
		Conn:  &connector.LocalConnector{},
		Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}},
	}
	mainCtx.SetCurrentHost(currentHost)
	mainCtx.SetControlNode(currentHost)

	if taskCacheValues != nil {
		for k, v := range taskCacheValues {
			mainCtx.TaskCache().Set(k, v)
		}
	}
	return mainCtx
}

// mockRunnerForInstallContainerd provides a mock implementation of runner.Runner.
type mockRunnerForInstallContainerd struct {
	runner.Runner
	ExistsFunc func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RunFunc    func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
	ChmodFunc  func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RemoveFunc func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForInstallContainerd) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForInstallContainerd) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForInstallContainerd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, conn, cmd, sudo)
	}
	return "", fmt.Errorf("RunFunc not implemented")
}
func (m *mockRunnerForInstallContainerd) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.ChmodFunc != nil {
		return m.ChmodFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("ChmodFunc not implemented")
}
func (m *mockRunnerForInstallContainerd) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}

func TestInstallContainerdStep_New(t *testing.T) {
	binaries := map[string]string{"bin/ctr": "/usr/local/bin/ctr-custom"}
	s := NewInstallContainerdStep("TestInstallCtd", "srcKey", binaries, "custom.service", "/opt/systemd/custom.service", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestInstallCtd", meta.Name)
	assert.Contains(t, meta.Description, "srcKey")

	ics, ok := s.(*InstallContainerdStep)
	require.True(t, ok)
	assert.Equal(t, "srcKey", ics.SourceExtractedPathSharedDataKey)
	assert.Equal(t, binaries, ics.BinariesToCopy)
	assert.Equal(t, "custom.service", ics.SystemdUnitFileSourceRelPath)
	assert.Equal(t, "/opt/systemd/custom.service", ics.SystemdUnitFileTargetPath)
	assert.True(t, ics.Sudo)

	sDefaults := NewInstallContainerdStep("", "", nil, "", "", false)
	icsDefaults, _ := sDefaults.(*InstallContainerdStep)
	assert.Equal(t, "InstallContainerdFromExtracted", icsDefaults.Meta().Name)
	assert.Equal(t, "DefaultExtractedContainerdPath", icsDefaults.SourceExtractedPathSharedDataKey)
	assert.NotEmpty(t, icsDefaults.BinariesToCopy)
	assert.Equal(t, "containerd.service", icsDefaults.SystemdUnitFileSourceRelPath)
	assert.Equal(t, "/usr/lib/systemd/system/containerd.service", icsDefaults.SystemdUnitFileTargetPath)
	assert.False(t, icsDefaults.Sudo)
}

func TestInstallContainerdStep_Precheck_AllExist(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{}
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-ctd-precheck-exists", nil)

	s := NewInstallContainerdStep("", "srcKey", nil, "containerd.service", "/etc/systemd/system/containerd.service", true).(*InstallContainerdStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		for _, defaultBinTarget := range s.BinariesToCopy {
			if path == defaultBinTarget {
				return true, nil
			}
		}
		if path == s.SystemdUnitFileTargetPath {
			return true, nil
		}
		return false, fmt.Errorf("unexpected Exists call for path: %s", path)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if all items exist")
}

func TestInstallContainerdStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{}
	extractedPath := "/tmp/extracted_ctd_content"
	taskCache := map[string]interface{}{"srcKey": extractedPath}
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-run-install-ctd", taskCache)

	binaries := map[string]string{"bin/containerd": "/usr/local/bin/containerd", "bin/ctr": "/usr/local/bin/ctr"}
	systemdSrcRel := "lib/systemd/system/containerd.service" // Relative to extractedPath
	systemdTarget := "/etc/systemd/system/containerd.service"
	s := NewInstallContainerdStep("", "srcKey", binaries, systemdSrcRel, systemdTarget, true).(*InstallContainerdStep)

	var mkdirPaths, cpCmds, chmodPaths []string
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		mkdirPaths = append(mkdirPaths, path)
		assert.True(t, sudo)
		return nil
	}
	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		// Assume source files exist in the extracted directory
		if strings.HasPrefix(path, extractedPath) {
			return true, nil
		}
		return false, nil
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if strings.HasPrefix(cmd, "cp -fp") || strings.HasPrefix(cmd, "cp -f") {
			cpCmds = append(cpCmds, cmd)
			assert.True(t, sudo)
			return "", nil
		}
		return "", fmt.Errorf("unexpected Run call: %s", cmd)
	}
	mockRunner.ChmodFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		chmodPaths = append(chmodPaths, path+":"+permissions)
		assert.True(t, sudo)
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.Contains(t, mkdirPaths, "/usr/local/bin")
	assert.Contains(t, mkdirPaths, "/etc/systemd/system")

	assert.Contains(t, cpCmds, fmt.Sprintf("cp -fp %s %s", filepath.Join(extractedPath, "bin/containerd"), "/usr/local/bin/containerd"))
	assert.Contains(t, cpCmds, fmt.Sprintf("cp -fp %s %s", filepath.Join(extractedPath, "bin/ctr"), "/usr/local/bin/ctr"))
	assert.Contains(t, cpCmds, fmt.Sprintf("cp -f %s %s", filepath.Join(extractedPath, systemdSrcRel), systemdTarget))

	assert.Contains(t, chmodPaths, "/usr/local/bin/containerd:0755")
	assert.Contains(t, chmodPaths, "/usr/local/bin/ctr:0755")
	assert.Contains(t, chmodPaths, systemdTarget+":0644")
}

func TestInstallContainerdStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{}
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-rollback-install-ctd", nil)

	binaries := map[string]string{"bin/containerd": "/usr/local/bin/containerd"}
	systemdTarget := "/etc/systemd/system/containerd.service"
	s := NewInstallContainerdStep("", "srcKey", binaries, "containerd.service", systemdTarget, true).(*InstallContainerdStep)

	var removedPaths []string
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removedPaths = append(removedPaths, path)
		assert.True(t, sudo)
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Contains(t, removedPaths, "/usr/local/bin/containerd")
	assert.Contains(t, removedPaths, systemdTarget)
}

var _ runner.Runner = (*mockRunnerForInstallContainerd)(nil)
var _ step.StepContext = (*mockStepContextForInstallContainerd)(t, nil, "", nil, nil)

// Dummy implementations for other runner.Runner methods
func (m *mockRunnerForInstallContainerd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForInstallContainerd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForInstallContainerd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForInstallContainerd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForInstallContainerd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallContainerd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallContainerd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForInstallContainerd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallContainerd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForInstallContainerd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForInstallContainerd) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForInstallContainerd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForInstallContainerd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForInstallContainerd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForInstallContainerd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForInstallContainerd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForInstallContainerd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForInstallContainerd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForInstallContainerd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallContainerd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallContainerd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallContainerd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallContainerd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallContainerd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForInstallContainerd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallContainerd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallContainerd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForInstallContainerd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForInstallContainerd) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_InstallCtd(t *testing.T) {
	var _ step.StepContext = mockStepContextForInstallContainerd(t, &mockRunnerForInstallContainerd{}, "dummy", nil)
}
