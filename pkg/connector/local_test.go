package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLocalConnector_Basic covers basic execution and file operations
func TestLocalConnector_Basic(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create local connector: %v", err)
	}

	t.Run("Simple Command", func(t *testing.T) {
		cmd := "echo 'hello world'"
		if runtime.GOOS == "windows" {
			cmd = "echo hello world"
		}
		stdout, _, err := conn.Exec(ctx, cmd, nil)
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		output := strings.TrimSpace(string(stdout))
		if output != "hello world" {
			t.Errorf("Expected 'hello world', got '%s'", output)
		}
	})

	t.Run("Environment Variables", func(t *testing.T) {
		cmd := "echo $MY_VAR"
		if runtime.GOOS == "windows" {
			cmd = "echo %MY_VAR%"
		}
		opts := &ExecOptions{
			Env: []string{"MY_VAR=test_value"},
		}
		stdout, _, err := conn.Exec(ctx, cmd, opts)
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		output := strings.TrimSpace(string(stdout))
		if output != "test_value" {
			t.Errorf("Expected 'test_value', got '%s'", output)
		}
	})

	t.Run("Working Directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := "pwd"
		if runtime.GOOS == "windows" {
			cmd = "cd"
		}
		opts := &ExecOptions{
			Dir: tmpDir,
		}
		stdout, _, err := conn.Exec(ctx, cmd, opts)
		if err != nil {
			t.Fatalf("Exec failed: %v", err)
		}
		output := strings.TrimSpace(string(stdout))
		if !strings.Contains(strings.ToLower(output), strings.ToLower(filepath.Base(tmpDir))) {
			t.Errorf("Expected output to contain %s, got %s", filepath.Base(tmpDir), output)
		}
	})
}

// TestLocalConnector_FileOperations covers standard file operations (non-sudo)
func TestLocalConnector_FileOperations(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create local connector: %v", err)
	}

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.txt")
	dstFile := filepath.Join(tmpDir, "dst.txt")
	content := []byte("test content")

	// 1. WriteFile
	t.Run("WriteFile", func(t *testing.T) {
		err := conn.WriteFile(ctx, content, srcFile, nil)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		// Verify content
		readContent, err := os.ReadFile(srcFile)
		if err != nil {
			t.Fatalf("Failed to read file locally: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch. Expected %s, got %s", content, readContent)
		}
	})

	// 2. ReadFile
	t.Run("ReadFile", func(t *testing.T) {
		readContent, err := conn.ReadFileWithOptions(ctx, srcFile, nil)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch. Expected %s, got %s", content, readContent)
		}
	})

	// 3. Copy
	t.Run("Copy", func(t *testing.T) {
		err := conn.Copy(ctx, srcFile, dstFile, nil)
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}
		// Verify content
		readContent, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("Failed to read file locally: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch. Expected %s, got %s", content, readContent)
		}
	})

	// 4. Stat
	t.Run("Stat", func(t *testing.T) {
		stat, err := conn.Stat(ctx, srcFile)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if stat == nil {
			t.Fatal("Stat returned nil")
		}
		if !stat.IsExist {
			t.Error("File should exist")
		}
		if stat.IsDir {
			t.Error("Should not be a directory")
		}
		if stat.Size != int64(len(content)) {
			t.Errorf("Size mismatch. Expected %d, got %d", len(content), stat.Size)
		}
	})

	// 5. IsFile
	t.Run("IsFile", func(t *testing.T) {
		isFile, err := conn.IsFile(ctx, srcFile)
		if err != nil {
			t.Fatalf("IsFile failed: %v", err)
		}
		if !isFile {
			t.Error("Should be a file")
		}
	})
}

