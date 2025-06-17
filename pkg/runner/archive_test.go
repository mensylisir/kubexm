package runner

import (
	"context"
	"errors"
	"fmt"
	// "os" // Not directly used in these mocks, but often in real tests
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for archive tests
func newTestRunnerForArchive(t *testing.T) (*Runner, *MockConnector) {
	mockConn := NewMockConnector()
	// Default GetOS for NewRunner
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// Default Exec for NewRunner fact gathering
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "hostname") { return []byte("test-host"), nil, nil }
		if strings.Contains(cmd, "uname -r") { return []byte("test-kernel"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024"), nil, nil } // 1MB
		if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
		// Fallback for other commands (like LookPath fallbacks in the actual functions)
		// This is tricky because LookPath is a connector method, not an Exec.
		// The ExecFunc here is for commands run *by the runner methods*, not by connector's LookPath.
		return []byte("default exec output for archive test setup"), nil, nil
	}
	// Default LookPath for NewRunner (if it were to use it, though it doesn't directly)
	// and for the archive functions themselves.
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		// Provide defaults for commands used in NewRunner if they were looked up (they aren't)
		// More importantly, provide for curl, wget, tar, unzip, mkdir, rm
		switch file {
		case "curl", "wget", "tar", "unzip", "mkdir", "rm", "cat", "hostname", "uname", "nproc", "grep", "awk", "ip":
			return "/usr/bin/" + file, nil
		default:
			return "", fmt.Errorf("LookPath mock: command %s not found", file)
		}
	}

	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("Failed to create runner for archive tests: %v", err)
	}
	return r, mockConn
}


func TestRunner_Download_Success_Curl(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)
	url := "http://example.com/file.zip"
	destPath := "/tmp/file.zip"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "curl" {
			return "/usr/bin/curl", nil
		}
		if file == "wget" { // Should not be called if curl is found
			t.Error("wget LookPath called when curl should have been found")
			return "", errors.New("wget not expected")
		}
		return "", fmt.Errorf("unexpected LookPath call: %s", file)
	}

	var downloadCmdCalled string
	// Override ExecFunc for this specific test
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		downloadCmdCalled = cmd
		if !strings.Contains(cmd, "curl -sSL -o") || !strings.Contains(cmd, destPath) || !strings.Contains(cmd, url) {
			t.Errorf("Download command with curl is incorrect: %s", cmd)
		}
		if options.Sudo { // Test case passes sudo: false
			t.Error("Download with curl expected sudo to be false based on test call")
		}
		return nil, nil, nil
	}

	err := r.Download(context.Background(), url, destPath, false)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !strings.Contains(downloadCmdCalled, "curl") {
		t.Errorf("Download did not use curl. Command was: %s", downloadCmdCalled)
	}
}

func TestRunner_Download_Success_Wget(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)
	url := "http://example.com/file.tar.gz"
	destPath := "/tmp/file.tar.gz"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "curl" {
			return "", errors.New("curl not found") // Simulate curl not found
		}
		if file == "wget" {
			return "/usr/bin/wget", nil
		}
		return "", fmt.Errorf("unexpected LookPath call: %s", file)
	}

	var downloadCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		downloadCmdCalled = cmd
		if !strings.Contains(cmd, "wget -qO") || !strings.Contains(cmd, destPath) || !strings.Contains(cmd, url) {
			t.Errorf("Download command with wget is incorrect: %s", cmd)
		}
		if !options.Sudo { // Test case passes sudo: true
			t.Error("Download with wget expected sudo to be true based on test call")
		}
		return nil, nil, nil
	}

	err := r.Download(context.Background(), url, destPath, true) // Sudo true
	if err != nil {
		t.Fatalf("Download() with wget error = %v", err)
	}
	if !strings.Contains(downloadCmdCalled, "wget") {
		t.Errorf("Download did not use wget when curl was not found. Command was: %s", downloadCmdCalled)
	}
}

func TestRunner_Download_Fail_NoTool(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		return "", errors.New(file + " not found") // Both curl and wget not found
	}

	err := r.Download(context.Background(), "url", "dest", false)
	if err == nil {
		t.Fatal("Download() expected error when no download tool is found, got nil")
	}
	if !strings.Contains(err.Error(), "neither curl nor wget found") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}

func TestRunner_Extract_TarGz(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)
	archivePath := "/tmp/archive.tar.gz"
	destDir := "/opt/extracted"

	var extractCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		extractCmdCalled = cmd
		expectedCmdPart := fmt.Sprintf("tar -xzf %s -C %s", archivePath, destDir)
		if !strings.Contains(cmd, expectedCmdPart) {
			t.Errorf("Extract command for .tar.gz is incorrect: %s, expected contains %s", cmd, expectedCmdPart)
		}
		return nil, nil, nil
	}

	err := r.Extract(context.Background(), archivePath, destDir, false)
	if err != nil {
		t.Fatalf("Extract() for .tar.gz error = %v", err)
	}
	if !strings.Contains(extractCmdCalled, "tar -xzf") {
		t.Errorf("Extract() for .tar.gz did not use correct tar command. Got: %s", extractCmdCalled)
	}
}

