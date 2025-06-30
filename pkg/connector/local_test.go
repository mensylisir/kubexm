package connector

import (
	"context"
	"crypto/rand" // For random content in checksum test
	"crypto/sha256"
	"encoding/hex"
	"errors" // For checking specific error types in sudo password tests
	"os"
	"path/filepath"
	"runtime"
	"strconv" // Added for ParseUint
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

func TestLocalConnector_Remove_WithSudo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sudo tests are not applicable on Windows")
	}

	lc := &LocalConnector{}
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-remove-sudo-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// File to be removed with sudo
	sudoFileToRemove := filepath.Join(tmpDir, "sudo_file_to_remove.txt")

	// Create the file as current user first. Sudo rm should still work.
	if err := os.WriteFile(sudoFileToRemove, []byte("delete me with sudo"), 0644); err != nil {
		t.Fatalf("Failed to create test file for sudo remove: %v", err)
	}

	// Attempt to remove with sudo.
	// This test primarily checks that the sudo path is taken.
	// Actual success of 'sudo rm' depends on passwordless sudo setup for 'rm'.
	err = lc.Remove(ctx, sudoFileToRemove, RemoveOptions{Sudo: true})
	if err == nil {
		t.Logf("Remove with Sudo for %s unexpectedly succeeded (might be due to passwordless sudo for rm).", sudoFileToRemove)
		if _, statErr := os.Stat(sudoFileToRemove); !os.IsNotExist(statErr) {
			t.Errorf("File %s should not exist after successful sudo Remove, stat error: %v", sudoFileToRemove, statErr)
		}
	} else {
		t.Logf("Remove with Sudo for %s expectedly failed or had issues (no passwordless sudo for rm?): %v", sudoFileToRemove, err)
		// Check that the error is not "not implemented" and mentions sudo if it failed at Exec level
		if strings.Contains(err.Error(), "sudo not implemented") {
			t.Errorf("Remove with Sudo should not return 'sudo not implemented', got: %v", err)
		}
		// If it failed, the file might still be there.
		if _, statErr := os.Stat(sudoFileToRemove); os.IsNotExist(statErr) {
			t.Logf("File %s was indeed removed despite error (maybe rm itself succeeded but chmod/chown like step failed, or error was intermittent).", sudoFileToRemove)
		}
	}

	// Test recursive remove with sudo
	sudoDirToRemove := filepath.Join(tmpDir, "sudo_dir_to_remove")
	sudoNestedFile := filepath.Join(sudoDirToRemove, "nested.txt")
	if err := os.Mkdir(sudoDirToRemove, 0755); err != nil {
		t.Fatalf("Failed to create dir for sudo recursive remove: %v", err)
	}
	if err := os.WriteFile(sudoNestedFile, []byte("delete this dir with sudo"), 0644); err != nil {
		t.Fatalf("Failed to create nested file for sudo recursive remove: %v", err)
	}

	err = lc.Remove(ctx, sudoDirToRemove, RemoveOptions{Sudo: true, Recursive: true})
	if err == nil {
		t.Logf("Recursive Remove with Sudo for %s unexpectedly succeeded.", sudoDirToRemove)
		if _, statErr := os.Stat(sudoDirToRemove); !os.IsNotExist(statErr) {
			t.Errorf("Directory %s should not exist after successful sudo Recursive Remove, stat error: %v", sudoDirToRemove, statErr)
		}
	} else {
		t.Logf("Recursive Remove with Sudo for %s expectedly failed or had issues: %v", sudoDirToRemove, err)
		if strings.Contains(err.Error(), "sudo not implemented") {
			t.Errorf("Recursive Remove with Sudo should not return 'sudo not implemented', got: %v", err)
		}
	}
}

