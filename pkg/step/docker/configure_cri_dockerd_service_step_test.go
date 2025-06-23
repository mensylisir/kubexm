package docker

import (
	"context"
	"fmt"
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

// Reusing mockStepContextForDockerCleanup and mockRunnerForDockerCleanup
// from cleanup_docker_config_step_test.go, adding ReadFileFunc.

type mockRunnerForConfigureCriDockerd struct {
	runner.Runner
	ExistsFunc    func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ReadFileFunc  func(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	WriteFileFunc func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	// Add other funcs if needed by the step
}

func (m *mockRunnerForConfigureCriDockerd) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForConfigureCriDockerd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(ctx, conn, path)
	}
	return nil, fmt.Errorf("ReadFileFunc not implemented")
}
func (m *mockRunnerForConfigureCriDockerd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, conn, content, destPath, permissions, sudo)
	}
	return fmt.Errorf("WriteFileFunc not implemented")
}


func TestConfigureCriDockerdServiceStep_New(t *testing.T) {
	args := map[string]string{"--network-plugin": "cni", "--another-flag": "value"}
	s := NewConfigureCriDockerdServiceStep("TestConfigCriD", "/opt/cri-dockerd.service", args, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestConfigCriD", meta.Name)
	assert.Contains(t, meta.Description, "/opt/cri-dockerd.service")
	assert.Contains(t, meta.Description, "--network-plugin")

	ccss, ok := s.(*ConfigureCriDockerdServiceStep)
	require.True(t, ok)
	assert.Equal(t, "/opt/cri-dockerd.service", ccss.ServiceFilePath)
	assert.Equal(t, args, ccss.ExecStartArgs)
	assert.True(t, ccss.Sudo)

	sDefaults := NewConfigureCriDockerdServiceStep("", "", nil, false)
	ccssDefaults, _ := sDefaults.(*ConfigureCriDockerdServiceStep)
	assert.Equal(t, "ConfigureCriDockerdService", ccssDefaults.Meta().Name)
	assert.Equal(t, DefaultCriDockerdServicePath, ccssDefaults.ServiceFilePath)
	assert.Nil(t, ccssDefaults.ExecStartArgs) // Default args is nil
	assert.True(t, ccssDefaults.Sudo)       // Default Sudo is true
}

func TestConfigureCriDockerdServiceStep_modifyExecStart(t *testing.T) {
	s := NewConfigureCriDockerdServiceStep("", "", nil, true).(*ConfigureCriDockerdServiceStep)
	tests := []struct {
		name           string
		currentExec    string
		argsToSet      map[string]string
		expectedExec   string
	}{
		{
			name:        "add new arg",
			currentExec: "/usr/bin/cri-dockerd --some-existing",
			argsToSet:   map[string]string{"--new-arg": "new-value"},
			// Order is not guaranteed by map iteration, so check for presence
		},
		{
			name:        "modify existing arg",
			currentExec: "/usr/bin/cri-dockerd --change-me old-value --keep-me",
			argsToSet:   map[string]string{"--change-me": "new-value"},
		},
		{
			name:        "add boolean flag",
			currentExec: "/usr/bin/cri-dockerd",
			argsToSet:   map[string]string{"--enable-feature": "true"}, // "true" indicates boolean present
		},
		{
			name:        "remove implicit boolean by setting value",
			currentExec: "/usr/bin/cri-dockerd --was-bool", // Assume --was-bool implies true
			argsToSet:   map[string]string{"--was-bool": "now-has-value"},
		},
		{
			name:        "complex case with multiple changes",
			currentExec: "/usr/bin/cri-dockerd --old-val 1 --another --yet-another valX",
			argsToSet:   map[string]string{"--old-val": "2", "--new-flag": "true", "--yet-another": "valY"},
		},
		{
			name: "empty current exec",
			currentExec: "",
			argsToSet: map[string]string{"--flag": "val"},
			expectedExec: "", // modifyExecStart expects at least the command part
		},
		{
			name: "command only",
			currentExec: "/usr/bin/cmd",
			argsToSet: map[string]string{"--f1": "v1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modified := s.modifyExecStart(tt.currentExec, tt.argsToSet)
			if tt.expectedExec != "" { // Only assert if specific output is expected (for simple cases)
				assert.Equal(t, tt.expectedExec, modified)
			} else { // For complex cases, check presence/absence of args
				parts := strings.Fields(modified)
				command := parts[0]
				if tt.currentExec != "" {
					assert.Equal(t, strings.Fields(tt.currentExec)[0], command)
				}

				// Check that all argsToSet are correctly represented
				for key, val := range tt.argsToSet {
					foundKey := false
					for i, part := range parts {
						if part == key {
							foundKey = true
							if val != "true" && val != "" { // If it's not a boolean flag and has a value
								require.Less(t, i+1, len(parts), "Value expected after key %s", key)
								assert.Equal(t, val, parts[i+1])
							}
							break
						}
					}
					assert.True(t, foundKey, "Key %s not found in modified exec: %s", key, modified)
				}
			}
		})
	}
}


func TestConfigureCriDockerdServiceStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForConfigureCriDockerd{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-cfg-crid") // Reusing context helper

	filePath := "/etc/systemd/system/cri-dockerd.service"
	initialContent := "[Service]\nExecStart=/usr/local/bin/cri-dockerd --old-flag value1\n"
	argsToSet := map[string]string{"--new-flag": "value2", "--old-flag": "new-value1"}

	s := NewConfigureCriDockerdServiceStep("", filePath, argsToSet, true).(*ConfigureCriDockerdServiceStep)

	mockRunner.ReadFileFunc = func(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
		if path == filePath {
			return []byte(initialContent), nil
		}
		return nil, fmt.Errorf("unexpected ReadFile call")
	}

	var writtenContent string
	mockRunner.WriteFileFunc = func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
		if destPath == filePath {
			writtenContent = string(content)
			assert.True(t, sudo)
			assert.Equal(t, "0644", permissions)
			return nil
		}
		return fmt.Errorf("unexpected WriteFile call")
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.Contains(t, writtenContent, "ExecStart=/usr/local/bin/cri-dockerd")
	assert.Contains(t, writtenContent, "--old-flag new-value1")
	assert.Contains(t, writtenContent, "--new-flag value2")
	assert.NotContains(t, writtenContent, "--old-flag value1") // Old value should be replaced
}


func TestConfigureCriDockerdServiceStep_Precheck_NoArgsToSet(t *testing.T) {
	mockRunner := &mockRunnerForConfigureCriDockerd{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-precheck-noargs")
	s := NewConfigureCriDockerdServiceStep("", DefaultCriDockerdServicePath, nil, true).(*ConfigureCriDockerdServiceStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if no args to set")
}

// Ensure mockRunnerForConfigureCriDockerd implements runner.Runner
var _ runner.Runner = (*mockRunnerForConfigureCriDockerd)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForConfigureCriDockerd
func (m *mockRunnerForConfigureCriDockerd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForConfigureCriDockerd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForConfigureCriDockerd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForConfigureCriDockerd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForConfigureCriDockerd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForConfigureCriDockerd) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForConfigureCriDockerd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureCriDockerd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForConfigureCriDockerd) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_DockerConfigureCriDSvc(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForConfigureCriDockerd{}, "dummy")
}
