package connector

import (
	"context"
	"crypto/rand" // For random content in checksum test
	"crypto/sha256"
	"encoding/hex"
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
	err = lc.WriteFile(ctx, content, writeFileDest, "0644", true)
	if err == nil {
		// This might pass if passwordless sudo is configured for `tee` or `mkdir` and `chmod`.
		// In a typical CI, it should fail. We check the file content if it passes.
		t.Logf("WriteFile with Sudo unexpectedly succeeded. This might happen with passwordless sudo.")
		// Attempt to read the file to verify content if it passed. This read is non-sudo.
		// If it succeeded, the file should exist.
		if _, statErr := os.Stat(writeFileDest); os.IsNotExist(statErr) {
			 t.Errorf("WriteFile with Sudo claimed success but file %s does not exist", writeFileDest)
		}
		// Clean up if it succeeded to avoid interfering with other tests or permissions issues.
		os.Remove(writeFileDest)
	} else {
		t.Logf("WriteFile with Sudo expectedly failed (likely no passwordless sudo): %v", err)
		if strings.Contains(err.Error(), "sudo not implemented") {
			t.Errorf("WriteFile with Sudo should not return 'sudo not implemented', got: %v", err)
		}
		// Example error on Linux if sudo requires password: "sudo: a terminal is required to read the password"
		// Or "failed to write to ... with sudo tee: sudo: a password is required"
		if !strings.Contains(err.Error(), "sudo") && !strings.Contains(err.Error(), "Sudo") {
			t.Errorf("WriteFile with Sudo error message %q did not contain 'sudo' or 'Sudo'", err.Error())
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