// TestLocalConnector_ExecComprehensive covers complex command scenarios
func TestLocalConnector_ExecComprehensive(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("FileOperations", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "exec_test_file.txt")
		testContent := "test content for exec"
		
		// Write file using echo and redirection
		cmd := fmt.Sprintf("echo \"%s\" > %s", testContent, testFile)
		_, _, err := conn.Exec(ctx, cmd, nil)
		if err != nil {
			t.Errorf("Failed to create file: %v", err)
		}
		
		// Read file using cat
		stdout, _, err := conn.Exec(ctx, "cat "+testFile, nil)
		if err != nil {
			t.Errorf("Failed to read file: %v", err)
		}
		if !strings.Contains(string(stdout), testContent) {
			t.Errorf("File content mismatch")
		}
		
		// Append to file
		_, _, err = conn.Exec(ctx, fmt.Sprintf("echo \"appended\" >> %s", testFile), nil)
		if err != nil {
			t.Errorf("Failed to append to file: %v", err)
		}
		
		// Count lines
		stdout, _, err = conn.Exec(ctx, "wc -l "+testFile, nil)
		if err != nil {
			t.Errorf("Failed to count lines: %v", err)
		}
		if !strings.Contains(string(stdout), "2") {
			t.Errorf("Expected 2 lines, got: %s", string(stdout))
		}
	})

	t.Run("ProcessManagement", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping process management tests on Windows")
		}
		// List processes
		stdout, _, err := conn.Exec(ctx, "ps aux | head -5", nil)
		if err != nil {
			t.Errorf("Failed to list processes: %v", err)
		}
		if len(stdout) == 0 {
			t.Error("Expected process list output")
		}
	})
}

// TestLocalConnector_SudoComprehensive covers all sudo operations
// Requires password "xiaoming98" for user "mensyli1" in the environment
func TestLocalConnector_SudoComprehensive(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	// Set password for sudo operations
	// NOTE: This assumes the test environment matches the user's description
	// If running in an environment without this user/pass, these tests might fail or need skipping
	cfg := ConnectionCfg{Password: "xiaoming98"}
	conn.Connect(ctx, cfg)
	
	// Verify we can run sudo
	_, _, err = conn.Exec(ctx, "sudo -S -p '' -E -- true", &ExecOptions{Sudo: true})
	if err != nil {
		t.Skipf("Skipping sudo tests: failed to verify sudo access (check password/permissions): %v", err)
		return
	}

	t.Run("SudoUpload", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "upload_src.txt")
		content := []byte("sudo upload test")
		os.WriteFile(srcFile, content, 0644)
		
		dstFile := "/tmp/sudo_upload_test.txt"
		err := conn.Upload(ctx, srcFile, dstFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo upload failed: %v", err)
		}
		defer conn.Remove(ctx, dstFile, RemoveOptions{Sudo: true})
		
		// Verify file exists and owned by root
		stdout, _, _ := conn.Exec(ctx, "ls -l "+dstFile, &ExecOptions{Sudo: true})
		if !strings.Contains(string(stdout), "root") {
			t.Errorf("Uploaded file should be owned by root")
		}
	})
	
	t.Run("SudoDownload", func(t *testing.T) {
		srcFile := "/tmp/sudo_download_src.txt"
		content := "sudo download test"
		conn.WriteFile(ctx, []byte(content), srcFile, &FileTransferOptions{Sudo: true})
		defer conn.Remove(ctx, srcFile, RemoveOptions{Sudo: true})
		
		dstFile := filepath.Join(t.TempDir(), "download_dst.txt")
		err := conn.Download(ctx, srcFile, dstFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo download failed: %v", err)
		}
		
		// Verify content (use sudo read because file might be 0600 root)
		readContent, err := conn.ReadFileWithOptions(ctx, dstFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Failed to verify download: %v", err)
		}
		if string(readContent) != content {
			t.Errorf("Downloaded content mismatch")
		}
	})
	
	t.Run("SudoCopyContent", func(t *testing.T) {
		content := []byte("sudo copy content test")
		dstFile := "/tmp/sudo_copy_content.txt"
		
		err := conn.CopyContent(ctx, content, dstFile, &FileTransferOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo CopyContent failed: %v", err)
		}
		defer conn.Remove(ctx, dstFile, RemoveOptions{Sudo: true})
		
		// Verify
		readContent, _ := conn.ReadFileWithOptions(ctx, dstFile, &FileTransferOptions{Sudo: true})
		if string(readContent) != string(content) {
			t.Errorf("CopyContent content mismatch")
		}
	})
	
	t.Run("SudoStat", func(t *testing.T) {
		testFile := "/tmp/sudo_stat_test.txt"
		conn.WriteFile(ctx, []byte("test"), testFile, &FileTransferOptions{Sudo: true})
		defer conn.Remove(ctx, testFile, RemoveOptions{Sudo: true})
		
		stat, err := conn.StatWithOptions(ctx, testFile, &StatOptions{Sudo: true})
		if err != nil {
			t.Errorf("Sudo Stat failed: %v", err)
		}
		if stat == nil || !stat.IsExist {
			t.Error("Stat should show file exists")
		}
	})

	t.Run("SudoMkdirRemove", func(t *testing.T) {
		testDir := "/tmp/sudo_isdir_test"
		conn.Mkdir(ctx, testDir, "0755")
		
		isDir, err := conn.IsDir(ctx, testDir)
		if err != nil {
			t.Errorf("Sudo IsDir failed: %v", err)
		}
		if !isDir {
			t.Error("Should be a directory")
		}
		
		conn.Remove(ctx, testDir, RemoveOptions{Sudo: true, Recursive: true})
		
		isDir, _ = conn.IsDir(ctx, testDir)
		if isDir {
			t.Error("Directory should be removed")
		}
	})
}

