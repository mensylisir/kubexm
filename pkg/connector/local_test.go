package connector

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLocalConnector_Connect(t *testing.T) {
	lc := &LocalConnector{}
	cfg := ConnectionCfg{User: "testuser"} // Example config
	err := lc.Connect(context.Background(), cfg)
	if err != nil {
		t.Errorf("LocalConnector.Connect() error = %v, wantErr nil", err)
	}
	if !lc.IsConnected() {
		t.Errorf("LocalConnector.IsConnected() = false, want true after Connect")
	}
}

func TestLocalConnector_Exec_Simple(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "echo hello"
	} else {
		cmdStr = "echo hello"
	}

	stdout, stderr, err := lc.Exec(ctx, cmdStr, nil)
	if err != nil {
		t.Fatalf("LocalConnector.Exec() error = %v", err)
	}
	if strings.TrimSpace(string(stdout)) != "hello" {
		t.Errorf("stdout = %q, want %q", string(stdout), "hello")
	}
	if string(stderr) != "" {
		t.Errorf("stderr = %q, want empty", string(stderr))
	}
}

func TestLocalConnector_Exec_Error(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	// Using a command that is unlikely to exist or will fail
	cmdStr := "command_that_should_fail_hopefully -invalidoption"

	_, _, err := lc.Exec(ctx, cmdStr, nil)
	if err == nil {
		t.Fatalf("LocalConnector.Exec() with failing command expected error, got nil")
	}
	if _, ok := err.(*CommandError); !ok {
		t.Errorf("Expected CommandError, got %T", err)
	}
}

func TestLocalConnector_Exec_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping timeout test on Windows due to differences in process handling")
	}
	lc := &LocalConnector{}
	ctx := context.Background()
	opts := &ExecOptions{Timeout: 100 * time.Millisecond}
	// Command that sleeps longer than timeout
	cmdStr := "sleep 1"

	_, _, err := lc.Exec(ctx, cmdStr, opts)
	if err == nil {
		t.Fatalf("LocalConnector.Exec() with timeout expected error, got nil")
	}
	// The error might be context.DeadlineExceeded wrapped in CommandError or directly
	t.Logf("Timeout error: %v", err) // Log error for inspection
	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "signal: killed") {
         // Depending on OS and shell, error message might vary slightly.
         // For CommandError, the Underlying error would be context.DeadlineExceeded.
         if cmdErr, ok := err.(*CommandError); ok {
             if cmdErr.Underlying != context.DeadlineExceeded && !strings.Contains(cmdErr.Underlying.Error(), "signal: killed") {
                 t.Errorf("Expected error to contain 'deadline exceeded' or 'signal: killed', got %v", cmdErr.Underlying)
             }
         } else {
            t.Errorf("Expected error to contain 'deadline exceeded' or 'signal: killed', got %v", err)
         }
	}
}


