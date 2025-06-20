package runtime
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Real import
)

type mockConnectorForRuntime struct {
	ConnectFunc     func(ctx context.Context, cfg connector.ConnectionCfg) error
	GetOSFunc       func(ctx context.Context) (*connector.OS, error)
	ExecFunc        func(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout []byte, stderr []byte, err error)
	IsConnectedFunc func() bool
	CloseFunc       func() error
	// Add other methods if NewRunner in the tests needs them, but keep minimal for runtime tests
}

func (m *mockConnectorForRuntime) Connect(ctx context.Context, cfg connector.ConnectionCfg) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx, cfg)
	}
	return nil // Default success
}

func (m *mockConnectorForRuntime) Exec(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, cmd, options)
	}
	// Default for NewRunner's fact gathering
	if strings.Contains(cmd, "hostname") { return []byte("mockhost"), nil, nil }
	if strings.Contains(cmd, "uname -r") { return []byte("mock-kernel"), nil, nil }
	if strings.Contains(cmd, "nproc") { return []byte("2"), nil, nil }
	if strings.Contains(cmd, "grep MemTotal") { return []byte("2048000"), nil, nil } // 2GB
	if strings.Contains(cmd, "ip -4 route") { return []byte("1.2.3.4"), nil, nil }
	if strings.Contains(cmd, "ip -6 route") { return []byte("::1"), nil, nil } // Provide a default for IPv6
	return []byte(""), []byte(""), nil
}

func (m *mockConnectorForRuntime) Copy(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error {
	return fmt.Errorf("not implemented in mockConnectorForRuntime")
}
func (m *mockConnectorForRuntime) CopyContent(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
	return fmt.Errorf("not implemented in mockConnectorForRuntime")
}
func (m *mockConnectorForRuntime) Fetch(ctx context.Context, remotePath, localPath string) error {
	return fmt.Errorf("not implemented in mockConnectorForRuntime")
}
func (m *mockConnectorForRuntime) Stat(ctx context.Context, path string) (*connector.FileStat, error) {
	return &connector.FileStat{Name: path, IsExist: true}, fmt.Errorf("not implemented in mockConnectorForRuntime but provides basic stat")
}
func (m *mockConnectorForRuntime) LookPath(ctx context.Context, file string) (string, error) {
	return "/" + file, nil // Default success, assuming command exists
}
func (m *mockConnectorForRuntime) GetOS(ctx context.Context) (*connector.OS, error) {
	if m.GetOSFunc != nil {
		return m.GetOSFunc(ctx)
	}
	return &connector.OS{ID: "linux-mock", Arch: "amd64", Kernel: "mock-kernel", VersionID: "1.0"}, nil // Default success
}
func (m *mockConnectorForRuntime) IsConnected() bool {
	if m.IsConnectedFunc != nil {
		return m.IsConnectedFunc()
	}
	return true // Default success
}
func (m *mockConnectorForRuntime) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil // Default success
}

var _ connector.Connector = &mockConnectorForRuntime{}
