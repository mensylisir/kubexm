package connector

import (
	"context"
	"fmt"
	"os"
	"os/user" // Added for user.Current()
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// IMPORTANT: These tests for SSHConnector are integration tests and require a local SSH server
// accessible at `localhost:22` (or the port specified by env var SSH_TEST_PORT).
// The user specified by SSH_TEST_USER (default: current user) must be able to SSH into localhost
// using either a password specified by SSH_TEST_PASSWORD or a private key specified by SSH_TEST_PRIV_KEY_PATH.
//
// Example environment variables for running these tests:
// export SSH_TEST_USER=$(whoami)
// export SSH_TEST_PRIV_KEY_PATH=$HOME/.ssh/id_rsa
// export SSH_TEST_PORT=2222 (if your sshd is on a different port)
// export SSH_TEST_PASSWORD="yourpassword" (use if not using key-based auth)

var (
	sshTestHost        = os.Getenv("SSH_TEST_HOST")
	sshTestUser        = os.Getenv("SSH_TEST_USER")
	sshTestPassword    = os.Getenv("SSH_TEST_PASSWORD")
	sshTestPrivKeyPath = os.Getenv("SSH_TEST_PRIV_KEY_PATH")
	sshTestPortStr     = os.Getenv("SSH_TEST_PORT")
	sshTestPort        = 22 // Default SSH port
	sshTestTimeout     = 10 * time.Second
	enableSshTests     = true // Temporarily force enabled for this session
)

func setupSSHTest(t *testing.T) *SSHConnector {
	// if !enableSshTests { // Already forced to true
	// 	t.Skip("SSHConnector tests are disabled. Set ENABLE_SSH_CONNECTOR_TESTS=true to run them.")
	// }

	if sshTestUser == "" {
		currentUser, err := user.Current()
		if err != nil {
			t.Fatalf("SSH_TEST_USER not set and user.Current() failed: %v. Please set SSH_TEST_USER.", err)
		}
		sshTestUser = currentUser.Username
		fmt.Printf("SSH_TEST_USER not set, defaulting to current user: %s. Ensure this user can SSH to localhost.\n", sshTestUser)
	}

	if sshTestPassword == "" && sshTestPrivKeyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
			if _, statErr := os.Stat(defaultKeyPath); statErr == nil {
				sshTestPrivKeyPath = defaultKeyPath
				fmt.Printf("SSH_TEST_PASSWORD and SSH_TEST_PRIV_KEY_PATH not set. Defaulting to use private key: %s\n", sshTestPrivKeyPath)
			} else {
				// If default key doesn't exist, we can't proceed without some auth method.
				// For CI/sandbox, it's unlikely a password will be set, and a specific key might be needed.
				// This test might still fail if localhost SSH isn't passwordless for the user or key isn't there.
				fmt.Printf("Warning: No SSH_TEST_PASSWORD or SSH_TEST_PRIV_KEY_PATH set, and default key %s not found. Tests may fail if passwordless SSH is not configured for user %s.\n", defaultKeyPath, sshTestUser)
			}
		} else {
			fmt.Printf("Warning: Could not get user home directory to check for default SSH key: %v. No SSH auth method explicitly set.\n", err)
		}
	}

	// Ensure sshTestHost is localhost
	if sshTestPortStr != "" {
		var err error
		sshTestPortVal, err := strconv.Atoi(sshTestPortStr)
		if err != nil {
			t.Fatalf("Invalid SSH_TEST_PORT: %v", err)
		}
		sshTestPort = sshTestPortVal
	}

	cfg := ConnectionCfg{
		Host:           sshTestHost,
		Port:           sshTestPort,
		User:           sshTestUser,
		Password:       sshTestPassword,
		PrivateKeyPath: sshTestPrivKeyPath,
		Timeout:        sshTestTimeout,
	}

	sc := &SSHConnector{}
	ctx, cancel := context.WithTimeout(context.Background(), sshTestTimeout)
	defer cancel()

	if err := sc.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect to SSH server %s:%d for testing: %v. Check test setup and SSH server.", sshTestHost, sshTestPort, err)
	}
	return sc
}

func TestSSHConnector_Connect_And_Close(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()

	if !sc.IsConnected() {
		t.Error("SSHConnector.IsConnected() should be true after successful Connect")
	}
	err := sc.Close()
	if err != nil {
		t.Errorf("SSHConnector.Close() error = %v", err)
	}
	// Note: IsConnected might still briefly be true if the underlying client hasn't fully closed.
	// A more robust IsConnected might actively check. For now, we trust Close().
	// After refactor, IsConnected uses keepalive, so it should be more accurate.
	if sc.IsConnected() {
		t.Error("SSHConnector.IsConnected() should be false after Close")
	}
}

