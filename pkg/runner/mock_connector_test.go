package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Assuming this is the correct path
)

// MockConnector is a mock implementation of the connector.Connector interface for testing.
type MockConnector struct {
	// ConnectFunc can be set to mock the Connect method.
	ConnectFunc func(ctx context.Context, cfg connector.ConnectionCfg) error
	// ExecFunc can be set to mock the Exec method.
	ExecFunc func(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout, stderr []byte, err error)
	// CopyFunc can be set to mock the Copy method.
	CopyFunc func(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error
	// CopyContentFunc can be set to mock the CopyContent method.
	CopyContentFunc func(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error
	// FetchFunc can be set to mock the Fetch method.
	FetchFunc func(ctx context.Context, remotePath, localPath string) error
	// StatFunc can be set to mock the Stat method.
	StatFunc func(ctx context.Context, path string) (*connector.FileStat, error)
	// LookPathFunc can be set to mock the LookPath method.
	LookPathFunc func(ctx context.Context, file string) (string, error)
	// GetOSFunc can be set to mock the GetOS method.
	GetOSFunc func(ctx context.Context) (*connector.OS, error)
	// IsConnectedFunc can be set to mock the IsConnected method.
	IsConnectedFunc func() bool
	// CloseFunc can be set to mock the Close method.
	CloseFunc func() error

	// LastExecCmd stores the last command passed to ExecFunc, useful for assertions.
	LastExecCmd     string
	LastExecOptions *connector.ExecOptions
	ExecHistory     []string // To store all commands if needed

	// FileSystem state for mock Stat, Copy, etc.
	mockFs map[string]*connector.FileStat
	mockFileContent map[string][]byte
}

// NewMockConnector creates a new MockConnector with default behaviors.
func NewMockConnector() *MockConnector {
	mc := &MockConnector{
		// Default IsConnected to true for most runner tests
		IsConnectedFunc: func() bool { return true },
		// Default Connect does nothing and returns nil
		ConnectFunc: func(ctx context.Context, cfg connector.ConnectionCfg) error { return nil },
		// Default Close does nothing and returns nil
		CloseFunc: func() error { return nil },
		// Default GetOS returns a generic Linux OS
		GetOSFunc: func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", VersionID: "test", Arch: "amd64", Kernel: "mock-kernel"}, nil
		},
		// Default LookPath assumes command is found
		LookPathFunc: func(ctx context.Context, file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
		// Default Stat assumes file exists and is a file
		StatFunc: func(ctx context.Context, path string) (*connector.FileStat, error) {
			if strings.HasSuffix(path, "nonexistent") || strings.Contains(path, "nonexistent") {
				return &connector.FileStat{Name: path, IsExist: false}, nil
			}
			isDir := strings.HasSuffix(path, "/") || path == "/tmp" || path == "/test/dir"
			return &connector.FileStat{
				Name:    path,
				Size:    1024,
				Mode:    0644,
				ModTime: time.Now(),
				IsDir:   isDir,
				IsExist: true,
			}, nil
		},
		mockFs: make(map[string]*connector.FileStat),
		mockFileContent: make(map[string][]byte),
	}
	// Default ExecFunc that updates LastExecCmd, LastExecOptions, and ExecHistory
	mc.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout []byte, stderr []byte, err error) {
		mc.LastExecCmd = cmd
		mc.LastExecOptions = options
		if mc.ExecHistory == nil {
			mc.ExecHistory = []string{}
		}
		mc.ExecHistory = append(mc.ExecHistory, cmd)
		return []byte(""), []byte(""), nil
	}
	mc.CopyContentFunc = func(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
		return nil
	}
	mc.CopyFunc = func(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error {
		return nil
	}
	mc.FetchFunc = func(ctx context.Context, remotePath, localPath string) error {
		return nil
	}
	return mc
}

func (m *MockConnector) Connect(ctx context.Context, cfg connector.ConnectionCfg) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx, cfg)
	}
	return fmt.Errorf("ConnectFunc not implemented in mock")
}

func (m *MockConnector) Exec(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout []byte, stderr []byte, err error) {
	m.LastExecCmd = cmd // Store for test assertions
	m.LastExecOptions = options
	if m.ExecHistory == nil {
		m.ExecHistory = []string{}
	}
	m.ExecHistory = append(m.ExecHistory, cmd)

	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, cmd, options)
	}
	return nil, nil, fmt.Errorf("ExecFunc not implemented in mock")
}

func (m *MockConnector) Copy(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error {
	if m.CopyFunc != nil {
		return m.CopyFunc(ctx, srcPath, dstPath, options)
	}
	return fmt.Errorf("CopyFunc not implemented in mock")
}

func (m *MockConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
	if m.CopyContentFunc != nil {
		return m.CopyContentFunc(ctx, content, dstPath, options)
	}
	return fmt.Errorf("CopyContentFunc not implemented in mock")
}

func (m *MockConnector) Fetch(ctx context.Context, remotePath, localPath string) error {
	if m.FetchFunc != nil {
		return m.FetchFunc(ctx, remotePath, localPath)
	}
	return fmt.Errorf("FetchFunc not implemented in mock")
}

func (m *MockConnector) Stat(ctx context.Context, path string) (*connector.FileStat, error) {
	if stat, exists := m.mockFs[path]; exists {
		return stat, nil
	}
	if m.StatFunc != nil {
		return m.StatFunc(ctx, path)
	}
	return nil, fmt.Errorf("StatFunc not implemented in mock, and path not in mockFs")
}

func (m *MockConnector) LookPath(ctx context.Context, file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(ctx, file)
	}
	return "", fmt.Errorf("LookPathFunc not implemented in mock")
}

func (m *MockConnector) GetOS(ctx context.Context) (*connector.OS, error) {
	if m.GetOSFunc != nil {
		return m.GetOSFunc(ctx)
	}
	return nil, fmt.Errorf("GetOSFunc not implemented in mock")
}

func (m *MockConnector) IsConnected() bool {
	if m.IsConnectedFunc != nil {
		return m.IsConnectedFunc()
	}
	return false // Default to false if not set
}

func (m *MockConnector) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return fmt.Errorf("CloseFunc not implemented in mock")
}

// Helper methods for mock setup
func (m *MockConnector) AddMockFile(path string, stat *connector.FileStat, content []byte) {
	m.mockFs[path] = stat
	if content != nil {
		m.mockFileContent[path] = content
	}
}

// Ensure MockConnector implements Connector interface
var _ connector.Connector = &MockConnector{}