func TestRunner_Extract_Zip(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)
	archivePath := "/tmp/archive.zip"
	destDir := "/opt/extracted_zip"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "unzip" { return "/usr/bin/unzip", nil }
		return "/usr/bin/"+file, nil // Default for other tools
	}

	var extractCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		extractCmdCalled = cmd
		expectedCmdPart := fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
		if !strings.Contains(cmd, expectedCmdPart) {
			t.Errorf("Extract command for .zip is incorrect: %s", cmd)
		}
		return nil, nil, nil
	}

	err := r.Extract(context.Background(), archivePath, destDir, true) // sudo true
	if err != nil {
		t.Fatalf("Extract() for .zip error = %v", err)
	}
	if !strings.Contains(extractCmdCalled, "unzip -o") {
		t.Errorf("Extract() for .zip did not use correct unzip command. Got: %s", extractCmdCalled)
	}
}

func TestRunner_Extract_Unsupported(t *testing.T) {
	r, _ := newTestRunnerForArchive(t)
	err := r.Extract(context.Background(), "/tmp/archive.rar", "/dest", false)
	if err == nil {
		t.Fatal("Extract() expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported archive format") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}


func TestRunner_DownloadAndExtract_Success(t *testing.T) {
	r, mockConn := newTestRunnerForArchive(t)

	url := "http://example.com/myarchive.tar.gz"
	destDir := "/opt/final_dest"
	// expectedTempPath is tricky if filepath.Join is used with a mock FS.
	// The logic in DownloadAndExtract uses filepath.Join("/tmp", safeArchiveName)
	// For testing, we can replicate that or make it predictable.
	archiveName := filepath.Base(url)
	safeArchiveName := strings.ReplaceAll(archiveName, "/", "_")
	safeArchiveName = strings.ReplaceAll(safeArchiveName, "..", "_")
	expectedTempPath := filepath.Join("/tmp", safeArchiveName)


	// Reset LookPath for this specific test's needs
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "curl": return "/usr/bin/curl", nil
		case "mkdir": return "/bin/mkdir", nil // For Mkdirp
		case "chmod": return "/bin/chmod", nil // For Mkdirp (if permissions set)
		case "rm": return "/bin/rm", nil // For Remove
		case "tar": return "/bin/tar", nil // For Extract
		default: // For NewRunner's internal fact gathering
			if file == "hostname" || file == "uname" || file == "nproc" || file == "grep" || file == "awk" || file == "ip" || file == "cat" {
				return "/usr/bin/"+file, nil
			}
			return "", fmt.Errorf("DownloadAndExtract LookPath: unexpected tool %s", file)
		}
	}

	var commandsExecuted []string
	// Override ExecFunc to capture all commands and validate them
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		commandsExecuted = append(commandsExecuted, cmd)

		// Download command
		if strings.Contains(cmd, "curl -sSL -o "+expectedTempPath) && strings.Contains(cmd, url) {
			return nil, nil, nil
		}
		// Mkdirp command for destDir
		if strings.Contains(cmd, "mkdir -p "+destDir) {
			return nil, nil, nil
		}
		// Chmod for Mkdirp (if permissions were set, the test calls with "0755")
		if strings.Contains(cmd, "chmod 0755 "+destDir){
			return nil, nil, nil
		}
		// Extract command
		if strings.Contains(cmd, fmt.Sprintf("tar -xzf %s -C %s", expectedTempPath, destDir)) {
			return nil, nil, nil
		}
		// Remove command for cleanup
		if strings.Contains(cmd, fmt.Sprintf("rm -rf %s", expectedTempPath)) {
			return nil, nil, nil
		}
		// Allow NewRunner's fact-gathering commands
		if strings.Contains(cmd, "hostname") || strings.Contains(cmd, "uname -r") || strings.Contains(cmd, "nproc") || strings.Contains(cmd, "grep MemTotal") || strings.Contains(cmd, "ip -4 route") || strings.Contains(cmd, "ip -6 route"){
			return []byte("dummy"), nil, nil
		}

		return nil, nil, fmt.Errorf("DownloadAndExtract Exec: unexpected command '%s'", cmd)
	}

	err := r.DownloadAndExtract(context.Background(), url, destDir, false) // sudo false
	if err != nil {
		t.Fatalf("DownloadAndExtract() error = %v. Executed commands: %v", err, commandsExecuted)
	}

	foundDownload := false
	foundMkdirp := false
	foundExtract := false
	foundRemove := false
	for _, cmd := range commandsExecuted {
		if strings.Contains(cmd, "curl -sSL -o "+expectedTempPath) { foundDownload = true }
		if strings.Contains(cmd, "mkdir -p "+destDir) { foundMkdirp = true }
		if strings.Contains(cmd, "tar -xzf "+expectedTempPath) { foundExtract = true }
		if strings.Contains(cmd, "rm -rf "+expectedTempPath) { foundRemove = true }
	}
	if !foundDownload { t.Error("Download command not executed in DownloadAndExtract") }
	if !foundMkdirp { t.Error("Mkdirp command for destDir not executed in DownloadAndExtract") }
	if !foundExtract { t.Error("Extract command not executed in DownloadAndExtract") }
	if !foundRemove { t.Error("Remove command for cleanup not executed in DownloadAndExtract") }
}
