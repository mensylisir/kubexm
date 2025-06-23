package etcd

import (
	"context"
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

// mockStepContextForEtcd is a helper to create a StepContext for testing etcd steps.
func mockStepContextForEtcd(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-etcd"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_etcd_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-etcd"
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
	return mainCtx
}

// mockRunnerForEtcd provides a mock implementation of runner.Runner for etcd steps.
type mockRunnerForEtcd struct {
	runner.Runner
	LookPathFunc       func(ctx context.Context, conn connector.Connector, file string) (string, error)
	ExistsFunc         func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc         func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RunWithOptionsFunc func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error)
	StatFunc           func(ctx context.Context, conn connector.Connector, path string) (*connector.FileStat, error)
	RemoveFunc         func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForEtcd) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
	if m.LookPathFunc != nil { return m.LookPathFunc(ctx, conn, file) }
	return "/usr/bin/" + file, nil // Default success
}
func (m *mockRunnerForEtcd) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil { return m.ExistsFunc(ctx, conn, path) }
	return false, nil // Default to not exists
}
func (m *mockRunnerForEtcd) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil { return m.MkdirpFunc(ctx, conn, path, permissions, sudo) }
	return nil // Default success
}
func (m *mockRunnerForEtcd) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if m.RunWithOptionsFunc != nil { return m.RunWithOptionsFunc(ctx, conn, cmd, opts) }
	return []byte("mock stdout"), nil, nil // Default success
}
func (m *mockRunnerForEtcd) Stat(ctx context.Context, conn connector.Connector, path string) (*connector.FileStat, error) {
	if m.StatFunc != nil { return m.StatFunc(ctx, conn, path) }
	return &connector.FileStat{Name: filepath.Base(path), Size: 1024, IsExist: true}, nil // Default success
}
func (m *mockRunnerForEtcd) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil { return m.RemoveFunc(ctx, conn, path, sudo) }
	return nil // Default success
}


func TestBackupEtcdStep_New(t *testing.T) {
	ts := time.Now().Format("2006-01-02T150405Z0700")
	defaultBackupPath := filepath.Join("/var/backups/etcd", fmt.Sprintf("snapshot-%s.db", ts))

	s := NewBackupEtcdStep("TestBackup", "https://etcd1:2379", "/ca.crt", "/client.crt", "/client.key", "/custom/backup.db", "/opt/bin/etcdctl", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestBackup", meta.Name)
	assert.Contains(t, meta.Description, "https://etcd1:2379")
	assert.Contains(t, meta.Description, "/custom/backup.db")

	bes, ok := s.(*BackupEtcdStep)
	require.True(t, ok)
	assert.Equal(t, "https://etcd1:2379", bes.Endpoints)
	assert.Equal(t, "/custom/backup.db", bes.BackupFilePath)
	assert.Equal(t, "/opt/bin/etcdctl", bes.EtcdctlPath)
	assert.True(t, bes.Sudo)

	sDefaults := NewBackupEtcdStep("", "", "", "", "", "", "", false)
	besDefaults, _ := sDefaults.(*BackupEtcdStep)
	assert.Equal(t, "BackupEtcd", besDefaults.Meta().Name)
	assert.Equal(t, "https://127.0.0.1:2379", besDefaults.Endpoints)
	// For default backup path, we can only check the directory due to timestamp
	assert.Equal(t, filepath.Dir(defaultBackupPath), filepath.Dir(besDefaults.BackupFilePath))
	assert.Equal(t, "etcdctl", besDefaults.EtcdctlPath)
	assert.False(t, besDefaults.Sudo)
}

func TestBackupEtcdStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-backup-etcd")

	backupFile := "/mnt/etcd_backups/snapshot.db"
	s := NewBackupEtcdStep("", "https://node1:2379", "/pki/ca.pem", "/pki/cert.pem", "/pki/key.pem", backupFile, "etcdctl", true).(*BackupEtcdStep)

	var mkdirpCalled, etcdctlCalled bool
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		if path == filepath.Dir(backupFile) {
			mkdirpCalled = true
			assert.True(t, sudo)
			assert.Equal(t, "0700", permissions)
			return nil
		}
		return fmt.Errorf("unexpected Mkdirp call: %s", path)
	}
	mockRunner.RunWithOptionsFunc = func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.HasPrefix(cmd, "ETCDCTL_API=3 etcdctl snapshot save") {
			etcdctlCalled = true
			assert.True(t, opts.Sudo)
			assert.Contains(t, cmd, backupFile)
			assert.Contains(t, cmd, "--endpoints=https://node1:2379")
			assert.Contains(t, cmd, "--cacert=/pki/ca.pem")
			return []byte("Snapshot saved at " + backupFile), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call: %s", cmd)
	}
	mockRunner.StatFunc = func(ctx context.Context, conn connector.Connector, path string) (*connector.FileStat, error) {
		if path == backupFile {
			return &connector.FileStat{Name: filepath.Base(backupFile), Size: 10240, IsExist: true}, nil
		}
		return nil, fmt.Errorf("unexpected Stat call")
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, mkdirpCalled, "Mkdirp should be called for backup directory")
	assert.True(t, etcdctlCalled, "etcdctl snapshot save should be called")
}

func TestBackupEtcdStep_Precheck_EtcdctlNotFound(t *testing.T) {
	mockRunner := &mockRunnerForEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-backup-precheck-noctl")
	s := NewBackupEtcdStep("", "", "", "", "", "", "/custom/nonexistent/etcdctl", true).(*BackupEtcdStep)

	mockRunner.LookPathFunc = func(ctx context.Context, conn connector.Connector, file string) (string, error) {
		if file == "/custom/nonexistent/etcdctl" {
			return "", fmt.Errorf("etcdctl not found")
		}
		return "", fmt.Errorf("unexpected LookPath call")
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "etcdctl command '/custom/nonexistent/etcdctl' not found")
}

// Ensure mockRunnerForEtcd implements runner.Runner
var _ runner.Runner = (*mockRunnerForEtcd)(nil)
// Ensure mockStepContextForEtcd implements step.StepContext
var _ step.StepContext = (*mockStepContextForEtcd)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForEtcd
func (m *mockRunnerForEtcd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForEtcd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForEtcd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForEtcd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForEtcd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForEtcd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForEtcd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForEtcd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForEtcd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForEtcd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForEtcd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForEtcd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForEtcd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForEtcd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForEtcd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForEtcd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForEtcd) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_EtcdBackup(t *testing.T) {
	var _ step.StepContext = mockStepContextForEtcd(t, &mockRunnerForEtcd{}, "dummy")
}
