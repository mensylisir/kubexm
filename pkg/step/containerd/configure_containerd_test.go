package containerd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	// "time" // Not strictly needed for these test cases

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// newTestContextForContainerd is defined in install_containerd_test.go (or shared mock file)
// It's assumed to be accessible as these files are in the same package.

func TestConfigureContainerdStepExecutor_Execute_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// OS doesn't matter much for this step's core logic if not affecting Mkdirp/WriteFile.
	ctx := newTestContextForContainerd(t, mockConn, "linux-generic")

	confSpec := &ConfigureContainerdStepSpec{
		UseSystemdCgroup: true,
		RegistryMirrors: map[string]string{"docker.io": "https://mirror.mycorp.com"},
		ConfigFilePath: "/test/containerd/config.toml", // Custom path for test
	}
	// Ensure the spec name is resolved if needed for logging within the test before execution
	// specName := confSpec.GetName()

	executor := step.GetExecutor(step.GetSpecTypeName(confSpec))
	if executor == nil {t.Fatalf("Executor not registered for ConfigureContainerdStepSpec (type: %s)", step.GetSpecTypeName(confSpec))}

	var writtenContent string
	var mkdirCalledWithPath string

	// Mock for Mkdirp (which uses Exec "mkdir -p ...")
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, "mkdir -p") && options.Sudo {
			parts := strings.Fields(cmd)
			if len(parts) >= 3 {
				mkdirCalledWithPath = parts[2]
			} else {
				t.Errorf("Mkdirp command format unexpected: %s", cmd)
			}
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("ConfigureContainerd unexpected Exec call: %s", cmd)
	}
	// Mock for WriteFile (which uses CopyContent)
	mockConn.CopyContentFunc = func(ctxGo context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
		if dstPath == confSpec.effectiveConfigPath() && options.Sudo && options.Permissions == "0644" {
			writtenContent = string(content)
			return nil
		}
		return fmt.Errorf("unexpected CopyContent call: path=%s, sudo=%v, perms=%s", dstPath, options.Sudo, options.Permissions)
	}

	res := executor.Execute(confSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Execute status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if mkdirCalledWithPath != "/test/containerd" { // Parent dir of ConfigFilePath
		t.Errorf("Mkdirp called with path %s, want /test/containerd", mkdirCalledWithPath)
	}
	if !strings.Contains(writtenContent, "SystemdCgroup = true") {
		t.Error("Rendered config does not contain SystemdCgroup = true")
	}
	if !strings.Contains(writtenContent, `[plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]`) ||
	   !strings.Contains(writtenContent, `endpoint = ["https://mirror.mycorp.com"]`) {
		t.Error("Rendered config does not contain docker.io mirror configuration")
	}
}

func TestConfigureContainerdStepExecutor_Check_ConfigExistsAndMatches(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "linux-generic")

	confSpec := &ConfigureContainerdStepSpec{
		UseSystemdCgroup: true,
		ConfigFilePath: DefaultContainerdConfigPath,
	}
	executor := step.GetExecutor(step.GetSpecTypeName(confSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		if path == confSpec.effectiveConfigPath() {
			return &connector.FileStat{Name: "config.toml", IsExist: true, IsDir: false}, nil
		}
		return &connector.FileStat{Name: path, IsExist: false}, nil
	}
	// Mock for ReadFile (which uses Exec "cat ...")
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, fmt.Sprintf("cat %s", confSpec.effectiveConfigPath())) {
			return []byte("SystemdCgroup = true\nSomeOtherContent"), nil, nil
		}
		return nil, nil, fmt.Errorf("ConfigureContainerd.Check unexpected Exec call: %s", cmd)
	}

	isDone, err := executor.Check(confSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (config matches)")}
}

func TestConfigureContainerdStepExecutor_Check_FileNotExists(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "linux-generic")
	confSpec := &ConfigureContainerdStepSpec{ConfigFilePath: "/no/such/config.toml"}
	executor := step.GetExecutor(step.GetSpecTypeName(confSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		if path == confSpec.effectiveConfigPath() {
			return &connector.FileStat{Name: "config.toml", IsExist: false}, nil
		}
		return nil, errors.New("StatFunc called for unexpected path")
	}
	isDone, err := executor.Check(confSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (file not exists)")}
}