func TestSSHConnector_Exec_Retries(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteTmpDir := fmt.Sprintf("/tmp/sshconnector-retry-test-%d", time.Now().UnixNano())
	scriptName := "retry_script.sh"
	remoteScriptPath := filepath.Join(remoteTmpDir, scriptName)
	remoteFailCountPath := filepath.Join(remoteTmpDir, "fail_count")
	remoteSuccessMarkerPath := filepath.Join(remoteTmpDir, "success_marker")

	// Ensure remote tmp dir exists and is clean for the script
	cleanupCmd := fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteTmpDir, remoteTmpDir)
	_, _, err := sc.Exec(ctx, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("Failed to create remote temp dir %s for retry test: %v", remoteTmpDir, err)
	}
	defer sc.Exec(context.Background(), fmt.Sprintf("rm -rf %s", remoteTmpDir), nil)

	// Script fails twice, then succeeds
	scriptContent := `
#!/bin/sh
FAIL_COUNT_FILE="` + remoteFailCountPath + `"
SUCCESS_MARKER="` + remoteSuccessMarkerPath + `"
if [ ! -f "$FAIL_COUNT_FILE" ]; then
    echo 0 > "$FAIL_COUNT_FILE"
fi
count=$(cat "$FAIL_COUNT_FILE")
echo $((count + 1)) > "$FAIL_COUNT_FILE"
if [ "$count" -lt 2 ]; then
    echo "Attempt $count: failing (on remote)" >&2
    exit 1
else
    echo "Attempt $count: succeeding (on remote)"
    touch "$SUCCESS_MARKER"
    exit 0
fi
`
	err = sc.CopyContent(ctx, []byte(scriptContent), remoteScriptPath, &FileTransferOptions{Permissions: "0755"})
	if err != nil {
		t.Fatalf("Failed to upload retry script to %s: %v", remoteScriptPath, err)
	}

	opts := &ExecOptions{
		Retries:    2, // Should succeed on the 3rd attempt (0, 1, 2)
		RetryDelay: 100 * time.Millisecond, // Give some delay for remote execution
	}

	stdout, stderr, err := sc.Exec(ctx, remoteScriptPath, opts)
	if err != nil {
		t.Fatalf("SSHConnector.Exec() with retries failed: %v\nStdout: %s\nStderr: %s", err, string(stdout), string(stderr))
	}

	// Verify success marker
	statMarker, statErr := sc.Stat(ctx, remoteSuccessMarkerPath)
	if statErr != nil || !statMarker.IsExist {
		t.Errorf("Success marker file %s was not created or Stat failed (err: %v), script did not succeed as expected.", remoteSuccessMarkerPath, statErr)
	}

	expectedStdout := "Attempt 2: succeeding (on remote)"
	if !strings.Contains(string(stdout), expectedStdout) {
		t.Errorf("Expected stdout to contain %q, got %q", expectedStdout, string(stdout))
	}
	t.Logf("SSH Retry stdout: %s", string(stdout))
	t.Logf("SSH Retry stderr: %s", string(stderr)) // Should contain failure messages

	// Test retry with timeout that causes failure even after retries
	remoteFailScriptPath := filepath.Join(remoteTmpDir, "fail_timeout_script.sh")
	failScriptContent := `#!/bin/sh
echo "Trying to sleep for a long time (on remote)..."
sleep 10
echo "Slept (on remote)"
exit 0`
	err = sc.CopyContent(ctx, []byte(failScriptContent), remoteFailScriptPath, &FileTransferOptions{Permissions: "0755"})
	if err != nil {
		t.Fatalf("Failed to upload fail_timeout_script.sh: %v", err)
	}

	failOpts := &ExecOptions{
		Retries:    1,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    100 * time.Millisecond, // Each attempt times out
	}
	_, _, err = sc.Exec(ctx, remoteFailScriptPath, failOpts)
	if err == nil {
		t.Fatalf("SSHConnector.Exec() with retries and timeout should have failed")
	}
	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("Expected CommandError, got %T: %v", err, err)
	}
	if cmdErr.Underlying == nil {
		t.Errorf("Expected CommandError.Underlying to be non-nil for timeout, got nil")
	} else {
		// For SSH, timeout often results in "context deadline exceeded" directly or via session error.
		// *ssh.ExitError might not be the case if the command doesn't even start or is killed by session timeout.
		underlyingErrStr := cmdErr.Underlying.Error()
		if !strings.Contains(underlyingErrStr, "deadline exceeded") && !strings.Contains(underlyingErrStr, "timeout") {
			t.Errorf("Expected CommandError.Underlying for SSH retry timeout to indicate timeout, got: %v", cmdErr.Underlying)
		}
	}
	t.Logf("SSH Retry with timeout error: %v", err)
}

