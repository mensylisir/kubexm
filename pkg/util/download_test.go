package util

import (
	"context"
	"fmt" // Added for Sprintf
	"os"  // For reading log file
	"path/filepath"
	"strings"
	"testing"
	// No longer using custom mock logger, will use real logger with file output
	// "github.com/mensylisir/kubexm/pkg/connector" // No longer needed
	"github.com/mensylisir/kubexm/pkg/logger"
)

// mockConnectorForUtil is no longer needed as DownloadFileWithConnector does not take a connector.
/*
type mockConnectorForUtil struct{}

func (m *mockConnectorForUtil) Connect(ctx context.Context, cfg connector.ConnectionCfg) error { return nil }
func (m *mockConnectorForUtil) Exec(ctx context.Context, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) { return nil, nil, nil }
func (m *mockConnectorForUtil) CopyContent(ctx context.Context, content []byte, destPath string, options *connector.FileTransferOptions) error { return nil }
func (m *mockConnectorForUtil) Stat(ctx context.Context, path string) (*connector.FileStat, error) { return &connector.FileStat{IsExist: true}, nil }
func (m *mockConnectorForUtil) LookPath(ctx context.Context, file string) (string, error) { return file, nil }
func (m *mockConnectorForUtil) Close() error { return nil }
func (m *mockConnectorForUtil) IsConnected() bool { return true }
func (m *mockConnectorForUtil) GetOS(ctx context.Context) (*connector.OS, error) { return &connector.OS{}, nil }
func (m *mockConnectorForUtil) ReadFile(ctx context.Context, path string) ([]byte, error) { return []byte("test"), nil }
func (m *mockConnectorForUtil) WriteFile(ctx context.Context, content []byte, destPath string, options *connector.FileTransferOptions) error { return nil }
func (m *mockConnectorForUtil) Mkdir(ctx context.Context, path string, perm string) error { return nil }
func (m *mockConnectorForUtil) Remove(ctx context.Context, path string, opts connector.RemoveOptions) error { return nil }
func (m *mockConnectorForUtil) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) { return "", nil }
*/

func TestDownloadFileWithConnector(t *testing.T) {
	ctx := context.Background()

	// Setup a real logger that writes to a temporary file for inspection
	tmpLogDir := t.TempDir() // Creates a temporary directory for the test
	logFilePath := filepath.Join(tmpLogDir, "test_download.log")

	opts := logger.DefaultOptions()
	opts.ConsoleOutput = false // Disable console output for cleaner test logs
	opts.FileOutput = true
	opts.LogFilePath = logFilePath
	opts.FileLevel = logger.InfoLevel // Ensure Info messages are written

	testSpecificLogger, err := logger.NewLogger(opts)
	if err != nil {
		t.Fatalf("Failed to create test-specific logger: %v", err)
	}
	defer testSpecificLogger.Sync()

	// mockConn := &mockConnectorForUtil{} // No longer needed

	url := "http://example.com/file.zip"
	targetDir := "/tmp/downloads"
	targetName := "file.zip"
	checksum := "somesha256"

	t.Run("successful_simulation", func(t *testing.T) {
		// Call without mockConn
		path, err := DownloadFileWithConnector(ctx, testSpecificLogger, url, targetDir, targetName, checksum)
		if err != nil {
			t.Errorf("DownloadFileWithConnector returned error: %v", err)
		}
		expectedPath := filepath.Join(targetDir, targetName)
		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}

		testSpecificLogger.Sync()
		logContent, readErr := os.ReadFile(logFilePath)
		if readErr != nil {
			t.Fatalf("Failed to read log file %s: %v", logFilePath, readErr)
		}

		logString := string(logContent)
		expectedLogSubstringBase := "Placeholder: DownloadFileWithConnector called"
		if !strings.Contains(logString, expectedLogSubstringBase) {
			t.Errorf("Log content missing expected substring base %q. Log was: %s", expectedLogSubstringBase, logString)
		}
		if !strings.Contains(logString, fmt.Sprintf("URL: %s", url)) {
			t.Errorf("Log content missing URL part. Log was: %s", logString)
		}
		if !strings.Contains(logString, fmt.Sprintf("TargetDir: %s", targetDir)) {
			t.Errorf("Log content missing TargetDir part. Log was: %s", logString)
		}
		if !strings.Contains(logString, fmt.Sprintf("TargetName: %s", targetName)) {
			t.Errorf("Log content missing TargetName part. Log was: %s", logString)
		}
		if !strings.Contains(logString, fmt.Sprintf("Checksum (not verified): %s", checksum)) {
			t.Errorf("Log content missing Checksum part. Log was: %s", logString)
		}
	})

	// The "nil_connector" subtest is no longer relevant as the connector parameter was removed.
	// t.Run("nil_connector", func(t *testing.T) { ... })
}