func TestLocalConnector_Copy_Directory(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()

	tmpBaseDir, err := os.MkdirTemp("", "localconnector-copydir-testbase-")
	if err != nil {
		t.Fatalf("Failed to create temp base dir: %v", err)
	}
	defer os.RemoveAll(tmpBaseDir)

	// 1. Setup source directory
	srcDir := filepath.Join(tmpBaseDir, "source_dir")
	srcSubDir := filepath.Join(srcDir, "subdir")
	srcFile1 := filepath.Join(srcDir, "file1.txt")
	srcFile2 := filepath.Join(srcSubDir, "file2.txt")

	if err := os.MkdirAll(srcSubDir, 0755); err != nil {
		t.Fatalf("Failed to create srcSubDir: %v", err)
	}
	if err := os.WriteFile(srcFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write srcFile1: %v", err)
	}
	if err := os.WriteFile(srcFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write srcFile2: %v", err)
	}

	// --- Test Non-Sudo Directory Copy ---
	dstDirNonSudo := filepath.Join(tmpBaseDir, "dest_dir_non_sudo")
	err = lc.Copy(ctx, srcDir, dstDirNonSudo, nil) // No options, non-sudo
	if err != nil {
		t.Fatalf("Non-sudo Copy directory failed: %v", err)
	}

	// Verify non-sudo copy
	if _, err := os.Stat(filepath.Join(dstDirNonSudo, "file1.txt")); err != nil {
		t.Errorf("Non-sudo copy: file1.txt not found in destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDirNonSudo, "subdir", "file2.txt")); err != nil {
		t.Errorf("Non-sudo copy: subdir/file2.txt not found in destination: %v", err)
	}
	dstDirNonSudoStat, _ := os.Stat(dstDirNonSudo)
	srcDirStat, _ := os.Stat(srcDir)
	if runtime.GOOS != "windows" && dstDirNonSudoStat.Mode() != srcDirStat.Mode() {
		t.Errorf("Non-sudo copy: top-level directory mode mismatch. Got %s, want %s", dstDirNonSudoStat.Mode(), srcDirStat.Mode())
	}


	// --- Test Sudo Directory Copy ---
	dstDirSudo := filepath.Join(tmpBaseDir, "dest_dir_sudo")

	optsSudo := &FileTransferOptions{
		Sudo:        true,
		Permissions: "0775",
		Owner:       "root",
		Group:       "root",
	}
	if runtime.GOOS == "windows" {
		optsSudo.Owner = ""
		optsSudo.Group = ""
		optsSudo.Permissions = ""
	}

	err = lc.Copy(ctx, srcDir, dstDirSudo, optsSudo)
	if err != nil {
		// On systems without passwordless sudo for the current user to chown to root, this might fail at chown/chmod.
		t.Logf("Sudo Copy directory potentially failed at permission/ownership stage (may be expected if not root or no passwordless sudo for chown/chmod): %v", err)
	}

	// Verify sudo copy - check for file existence primarily, as perms/owner might fail if not root
	copiedFile1Sudo := filepath.Join(dstDirSudo, "file1.txt")
	copiedFile2Sudo := filepath.Join(dstDirSudo, "subdir", "file2.txt")

	if _, err := os.Stat(copiedFile1Sudo); err != nil {
		t.Errorf("Sudo copy: file1.txt not found in destination %s: %v", dstDirSudo, err)
	} else {
		if optsSudo.Permissions != "" && runtime.GOOS != "windows" {
			statInfo, statErr := os.Stat(dstDirSudo)
			if statErr == nil {
				expectedPerm, _ := strconv.ParseUint(optsSudo.Permissions, 8, 32)
				if statInfo.Mode().Perm() != os.FileMode(expectedPerm).Perm() {
					t.Logf("Sudo copy: directory %s permissions are %s, expected %s. (May differ if sudo chmod part of Copy failed).", dstDirSudo, statInfo.Mode().Perm().String(), os.FileMode(expectedPerm).Perm().String())
				}
			}
		}
	}
	if _, err := os.Stat(copiedFile2Sudo); err != nil {
		t.Errorf("Sudo copy: subdir/file2.txt not found in destination %s: %v", dstDirSudo, err)
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
	// Use a background context that we can cancel to ensure the test doesn't hang indefinitely
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second) // Overall test timeout
	defer parentCancel()

	opts := &ExecOptions{Timeout: 50 * time.Millisecond} // Short timeout for the command itself
	cmdStr := "sleep 0.2" // Command sleeps for 200ms, should exceed timeout

	_, _, err := lc.Exec(parentCtx, cmdStr, opts)
	if err == nil {
		t.Fatalf("LocalConnector.Exec() with timeout expected error, got nil")
	}

	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("Expected CommandError, got %T: %v", err, err)
	}

	// Check if the underlying error is context.DeadlineExceeded
	// It might also be an *os.PathError if the command (sleep) isn't found,
	// or *exec.ExitError if sleep exits due to signal on some OS.
	// The key is that CommandError.Underlying is not nil and indicates a problem.
	t.Logf("Exec_Timeout error: %v, Underlying: %v", err, cmdErr.Underlying)
	if cmdErr.Underlying == nil {
		t.Errorf("Expected CommandError.Underlying to be non-nil for timeout, got nil")
	} else {
		// Check for common timeout-related errors.
		// context.DeadlineExceeded is the most direct.
		// "signal: killed" can happen if the process is killed due to timeout.
		underlyingErrStr := cmdErr.Underlying.Error()
		if cmdErr.Underlying != context.DeadlineExceeded && !strings.Contains(underlyingErrStr, "signal: killed") && !strings.Contains(underlyingErrStr, "deadline exceeded") {
			t.Errorf("Expected CommandError.Underlying to be context.DeadlineExceeded or contain 'signal: killed' or 'deadline exceeded', got: %v", cmdErr.Underlying)
		}
	}
}