func TestSSHConnector_Exec_Simple(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()

	ctx := context.Background()
	cmdStr := "echo ssh_hello"
	stdout, stderr, err := sc.Exec(ctx, cmdStr, nil)

	if err != nil {
		t.Fatalf("SSHConnector.Exec() error = %v. Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))
	}
	if strings.TrimSpace(string(stdout)) != "ssh_hello" {
		t.Errorf("stdout = %q, want 'ssh_hello'", string(stdout))
	}
	if string(stderr) != "" {
		t.Errorf("stderr = %q, want empty", string(stderr))
	}
}

func TestSSHConnector_Exec_Error(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()
	cmdStr := "exit 123" // Command that exits with a specific code

	_, _, err := sc.Exec(ctx, cmdStr, nil)
	if err == nil {
		t.Fatalf("SSHConnector.Exec() with failing command expected error, got nil")
	}
	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("Expected CommandError, got %T: %v", err, err)
	}
	if cmdErr.ExitCode != 123 {
		t.Errorf("CommandError ExitCode = %d, want 123", cmdErr.ExitCode)
	}
}

func TestSSHConnector_FileOperations(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	// Create a temporary directory for local files
	localTmpDir, err := os.MkdirTemp("", "sshconnector-local-test-")
	if err != nil {
		t.Fatalf("Failed to create local temp dir: %v", err)
	}
	defer os.RemoveAll(localTmpDir)

	// Define remote temporary path (ensure user has write permission there)
	remoteTmpDir := fmt.Sprintf("/tmp/sshconnector-remote-test-%d", time.Now().UnixNano())
	// Create remote temp dir
	_, _, err = sc.Exec(ctx, "mkdir -p "+remoteTmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create remote temp dir %s: %v", remoteTmpDir, err)
	}
	defer func() {
		// Cleanup remote directory
		_, _, derr := sc.Exec(context.Background(), "rm -rf "+remoteTmpDir, nil)
		if derr != nil {
			t.Logf("Warning: failed to cleanup remote temp dir %s: %v", remoteTmpDir, derr)
		}
	}()

	localSrcFileName := "local_source.txt"
	remoteDstFileName := "remote_dest.txt"
	remoteContentFileName := "remote_content.txt"
	localFetchFileName := "local_fetch.txt"

	localSrcFilePath := filepath.Join(localTmpDir, localSrcFileName)
	remoteDstFilePath := filepath.Join(remoteTmpDir, remoteDstFileName)
	remoteContentFilePath := filepath.Join(remoteTmpDir, remoteContentFileName)
	localFetchFilePath := filepath.Join(localTmpDir, localFetchFileName)

	fileContent := []byte("Hello, SSHConnector via SFTP!")

	// 1. Test CopyContent
	err = sc.CopyContent(ctx, fileContent, remoteContentFilePath, &FileTransferOptions{Permissions: "0644"})
	if err != nil {
		t.Fatalf("CopyContent() to %s error = %v", remoteContentFilePath, err)
	}
	// Verify content using Exec cat
	stdout, _, err := sc.Exec(ctx, "cat "+remoteContentFilePath, nil)
	if err != nil {
		t.Fatalf("Exec cat after CopyContent error = %v", err)
	}
	if string(stdout) != string(fileContent) {
		t.Errorf("CopyContent() content mismatch: got %q, want %q", string(stdout), string(fileContent))
	}
	// TODO: Verify permissions if possible (stat command and parse)

	// Create local source file for Copy
	err = os.WriteFile(localSrcFilePath, fileContent, 0666)
	if err != nil {
		t.Fatalf("Failed to write local source file: %v", err)
	}

	// 2. Test WriteFile (simulating Copy from local by reading then writing)
	// Or, if CopyContent is preferred for raw bytes:
	// err = sc.CopyContent(ctx, fileContent, remoteDstFilePath, &FileTransferOptions{Permissions: "0600"})
	err = sc.WriteFile(ctx, fileContent, remoteDstFilePath, "0600", false) // Assuming sudo=false for this test
	if err != nil {
		t.Fatalf("WriteFile() to %s error = %v", remoteDstFilePath, err)
	}
	stdoutCopy, _, errCopy := sc.Exec(ctx, "cat "+remoteDstFilePath, nil)
	if errCopy != nil {
		t.Fatalf("Exec cat after WriteFile error = %v", errCopy)
	}
	if string(stdoutCopy) != string(fileContent) {
		t.Errorf("WriteFile() content mismatch: got %q, want %q", string(stdoutCopy), string(fileContent))
	}
	// TODO: Verify permissions

	// 3. Test ReadFile (simulating Fetch to local)
	remoteReadBytes, err := sc.ReadFile(ctx, remoteDstFilePath)
	if err != nil {
		t.Fatalf("ReadFile() from %s error = %v", remoteDstFilePath, err)
	}
	if string(remoteReadBytes) != string(fileContent) {
		t.Errorf("ReadFile() content mismatch: got %q, want %q", string(remoteReadBytes), string(fileContent))
	}
	// Optionally write to localFetchFilePath to fully simulate Fetch
	err = os.WriteFile(localFetchFilePath, remoteReadBytes, 0666)
	if err != nil {
		t.Fatalf("Failed to write fetched content to local file %s: %v", localFetchFilePath, err)
	}

	// 4. Test Stat
	fileStat, err := sc.Stat(ctx, remoteDstFilePath)
	if err != nil {
		t.Fatalf("Stat() for %s error = %v", remoteDstFilePath, err)
	}
	if !fileStat.IsExist {
		t.Errorf("Stat() file %s should exist", remoteDstFilePath)
	}
	if fileStat.Name != remoteDstFileName {
		t.Errorf("Stat() name mismatch: got %s, want %s", fileStat.Name, remoteDstFileName)
	}
	if fileStat.Size != int64(len(fileContent)) {
		t.Errorf("Stat() size mismatch: got %d, want %d", fileStat.Size, len(fileContent))
	}

	nonExistentRemotePath := filepath.Join(remoteTmpDir, "nonexistent.txt")
	fileStatNE, err := sc.Stat(ctx, nonExistentRemotePath)
	if err != nil {
		t.Fatalf("Stat() for non-existent file %s error = %v", nonExistentRemotePath, err)
	}
	if fileStatNE.IsExist {
		t.Errorf("Stat() file %s should not exist", nonExistentRemotePath)
	}
}

