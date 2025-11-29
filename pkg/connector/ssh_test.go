package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSSHConnector_Integration is the main entry point for SSH integration tests.
// It iterates through the specified hosts and authentication methods.
func TestSSHConnector_Integration(t *testing.T) {
	// User-provided test environment details
	hosts := []string{"192.168.56.101", "192.168.56.102", "192.168.56.103"}
	user := "mensyli1"
	password := "xiaoming98"
	keyPath := "/home/mensyli1/.ssh/id_rsa" // This path is on the agent's machine
	rootUser := "root"
	rootPassword := "xiaoming98"

	// Attempt to read the private key from the agent's environment.
	keyContent, keyErr := os.ReadFile(keyPath)
	if keyErr != nil {
		t.Logf("WARN: Could not read private key from '%s'. Key-based auth tests will be skipped. Error: %v", keyPath, keyErr)
	}

	ctx := context.Background()

	for _, host := range hosts {
		t.Run(fmt.Sprintf("Host_%s", host), func(t *testing.T) {
			t.Parallel() // Run tests for each host in parallel.

			// --- Test Suite 1: Connect as a regular user with a password ---
			t.Run("User_PasswordAuth", func(t *testing.T) {
				cfg := ConnectionCfg{
					Host:     host,
					Port:     22,
					User:     user,
					Password: password,
					Timeout:  15 * time.Second,
				}
				runComprehensiveSSHTests(t, ctx, cfg)
			})

			// --- Test Suite 2: Connect as a regular user with a private key ---
			t.Run("User_KeyAuth", func(t *testing.T) {
				if keyErr != nil {
					t.Skip("Skipping key-based auth test because private key is not available.")
				}
				cfg := ConnectionCfg{
					Host:           host,
					Port:           22,
					User:           user,
					PrivateKey:     keyContent,
					PrivateKeyPath: keyPath,      // For reference
					Password:       password,     // Password is still needed for sudo operations
					Timeout:        15 * time.Second,
				}
				runComprehensiveSSHTests(t, ctx, cfg)
			})

			// --- Test Suite 3: Connect as root with a password ---
			t.Run("Root_PasswordAuth", func(t *testing.T) {
				cfg := ConnectionCfg{
					Host:     host,
					Port:     22,
					User:     rootUser,
					Password: rootPassword,
					Timeout:  15 * time.Second,
				}
				runComprehensiveSSHTests(t, ctx, cfg)
			})
		})
	}
}