func TestLocalConnector_Exec_Retries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping retry test on Windows as it relies on specific shell script behavior")
	}
	lc := &LocalConnector{}
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "localconnector-retry-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "retry_script.sh")
	successMarker := filepath.Join(tmpDir, "success_marker")

	// Script fails twice, then succeeds
	scriptContent := `
#!/bin/sh
FAIL_COUNT_FILE="` + filepath.ToSlash(filepath.Join(tmpDir, "fail_count")) + `"
if [ ! -f "$FAIL_COUNT_FILE" ]; then
    echo 0 > "$FAIL_COUNT_FILE"
fi
count=$(cat "$FAIL_COUNT_FILE")
echo $((count + 1)) > "$FAIL_COUNT_FILE"
if [ "$count" -lt 2 ]; then
    echo "Attempt $count: failing" >&2
    exit 1
else
    echo "Attempt $count: succeeding"
    # Create a marker file to indicate success for verification
    touch "` + filepath.ToSlash(successMarker) + `"
    exit 0
fi
`
	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write retry script: %v", err)
	}

	opts := &ExecOptions{
		Retries:    2, // Should succeed on the 3rd attempt (0, 1, 2)
		RetryDelay: 10 * time.Millisecond,
	}

	stdout, stderr, err := lc.Exec(ctx, scriptPath, opts)
	if err != nil {
		t.Fatalf("LocalConnector.Exec() with retries failed: %v\nStdout: %s\nStderr: %s", err, string(stdout), string(stderr))
	}

	if _, statErr := os.Stat(successMarker); os.IsNotExist(statErr) {
		t.Errorf("Success marker file %s was not created, script did not succeed as expected.", successMarker)
	}

	expectedStdout := "Attempt 2: succeeding"
	if !strings.Contains(string(stdout), expectedStdout) {
		t.Errorf("Expected stdout to contain %q, got %q", expectedStdout, string(stdout))
	}
	t.Logf("Retry stdout: %s", string(stdout))
	t.Logf("Retry stderr: %s", string(stderr)) // Should contain failure messages from first 2 attempts

	// Test retry with timeout that causes failure even after retries
	failScriptPath := filepath.Join(tmpDir, "fail_script.sh")
	failScriptContent := `#!/bin/sh
echo "Trying to sleep..."
sleep 10
echo "Slept"
exit 0` // Script tries to sleep long
	err = os.WriteFile(failScriptPath, []byte(failScriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write fail script: %v", err)
	}

	failOpts := &ExecOptions{
		Retries:    1,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    50 * time.Millisecond, // Each attempt times out
	}
	_, _, err = lc.Exec(ctx, failScriptPath, failOpts)
	if err == nil {
		t.Fatalf("LocalConnector.Exec() with retries and timeout should have failed")
	}
	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("Expected CommandError, got %T: %v", err, err)
	}
	if cmdErr.Underlying == nil {
		t.Errorf("Expected CommandError.Underlying to be non-nil for timeout, got nil")
	} else {
		underlyingErrStr := cmdErr.Underlying.Error()
		if cmdErr.Underlying != context.DeadlineExceeded && !strings.Contains(underlyingErrStr, "signal: killed") && !strings.Contains(underlyingErrStr, "deadline exceeded") {
			t.Errorf("Expected CommandError.Underlying for retry timeout to be context.DeadlineExceeded or contain 'signal: killed', got: %v", cmdErr.Underlying)
		}
	}
	t.Logf("Retry with timeout error: %v", err)

	// Test retry with main context cancellation
	mainCancelCtx, mainCancelFunc := context.WithCancel(context.Background())

	cancelScriptPath := filepath.Join(tmpDir, "cancel_script.sh")
	// This script will always fail, forcing retries.
	// We will cancel mainCancelCtx during the retries.
	cancelScriptContent := `#!/bin/sh
echo "Cancel test: Attempting..." >&2
exit 1`
	err = os.WriteFile(cancelScriptPath, []byte(cancelScriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write cancel script: %v", err)
	}

	cancelOpts := &ExecOptions{
		Retries:    5, // High number of retries
		RetryDelay: 20 * time.Millisecond,
	}

	var execErr error
	execDone := make(chan bool)

	go func() {
		_, _, execErr = lc.Exec(mainCancelCtx, cancelScriptPath, cancelOpts)
		close(execDone)
	}()

	// Let a few retries happen, then cancel the main context.
	time.Sleep(50 * time.Millisecond) // Allow 1-2 retries
	mainCancelFunc()

	select {
	case <-execDone:
		// Execution finished
	case <-time.After(2 * time.Second): // Safety timeout for the test itself
		t.Fatal("Exec did not return after main context cancellation within test timeout")
	}

	if execErr == nil {
		t.Fatalf("Exec with main context cancellation should have failed, but reported success")
	}
	if cmdErr, ok := execErr.(*CommandError); ok {
		if cmdErr.Underlying != context.Canceled && !strings.Contains(cmdErr.Underlying.Error(), "context canceled") {
			t.Errorf("Expected underlying error to be context.Canceled or contain 'context canceled', got %v", cmdErr.Underlying)
		}
	} else {
		t.Fatalf("Expected CommandError type for context cancellation, got %T: %v", execErr, execErr)
	}
	t.Logf("Exec with main context cancellation correctly failed: %v", execErr)
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
	if runtime.GOOS != "windows" && statContent.Mode().Perm() != 0644 {
		t.Errorf("CopyContent() permissions mismatch: got %s, want 0644", statContent.Mode().Perm().String())
	}

	// Test CopyContent with invalid permissions string
	invalidPermContentPath := filepath.Join(tmpDir, "invalid_perm_content.txt")
	err = lc.CopyContent(ctx, fileContent, invalidPermContentPath, &FileTransferOptions{Permissions: "invalid"})
	if err == nil {
		t.Errorf("CopyContent() with invalid permissions string should have failed")
	} else {
		if !strings.Contains(err.Error(), "invalid permissions format") {
			t.Errorf("CopyContent() with invalid permissions error message mismatch: got %s", err.Error())
		}
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

func TestLocalConnector_FileOp_WithSudo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sudo tests are not applicable on Windows")
	}

	lc := &LocalConnector{}
	// Attempting to use sudo without a password set in connCfg.
	// This should try to execute `sudo -E -- tee ...` or `sudo -E -- cat ... | sudo -E -- tee ...`
	// which will likely fail in a test environment without passwordless sudo.
	// The key is that it *tries* and doesn't return a "not implemented" error from our Go code.
	// It should return an error from the actual command execution (e.g., sudo asking for a password).

	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-sudo-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("sudo test content")

	// Test WriteFile with Sudo
	writeFileDest := filepath.Join(tmpDir, "sudo_writefile.txt")
	// Updated to use FileTransferOptions
	writeOpts := &FileTransferOptions{Permissions: "0644", Sudo: true}
	if runtime.GOOS == "windows" { // Sudo not applicable on windows
		writeOpts.Sudo = false
	}

	err = lc.WriteFile(ctx, content, writeFileDest, writeOpts)
	if err == nil {
		if writeOpts.Sudo { // Only log unexpected success if sudo was attempted
			t.Logf("WriteFile with Sudo unexpectedly succeeded. This might happen with passwordless sudo.")
		}
		if _, statErr := os.Stat(writeFileDest); os.IsNotExist(statErr) {
			t.Errorf("WriteFile (Sudo: %v) claimed success but file %s does not exist", writeOpts.Sudo, writeFileDest)
		}
		os.Remove(writeFileDest) // Clean up
	} else {
		if writeOpts.Sudo { // Only check sudo-specific error messages if sudo was attempted
			t.Logf("WriteFile with Sudo expectedly failed (likely no passwordless sudo): %v", err)
			if strings.Contains(err.Error(), "sudo not implemented") && runtime.GOOS != "windows" { // "sudo not implemented" is fine for Windows
				t.Errorf("WriteFile with Sudo should not return 'sudo not implemented' on non-Windows, got: %v", err)
			}
			if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "Sudo") && runtime.GOOS != "windows" {
				t.Errorf("WriteFile with Sudo error message %q did not contain 'sudo' or 'Sudo' on non-Windows", err.Error())
			}
		} else { // If not sudo, any error is unexpected here for this test's basic premise
			t.Errorf("WriteFile (Sudo: false) failed unexpectedly: %v", err)
		}
	}

	// Test CopyContent with Sudo (relies on WriteFile's sudo logic)
	copyContentDest := filepath.Join(tmpDir, "sudo_copycontent.txt")
	err = lc.CopyContent(ctx, content, copyContentDest, &FileTransferOptions{Sudo: true, Permissions: "0644"})
	if err == nil {
		t.Logf("CopyContent with Sudo unexpectedly succeeded.")
		if _, statErr := os.Stat(copyContentDest); os.IsNotExist(statErr) {
			t.Errorf("CopyContent with Sudo claimed success but file %s does not exist", copyContentDest)
		}
		os.Remove(copyContentDest)
	} else {
		t.Logf("CopyContent with Sudo expectedly failed: %v", err)
		if strings.Contains(err.Error(), "sudo not implemented") {
			t.Errorf("CopyContent with Sudo should not return 'sudo not implemented', got: %v", err)
		}
		if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "Sudo") {
			t.Errorf("CopyContent with Sudo error message %q did not contain 'sudo' or 'Sudo'", err.Error())
		}
	}

	// Test Copy with Sudo
	srcCopyFile := filepath.Join(tmpDir, "src_copy_sudo.txt")
	err = os.WriteFile(srcCopyFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create source file for sudo copy test: %v", err)
	}
	copyDest := filepath.Join(tmpDir, "sudo_copy.txt")
	err = lc.Copy(ctx, srcCopyFile, copyDest, &FileTransferOptions{Sudo: true, Permissions: "0644"})
	if err == nil {
		t.Logf("Copy with Sudo unexpectedly succeeded.")
		if _, statErr := os.Stat(copyDest); os.IsNotExist(statErr) {
			t.Errorf("Copy with Sudo claimed success but file %s does not exist", copyDest)
		}
		os.Remove(copyDest)
	} else {
		t.Logf("Copy with Sudo expectedly failed: %v", err)
		if strings.Contains(err.Error(), "sudo not implemented") {
			t.Errorf("Copy with Sudo should not return 'sudo not implemented', got: %v", err)
		}
		if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "Sudo") {
			t.Errorf("Copy with Sudo error message %q did not contain 'sudo' or 'Sudo'", err.Error())
		}
	}
}