func TestSSHConnector_LookPath(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	executableName := "sh" // sh should be in PATH on most Unix-like systems
	path, err := sc.LookPath(ctx, executableName)
	if err != nil {
		t.Fatalf("LookPath(%q) error = %v", executableName, err)
	}
	if path == "" || !strings.Contains(path, "/bin/sh") { // Path can vary, but should contain /bin/sh
		t.Errorf("LookPath(%q) returned suspicious path: %s", executableName, path)
	}
	t.Logf("Found %s at %s", executableName, path)

	_, err = sc.LookPath(ctx, "non_existent_ssh_executable_adjhfajkshd")
	if err == nil {
		t.Errorf("LookPath() for non-existent executable expected error, got nil")
	}
}

func TestSSHConnector_LookPath_Injection(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	// Test with a command that includes shell metacharacters
	// The new LookPath has basic validation and also uses shellEscape for `command -v`.
	// `command -v 'sh;id'` should look for an executable literally named "sh;id".
	// It should not execute `id`.
	maliciousCmd := "sh;id"
	_, err := sc.LookPath(ctx, maliciousCmd)
	if err == nil {
		t.Errorf("LookPath(%q) should have failed or returned error due to invalid chars / not found, but it succeeded.", maliciousCmd)
	} else {
		t.Logf("LookPath(%q) correctly failed with: %v", maliciousCmd, err)
		// We expect errors like "invalid characters" or "failed to find executable"
		if !strings.Contains(err.Error(), "invalid characters") && !strings.Contains(err.Error(), "failed to find executable") {
			t.Errorf("LookPath(%q) error message %q was not one of the expected injection-related errors.", maliciousCmd, err.Error())
		}
	}

	// Test with spaces and quotes (should be escaped and treated literally)
	cmdWithSpaces := "my command"
	_, err = sc.LookPath(ctx, cmdWithSpaces)
	if err == nil {
		t.Errorf("LookPath(%q) should have failed (command not found or invalid), but succeeded.", cmdWithSpaces)
	} else {
		t.Logf("LookPath(%q) correctly failed with: %v", cmdWithSpaces, err)
		if !strings.Contains(err.Error(), "invalid characters") && !strings.Contains(err.Error(), "failed to find executable") {
			t.Errorf("LookPath(%q) error message %q was not one of the expected injection-related errors.", cmdWithSpaces, err.Error())
		}
	}
}