// TestLocalConnector_EdgeCases covers edge cases and low coverage areas
func TestLocalConnector_EdgeCases(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}

	t.Run("Copy_EdgeCases", func(t *testing.T) {
		// Test Copy with non-existent source
		err := conn.Copy(ctx, "/nonexistent/src", "/tmp/dst", nil)
		if err == nil {
			t.Error("Expected error for non-existent source")
		}

		// Test Copy with same source and dest
		tmpFile := filepath.Join(t.TempDir(), "same_file.txt")
		os.WriteFile(tmpFile, []byte("test"), 0644)
		err = conn.Copy(ctx, tmpFile, tmpFile, nil)
		if err == nil {
			t.Error("Expected error for same source and dest")
		}
	})

	t.Run("StatWithOptions_EdgeCases", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "stat_test.txt")
		os.WriteFile(tmpFile, []byte("test"), 0644)
		
		// Test with nil options
		stat, err := conn.StatWithOptions(ctx, tmpFile, nil)
		if err != nil {
			t.Errorf("StatWithOptions(nil) failed: %v", err)
		}
		if stat == nil {
			t.Error("Stat result is nil")
		}
		
		// Test with timeout
		opts := &StatOptions{Timeout: 1 * time.Second}
		stat, err = conn.StatWithOptions(ctx, tmpFile, opts)
		if err != nil {
			t.Errorf("StatWithOptions(timeout) failed: %v", err)
		}
		
		// Test non-existent with options
		stat, err = conn.StatWithOptions(ctx, "/nonexistent", opts)
		if err != nil {
			t.Errorf("StatWithOptions(/nonexistent) failed: %v", err)
		}
		if stat.IsExist {
			t.Error("Expected IsExist=false for non-existent file")
		}
	})

	t.Run("GetOS_Caching", func(t *testing.T) {
		// First call
		os1, err := conn.GetOS(ctx)
		if err != nil {
			t.Errorf("First GetOS failed: %v", err)
		}
		
		// Second call (should use cache)
		os2, err := conn.GetOS(ctx)
		if err != nil {
			t.Errorf("Second GetOS failed: %v", err)
		}
		
		if os1 != os2 {
			t.Error("GetOS should return cached object")
		}
	})

	t.Run("ReadFileWithOptions_EdgeCases", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "read_opts.txt")
		os.WriteFile(tmpFile, []byte("content"), 0644)
		
		// Test with nil options
		content, err := conn.ReadFileWithOptions(ctx, tmpFile, nil)
		if err != nil {
			t.Errorf("ReadFileWithOptions(nil) failed: %v", err)
		}
		if string(content) != "content" {
			t.Error("Content mismatch")
		}
	})
	
	t.Run("CopyContent_EdgeCases", func(t *testing.T) {
		dstFile := filepath.Join(t.TempDir(), "copy_content.txt")
		content := []byte("test content")
		
		// Test with nil options
		err := conn.CopyContent(ctx, content, dstFile, nil)
		if err != nil {
			t.Errorf("CopyContent(nil) failed: %v", err)
		}
		
		// Verify
		read, _ := os.ReadFile(dstFile)
		if string(read) != string(content) {
			t.Error("Content mismatch")
		}
		
		// Test with invalid permissions
		err = conn.CopyContent(ctx, content, dstFile, &FileTransferOptions{Permissions: "invalid"})
		if err == nil {
			t.Error("Expected error for invalid permissions")
		}
	})
}