func TestLocalConnector_FileOperations(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()

	// Create a temporary directory for testing file operations
	tmpDir, err := os.MkdirTemp("", "localconnector-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFileName := "source.txt"
	dstFileName := "destination.txt"
	fetchFileName := "fetched.txt"
	contentFileName := "content.txt"

	srcFilePath := filepath.Join(tmpDir, srcFileName)
	dstFilePath := filepath.Join(tmpDir, dstFileName)
	fetchFilePath := filepath.Join(tmpDir, fetchFileName)
	contentFilePath := filepath.Join(tmpDir, contentFileName)

	fileContent := []byte("Hello, LocalConnector!")

	// 1. Test CopyContent
	err = lc.CopyContent(ctx, fileContent, contentFilePath, &FileTransferOptions{Permissions: "0644"})
	if err != nil {
		t.Fatalf("CopyContent() error = %v", err)
	}
	readContent, _ := os.ReadFile(contentFilePath)
	if string(readContent) != string(fileContent) {
		t.Errorf("CopyContent() content mismatch: got %q, want %q", string(readContent), string(fileContent))
	}
	statContent, _ := os.Stat(contentFilePath)
	if runtime.GOOS != "windows" && statContent.Mode().Perm() != 0644 { // Windows permissions are different
		t.Errorf("CopyContent() permissions mismatch: got %s, want 0644", statContent.Mode().Perm())
	}


	// Create source file for Copy and Fetch
	err = os.WriteFile(srcFilePath, fileContent, 0666)
	if err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// 2. Test Copy
	err = lc.Copy(ctx, srcFilePath, dstFilePath, &FileTransferOptions{Permissions: "0600"})
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	readDst, _ := os.ReadFile(dstFilePath)
	if string(readDst) != string(fileContent) {
		t.Errorf("Copy() content mismatch: got %q, want %q", string(readDst), string(fileContent))
	}
	statDst, _ := os.Stat(dstFilePath)
	if runtime.GOOS != "windows" && statDst.Mode().Perm() != 0600 {
		t.Errorf("Copy() permissions mismatch: got %s, want 0600", statDst.Mode().Perm())
	}

	// 3. Test Fetch (which uses Copy internally for local)
	err = lc.Fetch(ctx, dstFilePath, fetchFilePath)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	readFetch, _ := os.ReadFile(fetchFilePath)
	if string(readFetch) != string(fileContent) {
		t.Errorf("Fetch() content mismatch: got %q, want %q", string(readFetch), string(fileContent))
	}

	// 4. Test Stat
	fileStat, err := lc.Stat(ctx, srcFilePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !fileStat.IsExist {
		t.Errorf("Stat() file %s should exist", srcFilePath)
	}
	if fileStat.Name != srcFileName {
		t.Errorf("Stat() name mismatch: got %s, want %s", fileStat.Name, srcFileName)
	}
	if fileStat.Size != int64(len(fileContent)) {
		t.Errorf("Stat() size mismatch: got %d, want %d", fileStat.Size, len(fileContent))
	}

	nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")
	fileStatNE, err := lc.Stat(ctx, nonExistentPath)
	if err != nil {
		t.Fatalf("Stat() for non-existent file error = %v", err)
	}
	if fileStatNE.IsExist {
		t.Errorf("Stat() file %s should not exist", nonExistentPath)
	}
}

func TestLocalConnector_LookPath(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()

	var executableName string
	if runtime.GOOS == "windows" {
		executableName = "cmd.exe" // cmd should be in PATH on Windows
	} else {
		executableName = "sh" // sh should be in PATH on Unix-like systems
	}

	path, err := lc.LookPath(ctx, executableName)
	if err != nil {
		t.Fatalf("LookPath(%q) error = %v", executableName, err)
	}
	if path == "" {
		t.Errorf("LookPath(%q) returned empty path", executableName)
	}
	t.Logf("Found %s at %s", executableName, path)

	_, err = lc.LookPath(ctx, "non_existent_executable_sfdhjskfh")
	if err == nil {
		t.Errorf("LookPath() for non-existent executable expected error, got nil")
	}
}

func TestLocalConnector_GetOS(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()

	osInfo, err := lc.GetOS(ctx)
	if err != nil {
		t.Fatalf("GetOS() error = %v", err)
	}

	if osInfo == nil {
		t.Fatal("GetOS() returned nil OS info")
	}

	t.Logf("Detected OS: ID=%s, VersionID=%s, Codename=%s, Arch=%s, Kernel=%s",
		osInfo.ID, osInfo.VersionID, osInfo.Codename, osInfo.Arch, osInfo.Kernel)

	if osInfo.Arch == "" {
		t.Error("GetOS() returned empty Arch")
	}
	// Basic check for ID based on runtime.GOOS
	// More specific checks might be needed if detailed parsing is critical
	expectedID := strings.ToLower(runtime.GOOS)
	if !strings.Contains(osInfo.ID, expectedID) && osInfo.ID != "raspbian" { // Raspbian reports ID=raspbian
		// Allow for variations like "ubuntu" for "linux"
		if expectedID == "linux" && (osInfo.ID == "ubuntu" || osInfo.ID == "centos" || osInfo.ID == "debian" || osInfo.ID == "rhel" || osInfo.ID == "arch") {
			// This is fine
		} else {
			t.Errorf("GetOS() ID: got %s, expected to contain %s (or be a known Linux distro)", osInfo.ID, expectedID)
		}
	}


	// Test caching
	osInfo2, err := lc.GetOS(ctx)
	if err != nil {
		t.Fatalf("GetOS() second call error = %v", err)
	}
	if osInfo != osInfo2 { // Should be the same cached pointer
		t.Error("GetOS() caching failed; returned different struct pointers")
	}
}