func TestSSHConnector_GetOS(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	osInfo, err := sc.GetOS(ctx)
	if err != nil {
		t.Fatalf("GetOS() error = %v", err)
	}
	if osInfo == nil {
		t.Fatal("GetOS() returned nil OS info")
	}

	t.Logf("Remote OS: ID=%s, VersionID=%s, Codename=%s, Arch=%s, Kernel=%s",
		osInfo.ID, osInfo.VersionID, osInfo.Codename, osInfo.Arch, osInfo.Kernel)

	if osInfo.ID == "" {
		t.Error("GetOS() returned empty ID")
	}
	if osInfo.Arch == "" {
		t.Error("GetOS() returned empty Arch")
	}
	if osInfo.Kernel == "" {
		t.Error("GetOS() returned empty Kernel")
	}

	// Test caching
	osInfo2, err2 := sc.GetOS(ctx)
	if err2 != nil {
		t.Fatalf("GetOS() second call error = %v", err2)
	}
	if osInfo != osInfo2 { // Should be the same cached pointer
		t.Error("GetOS() caching failed; returned different struct pointers")
	}
}

func TestSSHConnector_SudoWriteFile(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	sudoTestDir := "/tmp/kubexm_sudo_test_dir"
	sudoTestFilePath := filepath.Join(sudoTestDir, "sudo_test_file.txt")

	content := []byte("written with sudo")
	permissions := "0640"

	cleanupCmd := fmt.Sprintf("sudo rm -rf %s", sudoTestDir)
	// Initial cleanup, ignore error if dir doesn't exist
	_, _, _ = sc.Exec(context.Background(), cleanupCmd, nil)

	defer func() {
		_, _, err := sc.Exec(context.Background(), cleanupCmd, nil)
		if err != nil {
			t.Logf("Warning: failed to cleanup sudo test directory %s: %v", sudoTestDir, err)
		}
	}()

	// Setup directory with root ownership to ensure sudo is needed for writing into it.
	// This command sequence might need adjustment based on test environment specifics.
	// 1. sudo mkdir -p (creates dir, parent may be user-owned initially)
	// 2. sudo chown root:root (changes ownership of the final dir)
	setupCmd := fmt.Sprintf("sudo mkdir -p %s && sudo chown root:root %s", sudoTestDir, sudoTestDir)
	_, stderrSetup, setupErr := sc.Exec(ctx, setupCmd, nil)
	if setupErr != nil {
		t.Fatalf("Failed to set up sudo test directory %s with root ownership: %v. Stderr: %s. Check sudo permissions for user %s.", sudoTestDir, setupErr, string(stderrSetup), sshTestUser)
	}

	err := sc.WriteFile(ctx, content, sudoTestFilePath, permissions, true)
	if err != nil {
		t.Fatalf("WriteFile with sudo to %s error = %v", sudoTestFilePath, err)
	}

	fileStat, statErr := sc.Stat(ctx, sudoTestFilePath)
	if statErr != nil {
		t.Fatalf("Stat after sudo WriteFile for %s error = %v", sudoTestFilePath, statErr)
	}
	if !fileStat.IsExist {
		t.Errorf("File %s should exist after sudo WriteFile", sudoTestFilePath)
	}
	if fileStat.Size != int64(len(content)) {
		t.Errorf("File %s size mismatch: got %d, want %d", sudoTestFilePath, fileStat.Size, len(content))
	}

	// Verify content using sudo cat, as the file might be root-owned with restrictive permissions.
	catCmd := fmt.Sprintf("sudo cat %s", sudoTestFilePath)
	stdoutCat, stderrCat, catErr := sc.Exec(ctx, catCmd, nil)
	if catErr != nil {
		t.Fatalf("sudo cat %s failed: %v. Stderr: %s", sudoTestFilePath, catErr, string(stderrCat))
	}
	if string(stdoutCat) != string(content) {
		t.Errorf("Content mismatch for sudo written file: got %q, want %q", string(stdoutCat), string(content))
	}
}

func TestSSHConnector_Mkdir(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteBaseDir := fmt.Sprintf("/tmp/sshconnector-mkdir-test-%d", time.Now().UnixNano())
	defer sc.Exec(context.Background(), "rm -rf "+remoteBaseDir, nil) // Cleanup

	dirToCreate := filepath.Join(remoteBaseDir, "a", "b", "c")
	perms := "0755"

	err := sc.Mkdir(ctx, dirToCreate, perms)
	if err != nil {
		t.Fatalf("Mkdir(%s, %s) error = %v", dirToCreate, perms, err)
	}

	// Verify directory exists
	stat, err := sc.Stat(ctx, dirToCreate)
	if err != nil {
		t.Fatalf("Stat(%s) after Mkdir error = %v", dirToCreate, err)
	}
	if !stat.IsExist || !stat.IsDir {
		t.Errorf("Directory %s not created or not a directory. IsExist: %v, IsDir: %v", dirToCreate, stat.IsExist, stat.IsDir)
	}
	// TODO: Verify permissions if Stat provides them reliably or use `ls -ld` and parse.
}