// TestLocalConnector_ConvenienceMethods tests Run, Read, Write, Copy wrapper methods
func TestLocalConnector_ConvenienceMethods(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	t.Run("Run", func(t *testing.T) {
		result, err := conn.Run(ctx, "echo test", nil)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		}
		if !strings.Contains(string(result.Stdout), "test") {
			t.Error("Expected stdout to contain 'test'")
		}
	})
	
	t.Run("Run_WithOptions", func(t *testing.T) {
		result, err := conn.Run(ctx, "echo $TEST_VAR", &RunOptions{
			Env: []string{"TEST_VAR=hello"},
		})
		if err != nil {
			t.Errorf("Run with options failed: %v", err)
		}
		if !strings.Contains(string(result.Stdout), "hello") {
			t.Error("Expected stdout to contain 'hello'")
		}
	})
	
	t.Run("Run_Error", func(t *testing.T) {
		result, err := conn.Run(ctx, "false", nil)
		if err == nil {
			t.Error("Expected error for failed command")
		}
		if result.ExitCode == 0 {
			t.Error("Expected non-zero exit code")
		}
	})
	
	t.Run("Read", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "read_test.txt")
		content := []byte("read test content")
		os.WriteFile(tmpFile, content, 0644)
		
		data, err := conn.Read(ctx, tmpFile, nil)
		if err != nil {
			t.Errorf("Read failed: %v", err)
		}
		if string(data) != string(content) {
			t.Error("Content mismatch")
		}
	})
	
	t.Run("Read_WithOptions", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "read_opts.txt")
		content := []byte("read options test")
		os.WriteFile(tmpFile, content, 0644)
		
		data, err := conn.Read(ctx, tmpFile, &ReadOptions{Timeout: 5 * time.Second})
		if err != nil {
			t.Errorf("Read with options failed: %v", err)
		}
		if string(data) != string(content) {
			t.Error("Content mismatch")
		}
	})
	
	t.Run("Write", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "write_test.txt")
		content := []byte("write test content")
		
		err := conn.Write(ctx, content, tmpFile, nil)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		
		data, _ := os.ReadFile(tmpFile)
		if string(data) != string(content) {
			t.Error("Content mismatch")
		}
	})
	
	t.Run("Write_WithOptions", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "write_opts.txt")
		content := []byte("write options test")
		
		err := conn.Write(ctx, content, tmpFile, &WriteOptions{
			Permissions: "0600",
			Timeout:     5 * time.Second,
		})
		if err != nil {
			t.Errorf("Write with options failed: %v", err)
		}
		
		fi, _ := os.Stat(tmpFile)
		if fi.Mode().Perm() != 0600 {
			t.Errorf("Expected permissions 0600, got %o", fi.Mode().Perm())
		}
	})
}