// runComprehensiveSSHTests executes a full suite of tests for a given SSH connection config.
func runComprehensiveSSHTests(t *testing.T, ctx context.Context, cfg ConnectionCfg) {
	conn := NewSSHConnector(nil)
	err := conn.Connect(ctx, cfg)
	if err != nil {
		t.Skipf("SKIPPING all tests for user '%s' on host '%s' due to connection failure: %v", cfg.User, cfg.Host, err)
		return
	}
	defer conn.Close()

	if !conn.IsConnected() {
		t.Fatal("IsConnected() returned false immediately after a successful connection.")
	}

	// Helper to create a unique remote path for each test to avoid conflicts.
	remotePath := func(name string) string {
		return fmt.Sprintf("/tmp/ssh_test-%s-%s-%d", cfg.User, name, time.Now().UnixNano())
	}

	// Helper function to verify file ownership and permissions with sudo.
	verifyRemoteFile := func(t *testing.T, path, expectedOwner, expectedPerms string) {
		t.Helper()
		// Use `stat` command to get file owner and permissions.
		statCmd := fmt.Sprintf("stat -c '%%U:%%G %%a' %s", path)
		stdout, stderr, err := conn.Exec(ctx, statCmd, &ExecOptions{Sudo: cfg.User != "root"})
		if err != nil {
			t.Fatalf("Failed to stat file %s: %v, stderr: %s", path, err, string(stderr))
		}
		output := strings.TrimSpace(string(stdout))
		parts := strings.Fields(output)
		if len(parts) != 2 {
			t.Fatalf("Unexpected stat output for %s: '%s'", path, output)
		}
		owner, perms := parts[0], parts[1]

		if !strings.HasPrefix(owner, expectedOwner) {
			t.Errorf("Path %s: Expected owner '%s', got '%s'", path, expectedOwner, owner)
		}
		if expectedPerms != "" && perms != expectedPerms {
			t.Errorf("Path %s: Expected permissions '%s', got '%s'", path, expectedPerms, perms)
		}
	}

	// --- Test Cases ---

	t.Run("Exec_Simple", func(t *testing.T) {
		stdout, _, err := conn.Exec(ctx, "whoami", nil)
		if err != nil {
			t.Fatalf("Exec('whoami') failed: %v", err)
		}
		if strings.TrimSpace(string(stdout)) != cfg.User {
			t.Errorf("Expected whoami to be '%s', got '%s'", cfg.User, string(stdout))
		}
	})

	t.Run("Exec_Sudo", func(t *testing.T) {
		if cfg.User == "root" {
			t.Skip("Sudo test not applicable for root user")
		}
		stdout, _, err := conn.Exec(ctx, "whoami", &ExecOptions{Sudo: true})
		if err != nil {
			t.Fatalf("Exec('sudo whoami') failed: %v", err)
		}
		if !strings.Contains(string(stdout), "root") {
			t.Errorf("Expected sudo whoami to contain 'root', got '%s'", string(stdout))
		}
	})

	t.Run("WriteFile_and_ReadFile", func(t *testing.T) {
		path := remotePath("write_read.txt")
		content := []byte("hello from ssh")
		err := conn.WriteFile(ctx, content, path, nil)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		defer conn.Remove(ctx, path, RemoveOptions{})

		readContent, err := conn.ReadFile(ctx, path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("WriteFile_Sudo_and_ReadFile_Sudo", func(t *testing.T) {
		path := remotePath("sudo_write_read.txt")
		content := []byte("sudo hello")
		opts := &FileTransferOptions{
			Sudo:        true,
			Permissions: "0640",
			Owner:       "root",
			Group:       "root",
		}
		err := conn.WriteFile(ctx, content, path, opts)
		if err != nil {
			t.Fatalf("Sudo WriteFile failed: %v", err)
		}
		defer conn.Remove(ctx, path, RemoveOptions{Sudo: true})

		verifyRemoteFile(t, path, "root:root", "640")

		readContent, err := conn.ReadFileWithOptions(ctx, path, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Fatalf("Sudo ReadFile failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("Upload_and_Download", func(t *testing.T) {
		localFile := filepath.Join(t.TempDir(), "upload.txt")
		content := []byte("upload/download test")
		os.WriteFile(localFile, content, 0644)

		path := remotePath("uploaded_file.txt")
		err := conn.Upload(ctx, localFile, path, nil)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
		defer conn.Remove(ctx, path, RemoveOptions{})

		downloadedFile := filepath.Join(t.TempDir(), "downloaded.txt")
		err = conn.Download(ctx, path, downloadedFile, nil)
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}

		readContent, _ := os.ReadFile(downloadedFile)
		if string(readContent) != string(content) {
			t.Errorf("Downloaded content mismatch")
		}
	})

	t.Run("Mkdir_and_Remove", func(t *testing.T) {
		path := remotePath("test_dir")
		err := conn.Mkdir(ctx, path, "0755")
		if err != nil {
			t.Fatalf("Mkdir failed: %v", err)
		}

		isDir, err := conn.IsDir(ctx, path)
		if err != nil || !isDir {
			t.Errorf("IsDir should be true for created directory")
		}

		err = conn.Remove(ctx, path, RemoveOptions{Recursive: true})
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		stat, err := conn.Stat(ctx, path)
		if err != nil || stat.IsExist {
			t.Errorf("Directory should not exist after Remove")
		}
	})

	t.Run("GetFileChecksum", func(t *testing.T) {
		path := remotePath("checksum.txt")
		content := "checksum content"
		conn.WriteFile(ctx, []byte(content), path, nil)
		defer conn.Remove(ctx, path, RemoveOptions{})

		// Expected sha256sum for "checksum content"
		expected := "5a1c2a02322d42d22631b67137f814426d5b066a3d82342de45c5722dc6f8314"
		checksum, err := conn.GetFileChecksum(ctx, path, "sha256")
		if err != nil {
			t.Fatalf("GetFileChecksum failed: %v", err)
		}
		if checksum != expected {
			t.Errorf("Checksum mismatch: expected %s, got %s", expected, checksum)
		}
	})
}
