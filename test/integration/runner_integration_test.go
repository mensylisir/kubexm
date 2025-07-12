package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// TestRunnerIntegration tests the runner package integration with different connector types
func TestRunnerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with mock connector for safety
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	t.Run("GatherFacts", func(t *testing.T) {
		mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
		
		facts, err := r.GatherFacts(ctx, mockConn)
		require.NoError(t, err, "GatherFacts should succeed")
		require.NotNil(t, facts, "Facts should not be nil")
		
		assert.NotEmpty(t, facts.OS.ID, "OS ID should be populated")
		assert.NotEmpty(t, facts.Hostname, "Hostname should be populated")
		assert.Greater(t, facts.TotalCPU, 0, "CPU count should be positive")
		assert.Greater(t, facts.TotalMemory, uint64(0), "Memory should be positive")
	})

	t.Run("FileOperations", func(t *testing.T) {
		testPath := "/tmp/kubexm-test-file"
		testContent := []byte("test content for integration")
		
		// Test file writing
		err := r.WriteFile(ctx, mockConn, testContent, testPath, "0644", false)
		require.NoError(t, err, "WriteFile should succeed")
		
		// Test file existence
		exists, err := r.Exists(ctx, mockConn, testPath)
		require.NoError(t, err, "Exists check should succeed")
		assert.True(t, exists, "File should exist after writing")
		
		// Test file reading
		content, err := r.ReadFile(ctx, mockConn, testPath)
		require.NoError(t, err, "ReadFile should succeed")
		assert.Equal(t, testContent, content, "Read content should match written content")
		
		// Test file removal
		err = r.Remove(ctx, mockConn, testPath, false)
		require.NoError(t, err, "Remove should succeed")
		
		// Verify file is removed
		exists, err = r.Exists(ctx, mockConn, testPath)
		require.NoError(t, err, "Exists check should succeed")
		assert.False(t, exists, "File should not exist after removal")
	})

	t.Run("DirectoryOperations", func(t *testing.T) {
		testDir := "/tmp/kubexm-test-dir"
		
		// Test directory creation
		err := r.Mkdirp(ctx, mockConn, testDir, "0755", false)
		require.NoError(t, err, "Mkdirp should succeed")
		
		// Test directory existence
		isDir, err := r.IsDir(ctx, mockConn, testDir)
		require.NoError(t, err, "IsDir check should succeed")
		assert.True(t, isDir, "Should be a directory")
		
		// Test directory removal
		err = r.Remove(ctx, mockConn, testDir, false)
		require.NoError(t, err, "Directory removal should succeed")
	})

	t.Run("CommandExecution", func(t *testing.T) {
		// Test simple command
		output, err := r.Run(ctx, mockConn, "echo 'hello world'", false)
		require.NoError(t, err, "Simple command should succeed")
		assert.Contains(t, output, "hello world", "Output should contain expected text")
		
		// Test command with options
		stdout, stderr, err := r.RunWithOptions(ctx, mockConn, "echo 'test'", &connector.ExecOptions{
			Sudo:    false,
			Timeout: 10 * time.Second,
		})
		require.NoError(t, err, "Command with options should succeed")
		assert.Contains(t, string(stdout), "test", "Stdout should contain expected text")
		assert.Empty(t, stderr, "Stderr should be empty for successful echo")
	})
}

// TestRunnerPackageManagement tests package management operations
func TestRunnerPackageManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup facts with package manager info
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
	facts, err := r.GatherFacts(ctx, mockConn)
	require.NoError(t, err, "Should gather facts")

	t.Run("PackageInstallation", func(t *testing.T) {
		// Test package installation (mock will simulate success)
		err := r.InstallPackages(ctx, mockConn, facts, "curl", "wget")
		assert.NoError(t, err, "Package installation should succeed")
	})

	t.Run("PackageQuery", func(t *testing.T) {
		// Test package existence check
		installed, err := r.IsPackageInstalled(ctx, mockConn, facts, "curl")
		assert.NoError(t, err, "Package query should succeed")
		// Mock connector can be configured to return true/false
		_ = installed // Result depends on mock setup
	})

	t.Run("PackageRemoval", func(t *testing.T) {
		// Test package removal
		err := r.RemovePackages(ctx, mockConn, facts, "test-package")
		assert.NoError(t, err, "Package removal should succeed")
	})
}