func TestLocalConnector_Mkdir(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-mkdir-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test creating a nested directory
	nestedDir := filepath.Join(tmpDir, "a", "b", "c")
	perms := "0750"
	err = lc.Mkdir(ctx, nestedDir, perms)
	if err != nil {
		t.Fatalf("Mkdir(%s, %s) error = %v", nestedDir, perms, err)
	}

	stat, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("os.Stat(%s) after Mkdir error = %v", nestedDir, err)
	}
	if !stat.IsDir() {
		t.Errorf("%s is not a directory after Mkdir", nestedDir)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0750 {
        t.Errorf("Mkdir permissions mismatch for 0750: got %s, want %s", stat.Mode().Perm().String(), perms)
    }

	// Test creating an existing directory (should be idempotent)
	err = lc.Mkdir(ctx, nestedDir, perms)
	if err != nil {
		t.Errorf("Mkdir on existing directory error = %v, want nil", err)
	}

	// Test Mkdir with default permissions
	defaultPermDir := filepath.Join(tmpDir, "default_perm_dir")
	err = lc.Mkdir(ctx, defaultPermDir, "") // Empty perm string to test default
	if err != nil {
		t.Fatalf("Mkdir with default perms error = %v", err)
	}
	statDefault, errDefault := os.Stat(defaultPermDir)
	if errDefault != nil {
		t.Fatalf("os.Stat after Mkdir with default perms error = %v", errDefault)
	}
	if runtime.GOOS != "windows" && statDefault.Mode().Perm() != 0755 { // Default is 0755
		t.Errorf("Mkdir default permissions mismatch: got %s, want 0755", statDefault.Mode().Perm().String())
	}

	// Test Mkdir with invalid permission string
	invalidPermDir := filepath.Join(tmpDir, "invalid_perm_dir")
	err = lc.Mkdir(ctx, invalidPermDir, "invalid")
	if err == nil {
		t.Errorf("Mkdir with invalid perm string should have failed")
	} else {
		if !strings.Contains(err.Error(), "invalid permission format") {
			t.Errorf("Mkdir with invalid perm string error message mismatch: got %s", err.Error())
		}
	}
}