// TestLocalConnector_HelperMethods tests IsFile, IsDir, GetFileMode, GetFileOwner, GetOSRelease
func TestLocalConnector_HelperMethods(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping helper method tests on Windows")
	}
	
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_file.txt")
	os.WriteFile(tmpFile, []byte("test"), 0644)
	
	t.Run("IsFile_True", func(t *testing.T) {
		isFile, err := conn.IsFile(ctx, tmpFile)
		if err != nil {
			t.Errorf("IsFile failed: %v", err)
		}
		if !isFile {
			t.Error("Expected IsFile to return true")
		}
	})
	
	t.Run("IsFile_False", func(t *testing.T) {
		isFile, err := conn.IsFile(ctx, tmpDir)
		if err != nil {
			t.Errorf("IsFile failed: %v", err)
		}
		if isFile {
			t.Error("Expected IsFile to return false for directory")
		}
	})
	
	t.Run("IsFile_NotExist", func(t *testing.T) {
		isFile, err := conn.IsFile(ctx, "/nonexistent/path")
		if err != nil {
			t.Errorf("IsFile should not error for non-existent path: %v", err)
		}
		if isFile {
			t.Error("Expected IsFile to return false for non-existent path")
		}
	})
	
	t.Run("IsDir_True", func(t *testing.T) {
		isDir, err := conn.IsDir(ctx, tmpDir)
		if err != nil {
			t.Errorf("IsDir failed: %v", err)
		}
		if !isDir {
			t.Error("Expected IsDir to return true")
		}
	})
	
	t.Run("IsDir_False", func(t *testing.T) {
		isDir, err := conn.IsDir(ctx, tmpFile)
		if err != nil {
			t.Errorf("IsDir failed: %v", err)
		}
		if isDir {
			t.Error("Expected IsDir to return false for file")
		}
	})
	
	t.Run("IsDir_NotExist", func(t *testing.T) {
		isDir, err := conn.IsDir(ctx, "/nonexistent/path")
		if err != nil {
			t.Errorf("IsDir should not error for non-existent path: %v", err)
		}
		if isDir {
			t.Error("Expected IsDir to return false for non-existent path")
		}
	})
	
	t.Run("GetFileMode", func(t *testing.T) {
		mode, err := conn.GetFileMode(ctx, tmpFile)
		if err != nil {
			t.Errorf("GetFileMode failed: %v", err)
		}
		if mode == 0 {
			t.Error("Expected non-zero file mode")
		}
	})
	
	t.Run("GetFileMode_Error", func(t *testing.T) {
		_, err := conn.GetFileMode(ctx, "/nonexistent/path")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})
	
	t.Run("GetFileOwner", func(t *testing.T) {
		owner, group, err := conn.GetFileOwner(ctx, tmpFile)
		if err != nil {
			t.Errorf("GetFileOwner failed: %v", err)
		}
		if owner == "" || group == "" {
			t.Error("Expected non-empty owner and group")
		}
	})
	
	t.Run("GetFileOwner_Error", func(t *testing.T) {
		_, _, err := conn.GetFileOwner(ctx, "/nonexistent/path")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})
	
	t.Run("GetOSRelease", func(t *testing.T) {
		osRelease, err := conn.GetOSRelease(ctx)
		if err != nil {
			t.Errorf("GetOSRelease failed: %v", err)
		}
		if len(osRelease) == 0 {
			t.Error("Expected non-empty OS release info")
		}
	})
}

