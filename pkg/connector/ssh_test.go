package connector

import (
	"context"
	"fmt"
	"os"
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
	sshTestHost         = "localhost"
	sshTestUser         = os.Getenv("SSH_TEST_USER")
	sshTestPassword     = os.Getenv("SSH_TEST_PASSWORD")
	sshTestPrivKeyPath  = os.Getenv("SSH_TEST_PRIV_KEY_PATH")
	sshTestPortStr      = os.Getenv("SSH_TEST_PORT")
	sshTestPort         = 22 // Default SSH port
	sshTestTimeout      = 10 * time.Second
	enableSshTests      = os.Getenv("ENABLE_SSH_CONNECTOR_TESTS") == "true"
)

func setupSSHTest(t *testing.T) *SSHConnector {
	if !enableSshTests {
		t.Skip("SSHConnector tests are disabled. Set ENABLE_SSH_CONNECTOR_TESTS=true to run them.")
	}

	if sshTestUser == "" {
		currentUser, err := os.UserHomeDir() // A bit of a hack to get a username if not set
		if err != nil {
			t.Fatal("SSH_TEST_USER not set and cannot determine current user")
		}
		sshTestUser = filepath.Base(currentUser) // often the username
		if sshTestUser == "." || sshTestUser == "/" { // if UserHomeDir returns something like "/home"
			u, _ := os.UserCurrent()
			if u != nil && u.Username != ""{
				sshTestUser = u.Username
			} else {
				t.Fatal("SSH_TEST_USER not set and could not reliably determine current user")
			}
		}
		fmt.Printf("SSH_TEST_USER not set, defaulting to: %s. Please ensure this user can SSH to localhost.\n", sshTestUser)
	}
	if sshTestPassword == "" && sshTestPrivKeyPath == "" {
		// Attempt to use default key if no auth method is specified
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultKey := filepath.Join(homeDir, ".ssh", "id_rsa")
			if _, err := os.Stat(defaultKey); err == nil {
				sshTestPrivKeyPath = defaultKey
				fmt.Printf("SSH_TEST_PASSWORD and SSH_TEST_PRIV_KEY_PATH not set, defaulting to use private key: %s\n", sshTestPrivKeyPath)
			} else {
				t.Fatal("SSH_TEST_PASSWORD or SSH_TEST_PRIV_KEY_PATH must be set for SSH tests if $HOME/.ssh/id_rsa doesn't exist.")
			}
		} else {
			t.Fatal("SSH_TEST_PASSWORD or SSH_TEST_PRIV_KEY_PATH must be set for SSH tests.")
		}
	}


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

	// 2. Test Copy
	err = sc.Copy(ctx, localSrcFilePath, remoteDstFilePath, &FileTransferOptions{Permissions: "0600"})
	if err != nil {
		t.Fatalf("Copy() to %s error = %v", remoteDstFilePath, err)
	}
	stdoutCopy, _, errCopy := sc.Exec(ctx, "cat "+remoteDstFilePath, nil)
	if errCopy != nil {
		t.Fatalf("Exec cat after Copy error = %v", errCopy)
	}
	if string(stdoutCopy) != string(fileContent) {
		t.Errorf("Copy() content mismatch: got %q, want %q", string(stdoutCopy), string(fileContent))
	}
	// TODO: Verify permissions


	// 3. Test Fetch
	err = sc.Fetch(ctx, remoteDstFilePath, localFetchFilePath)
	if err != nil {
		t.Fatalf("Fetch() from %s error = %v", remoteDstFilePath, err)
	}
	readFetch, _ := os.ReadFile(localFetchFilePath)
	if string(readFetch) != string(fileContent) {
		t.Errorf("Fetch() content mismatch: got %q, want %q", string(readFetch), string(fileContent))
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