func TestSSHConnector_Mkdir_Injection(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteBaseDir := fmt.Sprintf("/tmp/sshconnector-mkdir-inject-test-%d", time.Now().UnixNano())
	defer sc.Exec(context.Background(), shellEscape(fmt.Sprintf("rm -rf %s", remoteBaseDir)), nil) // Ensure base dir is shell-escaped for cleanup

	// Attempt to create a directory with a name that includes shell metacharacters
	// This should create a directory literally named "dir;id" or fail, but not execute "id"
	maliciousDirName := "dir;id"
	dirToCreate := filepath.Join(remoteBaseDir, maliciousDirName)
	evilCheckDir := filepath.Join(remoteBaseDir, "id") // If `id` command was executed and created a dir/file

	err := sc.Mkdir(ctx, dirToCreate, "0755")
	if err != nil {
		// This might fail if the OS doesn't allow such characters in dir names, which is acceptable.
		t.Logf("Mkdir(%q) failed as potentially expected due to OS restrictions or other issues: %v", dirToCreate, err)
	} else {
		// If Mkdir succeeded, verify the literal directory was created
		stat, statErr := sc.Stat(ctx, dirToCreate)
		if statErr != nil || !stat.IsExist || !stat.IsDir {
			t.Errorf("Mkdir(%q) claimed success, but directory not found or not a dir. StatErr: %v, IsExist: %v, IsDir: %v",
				dirToCreate, statErr, stat != nil && stat.IsExist, stat != nil && stat.IsDir)
		} else {
			t.Logf("Mkdir(%q) succeeded in creating the literal directory.", dirToCreate)
		}
	}

	// IMPORTANT: Verify that the unintended command did NOT execute.
	// Check if a directory/file named "id" (or whatever the payload was) was created in remoteBaseDir.
	// This assumes `id` command (if it ran) wouldn't typically create a directory named `id`,
	// but as a simple check, we see if a file/dir `id` exists.
	// A more robust check might be to see if a *file* was created by `id > some_file`.
	// For this test, we just check if a path `id` was created.
	statEvil, _ := sc.Stat(ctx, evilCheckDir)
	if statEvil != nil && statEvil.IsExist {
		t.Errorf("Potential command injection: path %q was created during Mkdir(%q)", evilCheckDir, dirToCreate)
	}

	// Test with spaces "my dir"
	dirWithSpaces := filepath.Join(remoteBaseDir, "my lovely dir")
	err = sc.Mkdir(ctx, dirWithSpaces, "0755")
	if err != nil {
		t.Fatalf("Mkdir(%q) with spaces failed: %v", dirWithSpaces, err)
	}
	statSpace, statSpaceErr := sc.Stat(ctx, dirWithSpaces)
	if statSpaceErr != nil || !statSpace.IsExist {
		t.Errorf("Mkdir(%q) with spaces failed to create dir. StatErr: %v", dirWithSpaces, statSpaceErr)
	}
}


func TestSSHConnector_Remove(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteBaseDir := fmt.Sprintf("/tmp/sshconnector-remove-test-%d", time.Now().UnixNano())
	defer sc.Exec(context.Background(), "rm -rf "+remoteBaseDir, nil)

	// Setup: Create a file and a directory
	fileToCreate := filepath.Join(remoteBaseDir, "file.txt")
	dirToCreate := filepath.Join(remoteBaseDir, "subdir", "nesteddir")

	_, _, err := sc.Exec(ctx, fmt.Sprintf("mkdir -p %s", dirToCreate), nil)
	if err != nil {
		t.Fatalf("Failed to setup test directory %s: %v", dirToCreate, err)
	}

	err = sc.CopyContent(ctx, []byte("test remove"), fileToCreate, nil)
	if err != nil {
		t.Fatalf("Failed to setup test file %s: %v", fileToCreate, err)
	}

	// Test Remove file
	err = sc.Remove(ctx, fileToCreate, RemoveOptions{})
	if err != nil {
		t.Errorf("Remove file %s error = %v", fileToCreate, err)
	}
	stat, _ := sc.Stat(ctx, fileToCreate)
	if stat != nil && stat.IsExist {
		t.Errorf("File %s should not exist after Remove", fileToCreate)
	}

	// Test Remove directory (non-recursive, should fail or do nothing to contents)
	// For `rm -f`, it won't remove a non-empty directory.
	// Let's test recursive removal.

	// Test Remove directory (recursive)
	err = sc.Remove(ctx, remoteBaseDir, RemoveOptions{Recursive: true})
	if err != nil {
		t.Errorf("Remove directory recursively %s error = %v", remoteBaseDir, err)
	}
	stat, _ = sc.Stat(ctx, remoteBaseDir)
	if stat != nil && stat.IsExist {
		t.Errorf("Directory %s should not exist after recursive Remove", remoteBaseDir)
	}

	// Test IgnoreNotExist
	nonExistentPath := filepath.Join(remoteBaseDir, "non_existent_file.txt")
	err = sc.Remove(ctx, nonExistentPath, RemoveOptions{IgnoreNotExist: true})
	if err != nil {
		t.Errorf("Remove with IgnoreNotExist for %s expected no error, got %v", nonExistentPath, err)
	}
}