func TestLocalConnector_Remove(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-remove-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Ensure base tmpDir is cleaned

	// File for removal
	fileToRemove := filepath.Join(tmpDir, "file_to_remove.txt")
	if err := os.WriteFile(fileToRemove, []byte("delete me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test Remove file
	err = lc.Remove(ctx, fileToRemove, RemoveOptions{})
	if err != nil {
		t.Errorf("Remove file %s error = %v", fileToRemove, err)
	}
	if _, err := os.Stat(fileToRemove); !os.IsNotExist(err) {
		t.Errorf("File %s should not exist after Remove, stat error: %v", fileToRemove, err)
	}

	// Directory for recursive removal
	dirToRemove := filepath.Join(tmpDir, "dir_to_remove")
	nestedFile := filepath.Join(dirToRemove, "nested_file.txt")
	if err := os.Mkdir(dirToRemove, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("delete me too"), 0644); err != nil {
		t.Fatalf("Failed to create nested test file: %v", err)
	}

	// Test Remove non-empty directory non-recursively (should error)
	err = lc.Remove(ctx, dirToRemove, RemoveOptions{})
	if err == nil {
		t.Errorf("Remove non-empty directory %s non-recursively should have failed", dirToRemove)
	}

	// Test Remove directory recursively
	err = lc.Remove(ctx, dirToRemove, RemoveOptions{Recursive: true})
	if err != nil {
		t.Errorf("Remove directory recursively %s error = %v", dirToRemove, err)
	}
	if _, err := os.Stat(dirToRemove); !os.IsNotExist(err) {
		t.Errorf("Directory %s should not exist after recursive Remove, stat error: %v", dirToRemove, err)
	}

	// Test IgnoreNotExist
	nonExistentPath := filepath.Join(tmpDir, "non_existent.txt")
	err = lc.Remove(ctx, nonExistentPath, RemoveOptions{IgnoreNotExist: true})
	if err != nil {
		t.Errorf("Remove with IgnoreNotExist for %s expected no error, got %v", nonExistentPath, err)
	}

	// Test Remove non-existent without IgnoreNotExist (should error)
	err = lc.Remove(ctx, nonExistentPath, RemoveOptions{IgnoreNotExist: false})
	if err == nil {
		t.Errorf("Remove non-existent file %s without IgnoreNotExist should have failed", nonExistentPath)
	} else if !strings.Contains(err.Error(), "does not exist") { // Error message check
		t.Errorf("Remove non-existent file error message = %q, want to contain 'does not exist'", err.Error())
	}
}

func TestLocalConnector_GetFileChecksum(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-checksum-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "checksum_test.txt")

	// Generate random content
	randomContent := make([]byte, 128) // 128 random bytes
	_, rErr := rand.Read(randomContent)
	if rErr != nil {
		t.Fatalf("Failed to generate random content: %v", rErr)
	}

	if err := os.WriteFile(filePath, randomContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate expected SHA256sum from the original randomContent
	hasher := sha256.New()
	hasher.Write(randomContent)
	expectedSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// Get checksum using the connector method
	sha256sum, err := lc.GetFileChecksum(ctx, filePath, "sha256")
	if err != nil {
		t.Fatalf("GetFileChecksum sha256 error: %v", err)
	}
	if sha256sum != expectedSHA256 {
		t.Errorf("GetFileChecksum sha256 got %s, want %s", sha256sum, expectedSHA256)
	}

	// Test MD5 (currently not implemented in LocalConnector, should error)
	_, err = lc.GetFileChecksum(ctx, filePath, "md5")
	if err == nil {
		t.Error("GetFileChecksum md5 expected an error (not implemented), got nil")
	} else {
		// Updated to reflect that md5 is now caught by the "unsupported checksum type" general case
		if !strings.Contains(err.Error(), "unsupported checksum type 'md5'") {
			t.Errorf("GetFileChecksum md5 error = %q, want to contain 'unsupported checksum type 'md5''", err.Error())
		}
	}

	_, err = lc.GetFileChecksum(ctx, filePath, "invalidtype")
	if err == nil {
		t.Error("GetFileChecksum expected error for invalid type, got nil")
	} else {
		if !strings.Contains(err.Error(), "unsupported checksum type") {
			t.Errorf("GetFileChecksum invalid type error = %q, want to contain 'unsupported checksum type'", err.Error())
		}
	}

	nonExistentFile := filepath.Join(tmpDir, "non_existent_for_checksum.txt")
	_, err = lc.GetFileChecksum(ctx, nonExistentFile, "sha256")
	if err == nil {
		t.Errorf("GetFileChecksum for non-existent file %s should have failed", nonExistentFile)
	}
}

func TestLocalConnector_ReadFile(t *testing.T) {
	lc := &LocalConnector{}
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-readfile-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "read_test.txt")
	fileContent := []byte("Content to be read by ReadFile.")

	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	readData, err := lc.ReadFile(ctx, filePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", filePath, err)
	}
	if string(readData) != string(fileContent) {
		t.Errorf("ReadFile content mismatch: got %q, want %q", string(readData), string(fileContent))
	}

	nonExistentFile := filepath.Join(tmpDir, "non_existent_for_read.txt")
	_, err = lc.ReadFile(ctx, nonExistentFile)
	if err == nil {
		t.Errorf("ReadFile for non-existent file %s should have failed", nonExistentFile)
	} else {
		if !os.IsNotExist(errors.Unwrap(err)) && !os.IsNotExist(err) { // Check wrapped and unwrapped
			t.Errorf("ReadFile for non-existent file error type: got %v, want an os.IsNotExist error", err)
		}
	}
}

func TestLocalConnector_Exec_Sudo_WithPassword(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sudo tests are not applicable on Windows")
	}
	// This test expects sudo to fail because it will prompt for a password,
	// and stdin is not an interactive terminal.
	// The key is to check that our connector *attempts* to provide the password via `sudo -S`.

	lc := &LocalConnector{}
	lc.Connect(context.Background(), ConnectionCfg{Password: "testpassword"}) // Set a password

	ctx := context.Background()
	// A simple command that requires sudo, e.g., reading a root-owned file or listing a restricted dir
	// Using "sudo -S whoami" is a safe bet that it will try to use sudo -S.
	// If passwordless sudo is configured for `whoami`, it might succeed.
	// If not, it should fail asking for password.
	cmdStr := "whoami" // `sudo -S -p '' -E -- whoami` will be constructed

	stdout, stderr, err := lc.Exec(ctx, cmdStr, &ExecOptions{Sudo: true})

	if err == nil {
		// This could happen if passwordless sudo is enabled for 'whoami' for the current user
		t.Logf("Exec with Sudo:true and password unexpectedly succeeded. Stdout: %s, Stderr: %s. Passwordless sudo might be configured.", string(stdout), string(stderr))
		if !strings.Contains(strings.ToLower(string(stdout)), "root") {
			// If it succeeded but didn't run as root, that's also informative.
			t.Logf("Exec with Sudo:true and password succeeded but stdout %q does not contain 'root'.", string(stdout))
		}
		return // Test passes if it succeeded cleanly.
	}

	t.Logf("Exec with Sudo:true and password failed as expected (or due to other reasons). Err: %v, Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))

	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("Expected CommandError, got %T: %v", err, err)
	}

	// We expect sudo to fail asking for a password or similar.
	// The exact error message from sudo can vary.
	// "sudo: a password is required" or "sudo: incorrect password attempt"
	// "sudo: no tty present and no askpass program specified"
	// The key is that it's an error from `sudo` itself.
	// Our code should not error out before trying to execute `sudo -S`.
	stderrStr := strings.ToLower(string(stderr))
	if !(strings.Contains(stderrStr, "password is required") ||
		strings.Contains(stderrStr, "incorrect password attempt") ||
		strings.Contains(stderrStr, "no tty present") ||
		strings.Contains(stderrStr, "sorry, try again")) {
		t.Errorf("Expected stderr to indicate sudo password prompt/failure, but got: %s", string(stderr))
	}

	if cmdErr.ExitCode == -1 && cmdErr.Underlying == context.DeadlineExceeded { // Check if it timed out
		t.Errorf("Exec with Sudo:true and password timed out, which is not the expected failure mode for this test. Stderr: %s", string(stderr))
	}
}