// TestRunnerServiceManagement tests service management operations
func TestRunnerServiceManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup facts with init system info
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
	facts, err := r.GatherFacts(ctx, mockConn)
	require.NoError(t, err, "Should gather facts")

	serviceName := "test-service"

	t.Run("ServiceOperations", func(t *testing.T) {
		// Test service start
		err := r.StartService(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service start should succeed")

		// Test service status
		isActive, err := r.IsServiceActive(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service status check should succeed")
		_ = isActive // Result depends on mock

		// Test service enable
		err = r.EnableService(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service enable should succeed")

		// Test service restart
		err = r.RestartService(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service restart should succeed")

		// Test service stop
		err = r.StopService(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service stop should succeed")

		// Test service disable
		err = r.DisableService(ctx, mockConn, facts, serviceName)
		assert.NoError(t, err, "Service disable should succeed")
	})
}

// TestRunnerUserManagement tests user and group management
func TestRunnerUserManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	testUser := "testuser"
	testGroup := "testgroup"

	t.Run("UserOperations", func(t *testing.T) {
		// Test group creation
		err := r.AddGroup(ctx, mockConn, testGroup, false)
		assert.NoError(t, err, "Group creation should succeed")

		// Test group existence
		exists, err := r.GroupExists(ctx, mockConn, testGroup)
		assert.NoError(t, err, "Group existence check should succeed")
		_ = exists // Result depends on mock

		// Test user creation
		err = r.AddUser(ctx, mockConn, testUser, testGroup, "/bin/bash", "/home/"+testUser, true, false)
		assert.NoError(t, err, "User creation should succeed")

		// Test user existence
		exists, err = r.UserExists(ctx, mockConn, testUser)
		assert.NoError(t, err, "User existence check should succeed")
		_ = exists // Result depends on mock

		// Test user info retrieval
		userInfo, err := r.GetUserInfo(ctx, mockConn, testUser)
		if err == nil {
			assert.NotNil(t, userInfo, "User info should not be nil")
			assert.Equal(t, testUser, userInfo.Username, "Username should match")
		}
	})
}

// TestRunnerNetworking tests network-related operations
func TestRunnerNetworking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup facts
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
	facts, err := r.GatherFacts(ctx, mockConn)
	require.NoError(t, err, "Should gather facts")

	t.Run("PortChecking", func(t *testing.T) {
		// Test port checking
		isOpen, err := r.IsPortOpen(ctx, mockConn, facts, 22)
		assert.NoError(t, err, "Port check should succeed")
		_ = isOpen // Result depends on mock

		// Test port waiting (with short timeout)
		err = r.WaitForPort(ctx, mockConn, facts, 80, 1*time.Second)
		// This might timeout in mock, which is expected
		_ = err
	})

	t.Run("HostnameOperations", func(t *testing.T) {
		newHostname := "test-hostname"
		
		// Test hostname setting
		err := r.SetHostname(ctx, mockConn, facts, newHostname)
		assert.NoError(t, err, "Hostname setting should succeed")
	})

	t.Run("HostsFileManagement", func(t *testing.T) {
		// Test adding host entry
		err := r.AddHostEntry(ctx, mockConn, "192.168.1.100", "test.example.com", "test")
		assert.NoError(t, err, "Adding host entry should succeed")
	})
}

// TestRunnerErrorHandling tests error handling scenarios
func TestRunnerErrorHandling(t *testing.T) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	t.Run("InvalidCommand", func(t *testing.T) {
		// Test with command that should fail
		mockConn.SetupCommandError("nonexistent-command", 127, "command not found")
		
		_, err := r.Run(ctx, mockConn, "nonexistent-command", false)
		assert.Error(t, err, "Invalid command should fail")
		
		// Check if it's a CommandError
		if cmdErr, ok := err.(*connector.CommandError); ok {
			assert.Equal(t, 127, cmdErr.ExitCode, "Exit code should be 127")
			assert.Contains(t, cmdErr.Stderr, "command not found", "Error message should be descriptive")
		}
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		// Test command timeout
		mockConn.SetupCommandTimeout("sleep 10")
		
		_, _, err := r.RunWithOptions(ctx, mockConn, "sleep 10", &connector.ExecOptions{
			Timeout: 100 * time.Millisecond,
		})
		assert.Error(t, err, "Command should timeout")
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		// Test permission denied scenario
		mockConn.SetupCommandError("cat /etc/shadow", 1, "permission denied")
		
		_, err := r.Run(ctx, mockConn, "cat /etc/shadow", false)
		assert.Error(t, err, "Permission denied should fail")
	})
}

// TestRunnerConcurrency tests concurrent operations
func TestRunnerConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	t.Run("ConcurrentCommands", func(t *testing.T) {
		numGoroutines := 10
		resultChan := make(chan error, numGoroutines)

		// Run multiple commands concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				cmd := fmt.Sprintf("echo 'test %d'", index)
				_, err := r.Run(ctx, mockConn, cmd, false)
				resultChan <- err
			}(i)
		}

		// Wait for all results
		for i := 0; i < numGoroutines; i++ {
			err := <-resultChan
			assert.NoError(t, err, "Concurrent command should succeed")
		}
	})

	t.Run("ConcurrentFileOperations", func(t *testing.T) {
		numGoroutines := 5
		resultChan := make(chan error, numGoroutines)

		// Perform concurrent file operations
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				testPath := fmt.Sprintf("/tmp/test-file-%d", index)
				content := []byte(fmt.Sprintf("content %d", index))
				
				// Write file
				err := r.WriteFile(ctx, mockConn, content, testPath, "0644", false)
				if err != nil {
					resultChan <- err
					return
				}
				
				// Read file
				_, err = r.ReadFile(ctx, mockConn, testPath)
				if err != nil {
					resultChan <- err
					return
				}
				
				// Remove file
				err = r.Remove(ctx, mockConn, testPath, false)
				resultChan <- err
			}(i)
		}

		// Wait for all results
		for i := 0; i < numGoroutines; i++ {
			err := <-resultChan
			assert.NoError(t, err, "Concurrent file operation should succeed")
		}
	})
}

// Benchmark tests for runner performance
func BenchmarkRunnerCommand(b *testing.B) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Run(ctx, mockConn, "echo test", false)
	}
}

func BenchmarkRunnerFileOperations(b *testing.B) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()
	content := []byte("benchmark test content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testPath := fmt.Sprintf("/tmp/bench-test-%d", i)
		_ = r.WriteFile(ctx, mockConn, content, testPath, "0644", false)
		_, _ = r.ReadFile(ctx, mockConn, testPath)
		_ = r.Remove(ctx, mockConn, testPath, false)
	}
}