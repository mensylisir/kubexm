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

// TestLocalConnector_ConvenienceMethods tests Run, Read, Write wrapper methods
func TestLocalConnector_ConvenienceMethods(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("Run", func(t *testing.T) {
		cmd := "echo 'test output'"
		if runtime.GOOS == "windows" {
			cmd = "echo test output"
		}
		
		result, err := conn.Run(ctx, cmd, nil)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		}
		if !strings.Contains(string(result.Stdout), "test output") {
			t.Errorf("Expected 'test output' in stdout, got: %s", string(result.Stdout))
		}
	})

	t.Run("Run_WithOptions", func(t *testing.T) {
		cmd := "echo $TEST_VAR"
		if runtime.GOOS == "windows" {
			cmd = "echo %TEST_VAR%"
		}
		
		opts := &RunOptions{
			Env: []string{"TEST_VAR=hello"},
		}
		result, err := conn.Run(ctx, cmd, opts)
		if err != nil {
			t.Errorf("Run with options failed: %v", err)
		}
		if !strings.Contains(string(result.Stdout), "hello") {
			t.Errorf("Expected 'hello' in output, got: %s", string(result.Stdout))
		}
	})

	t.Run("Read", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "read_test.txt")
		content := []byte("read test content")
		os.WriteFile(testFile, content, 0644)

		readContent, err := conn.Read(ctx, testFile, nil)
		if err != nil {
			t.Errorf("Read failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("Read_WithOptions", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "read_opts.txt")
		content := []byte("read with options")
		os.WriteFile(testFile, content, 0644)

		opts := &ReadOptions{
			Timeout: 5 * time.Second,
		}
		readContent, err := conn.Read(ctx, testFile, opts)
		if err != nil {
			t.Errorf("Read with options failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("Write", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "write_test.txt")
		content := []byte("write test content")

		err := conn.Write(ctx, content, testFile, nil)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}

		// Verify
		readContent, _ := os.ReadFile(testFile)
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("Write_WithOptions", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "write_opts.txt")
		content := []byte("write with options")

		opts := &WriteOptions{
			Permissions: "0600",
			Timeout:     5 * time.Second,
		}
		err := conn.Write(ctx, content, testFile, opts)
		if err != nil {
			t.Errorf("Write with options failed: %v", err)
		}

		// Verify content
		readContent, _ := os.ReadFile(testFile)
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}

		// Verify permissions (Unix only)
		if runtime.GOOS != "windows" {
			stat, _ := os.Stat(testFile)
			mode := stat.Mode().Perm()
			if mode != 0600 {
				t.Errorf("Expected permissions 0600, got %o", mode)
			}
		}
	})

	t.Run("GetConnectionConfig", func(t *testing.T) {
		cfg := conn.GetConnectionConfig()
		// For local connector, config should be empty/default
		if cfg.Host != "" {
			t.Errorf("Expected empty host for local connector, got: %s", cfg.Host)
		}
	})
}