// TestLocalConnector_LookPath tests LookPath and LookPathWithOptions
func TestLocalConnector_LookPath(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	t.Run("LookPath_Success", func(t *testing.T) {
		path, err := conn.LookPath(ctx, "sh")
		if err != nil {
			t.Errorf("LookPath failed: %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
	})
	
	t.Run("LookPath_NotFound", func(t *testing.T) {
		_, err := conn.LookPath(ctx, "nonexistent_command_12345")
		if err == nil {
			t.Error("Expected error for non-existent command")
		}
	})
	
	t.Run("LookPathWithOptions_Success", func(t *testing.T) {
		path, err := conn.LookPathWithOptions(ctx, "sh", &LookPathOptions{})
		if err != nil {
			t.Errorf("LookPathWithOptions failed: %v", err)
		}
		if path == "" {
			t.Error("Expected non-empty path")
		}
	})
	
	t.Run("LookPathWithOptions_InvalidChars", func(t *testing.T) {
		_, err := conn.LookPathWithOptions(ctx, "sh; ls", &LookPathOptions{})
		if err == nil {
			t.Error("Expected error for invalid characters")
		}
	})
	
	t.Run("LookPathWithOptions_NotFound", func(t *testing.T) {
		_, err := conn.LookPathWithOptions(ctx, "nonexistent_cmd", &LookPathOptions{})
		if err == nil {
			t.Error("Expected error for non-existent command")
		}
	})
}

// TestLocalConnector_Checksum tests GetFileChecksum
func TestLocalConnector_Checksum(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	tmpFile := filepath.Join(t.TempDir(), "checksum_test.txt")
	content := []byte("test content for checksum")
	os.WriteFile(tmpFile, content, 0644)
	
	t.Run("SHA256", func(t *testing.T) {
		checksum, err := conn.GetFileChecksum(ctx, tmpFile, "sha256")
		if err != nil {
			t.Errorf("GetFileChecksum(sha256) failed: %v", err)
		}
		if checksum == "" {
			t.Error("Expected non-empty checksum")
		}
		if len(checksum) != 64 { // SHA256 produces 64 hex characters
			t.Errorf("Expected 64 character SHA256, got %d", len(checksum))
		}
	})
	
	t.Run("MD5", func(t *testing.T) {
		checksum, err := conn.GetFileChecksum(ctx, tmpFile, "md5")
		if err != nil {
			t.Errorf("GetFileChecksum(md5) failed: %v", err)
		}
		if checksum == "" {
			t.Error("Expected non-empty checksum")
		}
		if len(checksum) != 32 { // MD5 produces 32 hex characters
			t.Errorf("Expected 32 character MD5, got %d", len(checksum))
		}
	})
	
	t.Run("UnsupportedType", func(t *testing.T) {
		_, err := conn.GetFileChecksum(ctx, tmpFile, "unsupported")
		if err == nil {
			t.Error("Expected error for unsupported checksum type")
		}
	})
	
	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := conn.GetFileChecksum(ctx, "/nonexistent/file", "sha256")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// TestLocalConnector_Timeouts tests timeout and retry logic
func TestLocalConnector_Timeouts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping timeout tests on Windows")
	}
	
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	t.Run("Timeout", func(t *testing.T) {
		_, _, err := conn.Exec(ctx, "sleep 10", &ExecOptions{
			Timeout: 100 * time.Millisecond,
		})
		if err == nil {
			t.Error("Expected timeout error")
		}
	})
	
	t.Run("Retries", func(t *testing.T) {
		// This command will fail but should retry
		_, _, err := conn.Exec(ctx, "false", &ExecOptions{
			Retries:    2,
			RetryDelay: 10 * time.Millisecond,
		})
		if err == nil {
			t.Error("Expected error after retries")
		}
	})
	
	t.Run("ContextCancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately
		
		_, _, err := conn.Exec(cancelCtx, "echo test", nil)
		if err == nil {
			t.Error("Expected error for cancelled context")
		}
	})
}

// TestLocalConnector_AllInterfaceMethods verifies all Connector interface methods are implemented
func TestLocalConnector_AllInterfaceMethods(t *testing.T) {
	var _ Connector = &LocalConnector{}
	t.Log("LocalConnector implements all Connector interface methods")
}

// TestLocalConnector_GetConnectionConfig tests GetConnectionConfig
func TestLocalConnector_GetConnectionConfig(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	cfg := ConnectionCfg{
		Host:     "localhost",
		User:     "testuser",
		Password: "testpass",
	}
	conn.Connect(ctx, cfg)
	
	retrievedCfg := conn.GetConnectionConfig()
	if retrievedCfg.Host != cfg.Host {
		t.Errorf("Expected host %s, got %s", cfg.Host, retrievedCfg.Host)
	}
	if retrievedCfg.User != cfg.User {
		t.Errorf("Expected user %s, got %s", cfg.User, retrievedCfg.User)
	}
	if retrievedCfg.Password != cfg.Password {
		t.Errorf("Expected password %s, got %s", cfg.Password, retrievedCfg.Password)
	}
}

// TestLocalConnector_Fetch tests Fetch method (alias for Copy)
func TestLocalConnector_Fetch(t *testing.T) {
	ctx := context.Background()
	conn, err := NewLocalConnector()
	if err != nil {
		t.Fatalf("Failed to create LocalConnector: %v", err)
	}
	
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "fetch_src.txt")
	dstFile := filepath.Join(tmpDir, "fetch_dst.txt")
	content := []byte("fetch test content")
	os.WriteFile(srcFile, content, 0644)
	
	err = conn.Fetch(ctx, srcFile, dstFile, nil)
	if err != nil {
		t.Errorf("Fetch failed: %v", err)
	}
	
	data, _ := os.ReadFile(dstFile)
	if string(data) != string(content) {
		t.Error("Content mismatch after fetch")
	}
}
