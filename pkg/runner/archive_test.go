package runner

import (
	"context"
	"errors"
	"fmt"
	// "os" // Not directly used in these mocks, but often in real tests
	"path/filepath"
	"strings"
	"testing"
	// "time" // Removed

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for archive tests
func newTestRunnerForArchive(t *testing.T) (Runner, *Facts, *MockConnector) {
	mockConn := NewMockConnector()
	// Default GetOS for GatherFacts
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// Default Exec for GatherFacts
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if mockConn.ExecHistory == nil {
			mockConn.ExecHistory = []string{}
		}
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		if strings.Contains(cmd, "hostname") { return []byte("test-host"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil }
		if strings.Contains(cmd, "ip -4 route get 8.8.8.8") { return []byte("8.8.8.8 dev eth0 src 1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route get") { return nil, nil, fmt.Errorf("no ipv6") }
		if strings.Contains(cmd, "command -v apt-get") { return []byte("/usr/bin/apt-get"), nil, nil}
		if strings.Contains(cmd, "command -v yum") { return []byte(""), nil, errors.New("not found")}
		if strings.Contains(cmd, "command -v dnf") { return []byte(""), nil, errors.New("not found")}
		if strings.Contains(cmd, "command -v systemctl") { return []byte("/usr/bin/systemctl"), nil, nil}
		if strings.Contains(cmd, "command -v service") { return []byte(""), nil, errors.New("not found")}
		if strings.HasPrefix(cmd, "test -e /etc/init.d") { return nil, nil, errors.New("no /etc/init.d for this mock")}

		return []byte("default exec output for fact gathering"), nil, nil
	}
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "curl", "wget", "tar", "unzip", "mkdir", "rm", "cat", "hostname", "uname", "nproc", "grep", "awk", "ip", "systemctl", "apt-get", "service":
			return "/usr/bin/" + file, nil
		default:
			return "", fmt.Errorf("LookPath mock: command %s not found by default", file)
		}
	}

	r := NewRunner()
	facts, err := r.GatherFacts(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("newTestRunnerForArchive: Failed to gather facts: %v", err)
	}
	if facts == nil {
		t.Fatalf("newTestRunnerForArchive: GatherFacts returned nil facts")
	}
	return r, facts, mockConn
}


func TestRunner_Download_Success_Curl(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	url := "http://example.com/file.zip"
	destPath := "/tmp/file.zip"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "curl" {
			return "/usr/bin/curl", nil
		}
		if file == "wget" {
			t.Error("wget LookPath called when curl should have been found")
			return "", errors.New("wget not expected")
		}
		if isFactGatheringCommandLookupForArchive(file) {return "/usr/bin/"+file, nil}
		return "", fmt.Errorf("unexpected LookPath call: %s", file)
	}

	var downloadCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		downloadCmdCalled = cmd
		if !strings.Contains(cmd, "curl -sSL -o") || !strings.Contains(cmd, destPath) || !strings.Contains(cmd, url) {
			t.Errorf("Download command with curl is incorrect: %s", cmd)
		}
		if options.Sudo {
			t.Error("Download with curl expected sudo to be false based on test call")
		}
		return nil, nil, nil
	}

	err := r.Download(context.Background(), mockConn, facts, url, destPath, false)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !strings.Contains(downloadCmdCalled, "curl") {
		t.Errorf("Download did not use curl. Command was: %s", downloadCmdCalled)
	}
}

func TestRunner_Download_Success_Wget(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	url := "http://example.com/file.tar.gz"
	destPath := "/tmp/file.tar.gz"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "curl" {
			return "", errors.New("curl not found")
		}
		if file == "wget" {
			return "/usr/bin/wget", nil
		}
		if isFactGatheringCommandLookupForArchive(file) {return "/usr/bin/"+file, nil}
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
		if !options.Sudo {
			t.Error("Download with wget expected sudo to be true based on test call")
		}
		return nil, nil, nil
	}

	err := r.Download(context.Background(), mockConn, facts, url, destPath, true) // Sudo true
	if err != nil {
		t.Fatalf("Download() with wget error = %v", err)
	}
	if !strings.Contains(downloadCmdCalled, "wget") {
		t.Errorf("Download did not use wget when curl was not found. Command was: %s", downloadCmdCalled)
	}
}

func TestRunner_Download_Fail_NoTool(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "curl" || file == "wget" {
			return "", errors.New(file + " not found")
		}
		if isFactGatheringCommandLookupForArchive(file) {return "/usr/bin/"+file, nil}
		return "", fmt.Errorf("unexpected LookPath call for %s", file)
	}

	err := r.Download(context.Background(), mockConn, facts, "url", "dest", false)
	if err == nil {
		t.Fatal("Download() expected error when no download tool is found, got nil")
	}
	if !strings.Contains(err.Error(), "neither curl nor wget found") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}

func TestRunner_Extract_TarGz(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	archivePath := "/tmp/archive.tar.gz"
	destDir := "/opt/extracted"

	var extractCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInArchiveTest(cmd) {return []byte("dummy"), nil, nil}
		extractCmdCalled = cmd
		expectedCmdPart := fmt.Sprintf("tar -xzf %s -C %s", archivePath, destDir)
		if !strings.Contains(cmd, expectedCmdPart) {
			t.Errorf("Extract command for .tar.gz is incorrect: %s, expected contains %s", cmd, expectedCmdPart)
		}
		return nil, nil, nil
	}

	err := r.Extract(context.Background(), mockConn, facts, archivePath, destDir, false)
	if err != nil {
		t.Fatalf("Extract() for .tar.gz error = %v", err)
	}
	if !strings.Contains(extractCmdCalled, "tar -xzf") {
		t.Errorf("Extract() for .tar.gz did not use correct tar command. Got: %s", extractCmdCalled)
	}
}