// TestLocalConnector_FileMetadata tests GetFileMode, GetFileOwner, GetFileChecksum
func TestLocalConnector_FileMetadata(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("GetFileMode", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "mode_test.txt")
		os.WriteFile(testFile, []byte("test"), 0644)

		mode, err := conn.GetFileMode(ctx, testFile)
		if err != nil {
			t.Errorf("GetFileMode failed: %v", err)
		}
		if mode == 0 {
			t.Error("Expected non-zero file mode")
		}
	})

	t.Run("GetFileMode_NonExistent", func(t *testing.T) {
		_, err := conn.GetFileMode(ctx, "/nonexistent/file")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("GetFileOwner", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping GetFileOwner on Windows")
		}

		testFile := filepath.Join(t.TempDir(), "owner_test.txt")
		os.WriteFile(testFile, []byte("test"), 0644)

		owner, group, err := conn.GetFileOwner(ctx, testFile)
		if err != nil {
			t.Errorf("GetFileOwner failed: %v", err)
		}
		if owner == "" {
			t.Error("Expected non-empty owner")
		}
		if group == "" {
			t.Error("Expected non-empty group")
		}
	})

	t.Run("GetFileOwner_NonExistent", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping on Windows")
		}

		_, _, err := conn.GetFileOwner(ctx, "/nonexistent/file")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("GetFileChecksum_MD5", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "checksum_test.txt")
		content := []byte("test content for checksum")
		os.WriteFile(testFile, content, 0644)

		checksum, err := conn.GetFileChecksum(ctx, testFile, "md5")
		if err != nil {
			t.Errorf("GetFileChecksum MD5 failed: %v", err)
		}
		if checksum == "" {
			t.Error("Expected non-empty checksum")
		}
		// MD5 should be 32 hex characters
		if len(checksum) != 32 {
			t.Errorf("Expected MD5 checksum length 32, got %d", len(checksum))
		}
	})

	t.Run("GetFileChecksum_SHA256", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "checksum_sha.txt")
		content := []byte("test content for sha256")
		os.WriteFile(testFile, content, 0644)

		checksum, err := conn.GetFileChecksum(ctx, testFile, "sha256")
		if err != nil {
			t.Errorf("GetFileChecksum SHA256 failed: %v", err)
		}
		if checksum == "" {
			t.Error("Expected non-empty checksum")
		}
		// SHA256 should be 64 hex characters
		if len(checksum) != 64 {
			t.Errorf("Expected SHA256 checksum length 64, got %d", len(checksum))
		}
	})

	t.Run("GetFileChecksum_InvalidType", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "checksum_invalid.txt")
		os.WriteFile(testFile, []byte("test"), 0644)

		_, err := conn.GetFileChecksum(ctx, testFile, "invalid")
		if err == nil {
			t.Error("Expected error for invalid checksum type")
		}
	})

	t.Run("GetFileChecksum_NonExistent", func(t *testing.T) {
		_, err := conn.GetFileChecksum(ctx, "/nonexistent/file", "md5")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// TestLocalConnector_SystemInfo tests GetOS, GetOSRelease, LookPath
func TestLocalConnector_SystemInfo(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("GetOS", func(t *testing.T) {
		osInfo, err := conn.GetOS(ctx)
		if err != nil {
			t.Errorf("GetOS failed: %v", err)
		}
		if osInfo == nil {
			t.Fatal("Expected non-nil OS info")
		}
		if osInfo.Arch == "" {
			t.Error("Expected non-empty architecture")
		}

		// Test caching - second call should return same object
		osInfo2, err := conn.GetOS(ctx)
		if err != nil {
			t.Errorf("Second GetOS failed: %v", err)
		}
		if osInfo != osInfo2 {
			t.Error("GetOS should return cached object")
		}
	})

	t.Run("GetOSRelease", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping GetOSRelease on Windows")
		}

		release, err := conn.GetOSRelease(ctx)
		if err != nil {
			t.Errorf("GetOSRelease failed: %v", err)
		}
		if release == nil {
			t.Fatal("Expected non-nil release info")
		}
		// Should have at least some fields
		if len(release) == 0 {
			t.Error("Expected non-empty release info")
		}
	})

	t.Run("LookPath", func(t *testing.T) {
		// Look for a common executable
		executable := "ls"
		if runtime.GOOS == "windows" {
			executable = "cmd.exe"
		}

		path, err := conn.LookPath(ctx, executable)
		if err != nil {
			t.Errorf("LookPath failed: %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
		if !strings.Contains(path, executable) {
			t.Errorf("Expected path to contain %s, got: %s", executable, path)
		}
	})

	t.Run("LookPath_NonExistent", func(t *testing.T) {
		_, err := conn.LookPath(ctx, "nonexistent_executable_12345")
		if err == nil {
			t.Error("Expected error for non-existent executable")
		}
	})

	t.Run("LookPathWithOptions", func(t *testing.T) {
		executable := "ls"
		if runtime.GOOS == "windows" {
			executable = "cmd.exe"
		}

		opts := &LookPathOptions{
			Timeout: 5 * time.Second,
		}
		path, err := conn.LookPathWithOptions(ctx, executable, opts)
		if err != nil {
			t.Errorf("LookPathWithOptions failed: %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
	})

	t.Run("LookPathWithOptions_Sudo", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping sudo test on Windows")
		}

		// Set password for sudo
		cfg := ConnectionCfg{Password: "xiaoming98"}
		conn.Connect(ctx, cfg)

		opts := &LookPathOptions{
			Sudo:    true,
			Timeout: 5 * time.Second,
		}
		path, err := conn.LookPathWithOptions(ctx, "ls", opts)
		if err != nil {
			t.Skipf("Skipping sudo LookPath test: %v", err)
			return
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
	})
}

// TestLocalConnector_ReadFile tests the ReadFile wrapper
func TestLocalConnector_ReadFile(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("ReadFile", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "readfile_test.txt")
		content := []byte("readfile content")
		os.WriteFile(testFile, content, 0644)

		readContent, err := conn.ReadFile(ctx, testFile)
		if err != nil {
			t.Errorf("ReadFile failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("ReadFile_NonExistent", func(t *testing.T) {
		_, err := conn.ReadFile(ctx, "/nonexistent/file.txt")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// TestLocalConnector_IsFile_EdgeCases tests edge cases for IsFile
func TestLocalConnector_IsFile_EdgeCases(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("IsFile_Directory", func(t *testing.T) {
		testDir := t.TempDir()
		isFile, err := conn.IsFile(ctx, testDir)
		if err != nil {
			t.Errorf("IsFile failed: %v", err)
		}
		if isFile {
			t.Error("Directory should not be identified as file")
		}
	})

	t.Run("IsFile_NonExistent", func(t *testing.T) {
		isFile, err := conn.IsFile(ctx, "/nonexistent/path")
		if err != nil {
			t.Errorf("IsFile should not error on non-existent: %v", err)
		}
		if isFile {
			t.Error("Non-existent path should not be a file")
		}
	})
}