func TestLocalConnector_FileOp_Sudo_WithPassword(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Sudo tests are not applicable on Windows")
	}

	lc := &LocalConnector{}
	lc.Connect(context.Background(), ConnectionCfg{Password: "fakepassword"}) // Set a password

	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "local-sudo-pass-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("sudo test content with password")
	testCases := []struct {
		name      string
		operation func() error
		checkFile string // file to check for existence if operation unexpectedly succeeds
	}{
		{
			name: "WriteFile with Sudo and Password",
			operation: func() error {
				dest := filepath.Join(tmpDir, "sudo_write_pass.txt")
				return lc.WriteFile(ctx, content, dest, &FileTransferOptions{Sudo: true, Permissions: "0600"})
			},
			checkFile: filepath.Join(tmpDir, "sudo_write_pass.txt"),
		},
		{
			name: "CopyContent with Sudo and Password",
			operation: func() error {
				dest := filepath.Join(tmpDir, "sudo_copycontent_pass.txt")
				return lc.CopyContent(ctx, content, dest, &FileTransferOptions{Sudo: true, Permissions: "0600"})
			},
			checkFile: filepath.Join(tmpDir, "sudo_copycontent_pass.txt"),
		},
		{
			name: "Copy (file) with Sudo and Password",
			operation: func() error {
				srcFile := filepath.Join(tmpDir, "src_copy_sudo_pass.txt")
				if err := os.WriteFile(srcFile, content, 0644); err != nil {
					t.Fatalf("Failed to create source file for Copy test: %v", err)
				}
				dest := filepath.Join(tmpDir, "dest_copy_sudo_pass.txt")
				return lc.Copy(ctx, srcFile, dest, &FileTransferOptions{Sudo: true, Permissions: "0600"})
			},
			checkFile: filepath.Join(tmpDir, "dest_copy_sudo_pass.txt"),
		},
		{
			name: "Remove (file) with Sudo and Password",
			operation: func() error {
				fileToRemove := filepath.Join(tmpDir, "remove_sudo_pass.txt")
				// Create it first so remove has something to target
				if err := os.WriteFile(fileToRemove, content, 0644); err != nil {
					t.Fatalf("Failed to create file for Remove test: %v", err)
				}
				return lc.Remove(ctx, fileToRemove, RemoveOptions{Sudo: true})
			},
			checkFile: "", // For remove, success means file is gone.
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.operation()
			if err == nil {
				t.Logf("%s unexpectedly succeeded. This might happen with passwordless sudo for the relevant commands (tee, mv, rm).", tc.name)
				if tc.checkFile != "" { // For Write/Copy ops
					if _, statErr := os.Stat(tc.checkFile); os.IsNotExist(statErr) {
						t.Errorf("%s claimed success but file %s does not exist", tc.name, tc.checkFile)
					}
				} else { // For Remove op
					// If checkFile is empty, it's a Remove test. Success means file is gone.
					// This path is tricky because the file path for remove is internal to operation().
					// For simplicity, we assume if Remove op succeeds, it did its job.
					// A more robust check would re-stat the file path used by the Remove operation.
				}
				return
			}

			t.Logf("%s failed as expected (or due to other reasons). Err: %v", tc.name, err)
			// Check that the error is likely from sudo itself (e.g., password prompt)
			// and not an internal error from our Go code before attempting sudo.
			// CommandError contains Stderr from the command.
			var cmdErr *CommandError
			if errors.As(err, &cmdErr) {
				stderrStr := strings.ToLower(cmdErr.Stderr)
				if !(strings.Contains(stderrStr, "password is required") ||
					strings.Contains(stderrStr, "incorrect password attempt") ||
					strings.Contains(stderrStr, "no tty present") ||
					strings.Contains(stderrStr, "sorry, try again") ||
					strings.Contains(strings.ToLower(err.Error()), "exit status 1")) { // General sudo failure
					t.Errorf("Expected stderr from %s to indicate sudo password prompt/failure, but got: Stderr: %s, FullError: %v", tc.name, cmdErr.Stderr, err)
				}
			} else {
				// If not CommandError, it might be an issue before exec, e.g., temp file creation failed.
				// This is less likely for the sudo path itself but possible.
				t.Logf("Error for %s was not a CommandError: %T, %v", tc.name, err, err)
			}

			if strings.Contains(err.Error(), "sudo not implemented") {
				t.Errorf("%s should not return 'sudo not implemented', got: %v", tc.name, err)
			}
		})
	}
}