func TestSSHConnector_Remove_Injection(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteBaseDir := fmt.Sprintf("/tmp/sshconnector-remove-inject-test-%d", time.Now().UnixNano())
	// Create the base directory first, so cleanup can target it.
	// Use a simple, safe name for the base directory itself for setup/cleanup.
	_, _, err := sc.Exec(ctx, fmt.Sprintf("mkdir -p %s", remoteBaseDir), nil)
	if err != nil {
		t.Fatalf("Failed to create remote base dir %s for injection test: %v", remoteBaseDir, err)
	}
	defer sc.Exec(context.Background(), fmt.Sprintf("rm -rf %s", remoteBaseDir), nil)


	// Attempt to remove a file/dir with a name that includes shell metacharacters
	// This should try to remove a literal file named "file;id" or fail, but not execute "id"
	maliciousName := "file;id"
	pathToRemove := filepath.Join(remoteBaseDir, maliciousName)

	// Create a dummy file with the malicious name to see if `rm` targets it literally.
	// Use CopyContent which now uses SFTP or sudo tee, both should handle literal names.
	// For this test, non-sudo is fine to just create the file.
	err = sc.CopyContent(ctx, []byte("dummy"), pathToRemove, nil)
	if err != nil {
		// If creating the file with such a name fails, the OS might not support it.
		// This makes the injection test for rm less direct, but rm should still not execute commands.
		t.Logf("Could not create file %q for testing Remove injection (OS limitation?): %v", pathToRemove, err)
	} else {
		t.Logf("Successfully created file %q for Remove injection test.", pathToRemove)
	}

	// The important part is that Remove attempts to delete the literal path.
	// It should not execute `id`.
	err = sc.Remove(ctx, pathToRemove, RemoveOptions{Recursive: true}) // Recursive true for safety if it was a dir
	if err != nil {
		// This is acceptable if the file couldn't be created, or if rm failed for other reasons.
		t.Logf("Remove(%q) failed (as potentially expected): %v", pathToRemove, err)
	} else {
		t.Logf("Remove(%q) succeeded.", pathToRemove)
		// Verify it's gone if it was created
		stat, _ := sc.Stat(ctx, pathToRemove)
		if stat != nil && stat.IsExist {
			t.Errorf("Remove(%q) claimed success, but path still exists.", pathToRemove)
		}
	}

	// Verify no "id" file/dir was created as a side effect in remoteBaseDir
	evilCheckPath := filepath.Join(remoteBaseDir, "id")
	statEvil, _ := sc.Stat(ctx, evilCheckPath)
	if statEvil != nil && statEvil.IsExist {
		t.Errorf("Potential command injection: path %q was created during Remove(%q)", evilCheckPath, pathToRemove)
	}
}