func TestRunner_Extract_Zip(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	archivePath := "/tmp/archive.zip"
	destDir := "/opt/extracted_zip"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "unzip" { return "/usr/bin/unzip", nil }
		if isFactGatheringCommandLookupForArchive(file) {return "/usr/bin/"+file, nil}
		return "", fmt.Errorf("LookPath mock in Extract_Zip: command %s not found by default", file)
	}


	var extractCmdCalled string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInArchiveTest(cmd) {return []byte("dummy"), nil, nil}
		extractCmdCalled = cmd
		expectedCmdPart := fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
		if !strings.Contains(cmd, expectedCmdPart) {
			t.Errorf("Extract command for .zip is incorrect: %s", cmd)
		}
		return nil, nil, nil
	}

	err := r.Extract(context.Background(), mockConn, facts, archivePath, destDir, true) // sudo true
	if err != nil {
		t.Fatalf("Extract() for .zip error = %v", err)
	}
	if !strings.Contains(extractCmdCalled, "unzip -o") {
		t.Errorf("Extract() for .zip did not use correct unzip command. Got: %s", extractCmdCalled)
	}
}

func TestRunner_Extract_Unsupported(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)
	err := r.Extract(context.Background(), mockConn, facts, "/tmp/archive.rar", "/dest", false)
	if err == nil {
		t.Fatal("Extract() expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported archive format") {
		t.Errorf("Error message mismatch for unsupported format: got %v", err)
	}
}

func TestRunner_Extract_Unsupported_WithValidConn(t *testing.T) {
    r, facts, mockConn := newTestRunnerForArchive(t)
    err := r.Extract(context.Background(), mockConn, facts, "/tmp/archive.rar", "/dest", false)
    if err == nil {
        t.Fatal("Extract() expected error for unsupported format, got nil")
    }
    if !strings.Contains(err.Error(), "unsupported archive format") {
        t.Errorf("Error message mismatch for unsupported format: got %v", err)
    }
}


func TestRunner_DownloadAndExtract_Success(t *testing.T) {
	r, facts, mockConn := newTestRunnerForArchive(t)

	url := "http://example.com/myarchive.tar.gz"
	destDir := "/opt/final_dest"
	archiveName := filepath.Base(url)
	safeArchiveName := strings.ReplaceAll(archiveName, "/", "_")
	safeArchiveName = strings.ReplaceAll(safeArchiveName, "..", "_")
	expectedTempPath := filepath.Join("/tmp", safeArchiveName)


	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "curl": return "/usr/bin/curl", nil
		case "mkdir": return "/bin/mkdir", nil
		case "chmod": return "/bin/chmod", nil
		case "rm": return "/bin/rm", nil
		case "tar": return "/bin/tar", nil
		default:
			if isFactGatheringCommandLookupForArchive(file) {
				return "/usr/bin/"+file, nil
			}
			return "", fmt.Errorf("DownloadAndExtract LookPath: unexpected tool %s", file)
		}
	}

	var commandsExecuted []string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		commandsExecuted = append(commandsExecuted, cmd)

		if strings.Contains(cmd, "curl -sSL -o "+expectedTempPath) && strings.Contains(cmd, url) {
			return nil, nil, nil
		}
		if strings.Contains(cmd, "mkdir -p "+destDir) {
			return nil, nil, nil
		}
		if strings.Contains(cmd, "chmod 0755 "+destDir){
			return nil, nil, nil
		}
		if strings.Contains(cmd, fmt.Sprintf("tar -xzf %s -C %s", expectedTempPath, destDir)) {
			return nil, nil, nil
		}
		if strings.Contains(cmd, fmt.Sprintf("rm -rf %s", expectedTempPath)) {
			return nil, nil, nil
		}
		if isExecCmdForFactsInArchiveTest(cmd){
			return []byte("dummy"), nil, nil
		}

		return nil, nil, fmt.Errorf("DownloadAndExtract Exec: unexpected command '%s'", cmd)
	}

	err := r.DownloadAndExtract(context.Background(), mockConn, facts, url, destDir, false)
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

// Helper for LookPath in archive tests, to distinguish from other test files' helpers
func isFactGatheringCommandLookupForArchive(file string) bool {
	switch file {
	case "hostname", "uname", "nproc", "grep", "awk", "ip", "cat", "test", "command", "systemctl", "apt-get", "service", "yum", "dnf":
		return true
	default:
		return false
	}
}

// Helper for ExecFunc in archive tests
func isExecCmdForFactsInArchiveTest(cmd string) bool {
	return strings.Contains(cmd, "hostname") ||
		strings.Contains(cmd, "uname -r") ||
		strings.Contains(cmd, "nproc") ||
		strings.Contains(cmd, "grep MemTotal") ||
		strings.Contains(cmd, "ip -4 route") ||
		strings.Contains(cmd, "ip -6 route") ||
		strings.Contains(cmd, "command -v") ||
		strings.Contains(cmd, "test -e /etc/init.d")
}
