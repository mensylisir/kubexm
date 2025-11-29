package connector

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSSHConnector_Integration covers comprehensive SSH integration tests
// Uses real hosts: 192.168.56.101-103
func TestSSHConnector_Integration(t *testing.T) {
	hosts := []string{"192.168.56.101", "192.168.56.102", "192.168.56.103"}
	user := "mensyli1"
	password := "xiaoming98"
	keyPath := "/home/mensyli1/.ssh/id_rsa"
	rootPassword := "xiaoming98"

	keyContent, keyErr := os.ReadFile(keyPath)
	if keyErr != nil {
		t.Logf("Warning: Failed to read key file %s: %v", keyPath, keyErr)
	}

	ctx := context.Background()

	for _, host := range hosts {
		t.Run(fmt.Sprintf("Host_%s", host), func(t *testing.T) {
			// Test 1: Password Authentication
			t.Run("PasswordAuth", func(t *testing.T) {
				cfg := ConnectionCfg{
					Host:     host,
					Port:     22,
					User:     user,
					Password: password,
					Timeout:  10 * time.Second,
				}
				runComprehensiveSSHTests(t, ctx, cfg, "PasswordAuth")
			})

			// Test 2: Key Authentication
			t.Run("KeyAuth", func(t *testing.T) {
				if keyErr != nil {
					t.Skip("Skipping KeyAuth due to missing key file")
				}
				cfg := ConnectionCfg{
					Host:       host,
					Port:       22,
					User:       user,
					PrivateKey: keyContent,
					Password:   password, // Password might be needed for sudo even with key auth
					Timeout:    10 * time.Second,
				}
				runComprehensiveSSHTests(t, ctx, cfg, "KeyAuth")
			})

			// Test 3: Root Login
			t.Run("RootLogin", func(t *testing.T) {
				cfg := ConnectionCfg{
					Host:     host,
					Port:     22,
					User:     "root",
					Password: rootPassword,
					Timeout:  10 * time.Second,
				}
				conn := NewSSHConnector(nil)
				if err := conn.Connect(ctx, cfg); err != nil {
					t.Logf("Failed to connect as root to %s: %v", host, err)
					return
				}
				defer conn.Close()

				stdout, _, err := conn.Exec(ctx, "whoami", nil)
				if err != nil {
					t.Errorf("Exec as root failed: %v", err)
				}
				if strings.TrimSpace(string(stdout)) != "root" {
					t.Errorf("Expected whoami=root, got %s", string(stdout))
				}
			})
		})
	}
}

func runComprehensiveSSHTests(t *testing.T, ctx context.Context, cfg ConnectionCfg, testName string) {
	conn := NewSSHConnector(nil)
	if err := conn.Connect(ctx, cfg); err != nil {
		t.Fatalf("[%s] Failed to connect: %v", testName, err)
	}
	defer conn.Close()

	if !conn.IsConnected() {
		t.Errorf("[%s] IsConnected returned false", testName)
	}

	// 1. Basic Exec
	t.Run(testName+"/Exec", func(t *testing.T) {
		stdout, _, err := conn.Exec(ctx, "hostname", nil)
		if err != nil {
			t.Errorf("Exec failed: %v", err)
		}
		if len(stdout) == 0 {
			t.Error("Expected hostname output")
		}
	})

	// 2. Sudo Exec
	t.Run(testName+"/SudoExec", func(t *testing.T) {
		stdout, _, err := conn.Exec(ctx, "whoami", &ExecOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo exec failed: %v", err)
		}
		if !strings.Contains(string(stdout), "root") {
			t.Errorf("Expected root, got: %s", string(stdout))
		}
	})

	// 3. File Operations (Write, Read, Stat)
	t.Run(testName+"/FileOps", func(t *testing.T) {
		testFile := "/tmp/ssh_test_" + testName + ".txt"
		content := "ssh test content"

		// Write
		err := conn.WriteFile(ctx, []byte(content), testFile, nil)
		if err != nil {
			t.Errorf("WriteFile failed: %v", err)
		}
		defer conn.Remove(ctx, testFile, RemoveOptions{})

		// Read
		readContent, err := conn.ReadFileWithOptions(ctx, testFile, nil)
		if err != nil {
			t.Errorf("ReadFile failed: %v", err)
		}
		if string(readContent) != content {
			t.Errorf("Content mismatch")
		}

		// Stat
		stat, err := conn.Stat(ctx, testFile)
		if err != nil {
			t.Errorf("Stat failed: %v", err)
		}
		if stat == nil || !stat.IsExist {
			t.Error("File should exist")
		}
	})

	// 4. Sudo File Operations
	t.Run(testName+"/SudoFileOps", func(t *testing.T) {
		testFile := "/tmp/ssh_sudo_test_" + testName + ".txt"
		content := "ssh sudo content"

		// Write with sudo
		err := conn.WriteFile(ctx, []byte(content), testFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo WriteFile failed: %v", err)
		}
		defer conn.Remove(ctx, testFile, RemoveOptions{Sudo: true})

		// Read with sudo (required if file is root owned)
		readContent, err := conn.ReadFileWithOptions(ctx, testFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo ReadFile failed: %v", err)
		}
		if string(readContent) != content {
			t.Errorf("Content mismatch")
		}
		
		// CopyContent with sudo
		copyFile := "/tmp/ssh_sudo_copy_" + testName + ".txt"
		err = conn.CopyContent(ctx, []byte(content), copyFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo CopyContent failed: %v", err)
		}
		defer conn.Remove(ctx, copyFile, RemoveOptions{Sudo: true})
		
		// Verify copy
		readCopy, _ := conn.ReadFileWithOptions(ctx, copyFile, &FileTransferOptions{Sudo: true})
		if string(readCopy) != content {
			t.Errorf("CopyContent mismatch")
		}
	})
	
	// 5. Directory Operations
	t.Run(testName+"/DirOps", func(t *testing.T) {
		testDir := "/tmp/ssh_dir_" + testName
		err := conn.Mkdir(ctx, testDir, "0755")
		if err != nil {
			t.Errorf("Mkdir failed: %v", err)
		}
		defer conn.Remove(ctx, testDir, RemoveOptions{Recursive: true})
		
		isDir, err := conn.IsDir(ctx, testDir)
		if err != nil {
			t.Errorf("IsDir failed: %v", err)
		}
		if !isDir {
			t.Error("Should be a directory")
		}
	})
}