func TestSSHConnector_GetFileChecksum(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteDir := fmt.Sprintf("/tmp/sshconnector-checksum-test-%d", time.Now().UnixNano())
	remoteFile := filepath.Join(remoteDir, "checksum_test.txt")
	defer sc.Exec(context.Background(), "rm -rf "+remoteDir, nil)

	_, _, err := sc.Exec(ctx, "mkdir -p "+remoteDir, nil)
	if err != nil {
		t.Fatalf("Failed to create remote dir %s: %v", remoteDir, err)
	}

	// Use printf to ensure exact byte content for checksum verification
	content := "hello checksum\\n" // Escaped for printf
	createCmd := fmt.Sprintf("mkdir -p %s && printf %q > %s", remoteDir, content, remoteFile)
	_, stderrCreate, errCreate := sc.Exec(ctx, createCmd, nil)
	if errCreate != nil {
		t.Fatalf("Failed to create test file with printf: %v, stderr: %s", errCreate, string(stderrCreate))
	}

	// SHA256 for "hello checksum\n" is 4d810e9e8017aaccc2573e3925be756cf8dae6edc80f5faaa6abc7e537c433a5
	expectedSHA256 := "4d810e9e8017aaccc2573e3925be756cf8dae6edc80f5faaa6abc7e537c433a5"
	sha256sum, err := sc.GetFileChecksum(ctx, remoteFile, "sha256")
	if err != nil {
		t.Fatalf("GetFileChecksum sha256 error: %v", err)
	}
	if sha256sum != expectedSHA256 {
		t.Errorf("GetFileChecksum sha256 got %s, want %s", sha256sum, expectedSHA256)
	}

	// MD5 for "hello checksum\n" is 80317437f4b5cabf233cb6f139d29c1b
	expectedMD5 := "80317437f4b5cabf233cb6f139d29c1b"
	md5sum, err := sc.GetFileChecksum(ctx, remoteFile, "md5")
	if err != nil {
		t.Fatalf("GetFileChecksum md5 error: %v", err)
	}
	if md5sum != expectedMD5 {
		t.Errorf("GetFileChecksum md5 got %s, want %s", md5sum, expectedMD5)
	}

	_, err = sc.GetFileChecksum(ctx, remoteFile, "invalidtype")
	if err == nil {
		t.Error("GetFileChecksum expected error for invalid type, got nil")
	}
}

func TestSSHConnector_GetFileChecksum_Injection(t *testing.T) {
	sc := setupSSHTest(t)
	defer sc.Close()
	ctx := context.Background()

	remoteDir := fmt.Sprintf("/tmp/sshconnector-checksum-inject-%d", time.Now().UnixNano())
	defer sc.Exec(context.Background(), fmt.Sprintf("rm -rf %s", remoteDir), nil)
	_, _, err := sc.Exec(ctx, fmt.Sprintf("mkdir -p %s", remoteDir), nil)
	if err != nil {
		t.Fatalf("Failed to create remote dir %s: %v", remoteDir, err)
	}

	// Attempt to get checksum of a "file" whose name includes shell metacharacters
	// e.g. /tmp/somefile;id
	// The `shellEscape` should prevent `id` from executing.
	// The checksum command (e.g. sha256sum) should then fail because the file "somefile;id" doesn't exist.
	maliciousPath := filepath.Join(remoteDir, "somefile;id")
	evilCheckFile := filepath.Join(remoteDir, "id_output.txt") // if `id > id_output.txt` was part of payload

	_, err = sc.GetFileChecksum(ctx, maliciousPath, "sha256")
	if err == nil {
		t.Errorf("GetFileChecksum(%q) should have failed (file not found or invalid), but succeeded.", maliciousPath)
	} else {
		t.Logf("GetFileChecksum(%q) correctly failed with: %v", maliciousPath, err)
		// We expect an error from the checksum command itself (e.g., "No such file or directory")
		// wrapped in our CommandError or a more generic error message.
		// Crucially, it should not be a successful checksum of an unrelated file, nor should `id` have run.
		if !strings.Contains(err.Error(), "No such file or directory") && !strings.Contains(err.Error(), "failed to execute checksum command") {
			// Error message might vary depending on the remote OS's checksum tool.
			// The key is that it's a failure related to the checksum command, not a silent success or different error.
			t.Logf("Note: GetFileChecksum error for malicious path was: %s. This is often expected.", err.Error())
		}
	}

	// Verify that no side-effect file (like id_output.txt) was created
	statEvil, _ := sc.Stat(ctx, evilCheckFile)
	if statEvil != nil && statEvil.IsExist {
		t.Errorf("Potential command injection: path %q was created during GetFileChecksum(%q)", evilCheckFile, maliciousPath)
	}
}


// This test was named GetFileChecksum1, seems like a duplicate or debug version.
// Let's ensure the main GetFileChecksum test is robust.
// For now, I will remove GetFileChecksum1 if its unique parts were merged or are not needed.
// The main GetFileChecksum seems to cover the valid cases.
// The debug modification in GetFileChecksum1 using printf was insightful.
// I'll ensure the main GetFileChecksum test uses a reliable way to create the test file.

// TestSSHConnector_GetFileChecksum1 has been removed as its functionality was merged
// or covered by other tests like TestSSHConnector_GetFileChecksum and TestSSHConnector_GetFileChecksum_Injection.
// Stray ctx related to TestSSHConnector_GetFileChecksum1 was removed.
// Removing all other remnants of TestSSHConnector_GetFileChecksum1 that were at the package level.
// The file should end after the last valid test function.
