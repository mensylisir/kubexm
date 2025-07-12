package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// TestSecurityValidation tests security-related validations and operations
func TestSecurityValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("SSHKeyValidation", func(t *testing.T) {
		// Test SSH key generation and validation
		keyPath := "/tmp/test-ssh-key"
		
		// Generate SSH key pair
		err := r.GenerateSSHKey(ctx, mockConn, keyPath, "rsa", 2048, "test@example.com")
		assert.NoError(t, err, "SSH key generation should succeed")

		// Validate SSH key
		isValid, err := r.ValidateSSHKey(ctx, mockConn, keyPath)
		assert.NoError(t, err, "SSH key validation should succeed")
		assert.True(t, isValid, "Generated SSH key should be valid")

		// Test SSH key permissions
		permissions, err := r.GetFilePermissions(ctx, mockConn, keyPath)
		assert.NoError(t, err, "Getting file permissions should succeed")
		assert.Equal(t, "600", permissions, "SSH private key should have 600 permissions")
	})

	t.Run("CertificateValidation", func(t *testing.T) {
		// Test TLS certificate operations
		certPath := "/tmp/test-cert.pem"
		keyPath := "/tmp/test-cert-key.pem"

		// Generate self-signed certificate
		certConfig := runner.CertificateConfig{
			CommonName:   "test.example.com",
			Organization: "Test Org",
			Country:      "US",
			ValidDays:    365,
			KeySize:      2048,
		}

		err := r.GenerateCertificate(ctx, mockConn, certPath, keyPath, certConfig)
		assert.NoError(t, err, "Certificate generation should succeed")

		// Validate certificate
		info, err := r.ValidateCertificate(ctx, mockConn, certPath)
		assert.NoError(t, err, "Certificate validation should succeed")
		assert.Equal(t, "test.example.com", info.CommonName, "Common name should match")
		assert.False(t, info.IsExpired, "Certificate should not be expired")
	})

	t.Run("FilePermissionsAudit", func(t *testing.T) {
		// Test file permission auditing
		testFiles := []string{
			"/etc/passwd",
			"/etc/shadow", 
			"/etc/ssh/sshd_config",
			"/var/log/auth.log",
		}

		for _, file := range testFiles {
			mockConn.SetFileContent(file, []byte("mock content"))
			
			permissions, err := r.GetFilePermissions(ctx, mockConn, file)
			assert.NoError(t, err, fmt.Sprintf("Getting permissions for %s should succeed", file))
			assert.NotEmpty(t, permissions, "Permissions should not be empty")

			owner, err := r.GetFileOwner(ctx, mockConn, file)
			assert.NoError(t, err, fmt.Sprintf("Getting owner for %s should succeed", file))
			assert.NotEmpty(t, owner, "Owner should not be empty")
		}
	})

	t.Run("NetworkSecurityAudit", func(t *testing.T) {
		// Test network security auditing
		openPorts, err := r.GetOpenPorts(ctx, mockConn)
		assert.NoError(t, err, "Getting open ports should succeed")
		assert.NotNil(t, openPorts, "Open ports list should not be nil")

		// Test firewall status
		firewallStatus, err := r.GetFirewallStatus(ctx, mockConn)
		assert.NoError(t, err, "Getting firewall status should succeed")
		_ = firewallStatus // Status depends on mock

		// Test network interfaces
		interfaces, err := r.GetNetworkInterfaces(ctx, mockConn)
		assert.NoError(t, err, "Getting network interfaces should succeed")
		assert.NotNil(t, interfaces, "Network interfaces should not be nil")
	})

	t.Run("ProcessAudit", func(t *testing.T) {
		// Test process auditing
		processes, err := r.GetRunningProcesses(ctx, mockConn)
		assert.NoError(t, err, "Getting running processes should succeed")
		assert.NotNil(t, processes, "Process list should not be nil")

		// Test for suspicious processes (in real environment)
		suspiciousProcesses := []string{"cryptominer", "backdoor", "malware"}
		for _, proc := range suspiciousProcesses {
			found, err := r.FindProcess(ctx, mockConn, proc)
			assert.NoError(t, err, "Process search should succeed")
			assert.False(t, found, fmt.Sprintf("Suspicious process %s should not be found", proc))
		}
	})
}

// TestConfigurationValidation tests configuration validation
func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid minimal config",
			configData: `
apiVersion: kubexm.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  roleGroups:
    master:
      hosts:
        - master1.example.com
    worker:
      hosts:
        - worker1.example.com
    etcd:
      hosts:
        - etcd1.example.com
  kubernetes:
    version: v1.28.0
`,
			expectError: false,
		},
		{
			name: "invalid kubernetes version",
			configData: `
apiVersion: kubexm.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  roleGroups:
    master:
      hosts:
        - master1.example.com
  kubernetes:
    version: v1.0.0
`,
			expectError: true,
			errorMsg:    "version",
		},
		{
			name: "missing required fields",
			configData: `
apiVersion: kubexm.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  kubernetes:
    version: v1.28.0
`,
			expectError: true,
			errorMsg:    "required",
		},
		{
			name: "invalid network CIDR",
			configData: `
apiVersion: kubexm.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  roleGroups:
    master:
      hosts:
        - master1.example.com
  kubernetes:
    version: v1.28.0
  networking:
    podCIDR: "invalid-cidr"
`,
			expectError: true,
			errorMsg:    "CIDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse configuration
			clusterConfig, err := config.ParseFromString(tt.configData)
			
			if tt.expectError {
				if err == nil {
					// Try validation
					err = v1alpha1.Validate_Cluster(clusterConfig)
				}
				assert.Error(t, err, "Should fail validation")
				if tt.errorMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorMsg))
				}
			} else {
				assert.NoError(t, err, "Should parse successfully")
				if clusterConfig != nil {
					err = v1alpha1.Validate_Cluster(clusterConfig)
					assert.NoError(t, err, "Should pass validation")
				}
			}
		})
	}
}

// TestResourceValidation tests resource allocation and limits validation
func TestResourceValidation(t *testing.T) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("SystemResourceCheck", func(t *testing.T) {
		// Test system resource gathering
		facts, err := r.GatherFacts(ctx, mockConn)
		require.NoError(t, err, "Should gather facts")

		// Validate minimum requirements
		assert.Greater(t, facts.TotalCPU, 0, "Should have at least 1 CPU")
		assert.Greater(t, facts.TotalMemory, uint64(1024*1024*1024), "Should have at least 1GB memory")
		assert.Greater(t, facts.TotalDisk, uint64(10*1024*1024*1024), "Should have at least 10GB disk")
	})

	t.Run("NetworkConnectivityCheck", func(t *testing.T) {
		// Test network connectivity
		testHosts := []string{
			"google.com",
			"github.com",
			"k8s.io",
		}

		for _, host := range testHosts {
			// Mock successful ping
			mockConn.SetupCommandResponse(fmt.Sprintf("ping -c 1 %s", host), "PING successful", "", 0)
			
			reachable, err := r.CheckConnectivity(ctx, mockConn, host, 80, 5*time.Second)
			assert.NoError(t, err, fmt.Sprintf("Connectivity check to %s should succeed", host))
			_ = reachable // Result depends on mock
		}
	})

	t.Run("DiskSpaceValidation", func(t *testing.T) {
		// Test disk space requirements
		requiredPaths := []string{
			"/var/lib/docker",
			"/var/lib/kubelet", 
			"/etc/kubernetes",
			"/opt/cni",
		}

		for _, path := range requiredPaths {
			// Create directory
			err := r.Mkdirp(ctx, mockConn, path, "0755", true)
			assert.NoError(t, err, fmt.Sprintf("Creating directory %s should succeed", path))

			// Check available space
			space, err := r.GetAvailableDiskSpace(ctx, mockConn, path)
			assert.NoError(t, err, fmt.Sprintf("Getting disk space for %s should succeed", path))
			assert.Greater(t, space, uint64(1024*1024*1024), "Should have at least 1GB available")
		}
	})
}

// TestErrorRecovery tests error recovery and rollback scenarios
func TestErrorRecovery(t *testing.T) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("CommandRetry", func(t *testing.T) {
		// Setup command that fails initially then succeeds
		attempts := 0
		mockConn.SetupCommandResponse("unstable-command", "", "", 0)
		
		// Mock a command that fails first time
		firstCall := true
		originalExec := mockConn.Exec
		mockConn.Exec = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if cmd == "unstable-command" && firstCall {
				firstCall = false
				attempts++
				return nil, []byte("temporary failure"), &connector.CommandError{ExitCode: 1}
			}
			attempts++
			return originalExec(ctx, cmd, options)
		}

		// Test retry mechanism
		_, err := r.RunWithRetry(ctx, mockConn, "unstable-command", 3, 100*time.Millisecond)
		assert.NoError(t, err, "Command with retry should eventually succeed")
		assert.Equal(t, 2, attempts, "Should have retried once")
	})

	t.Run("PartialFailureRecovery", func(t *testing.T) {
		// Test recovery from partial failures
		services := []string{"service1", "service2", "service3"}
		
		// Make service2 fail
		facts, err := r.GatherFacts(ctx, mockConn)
		require.NoError(t, err)

		mockConn.SetupCommandError("systemctl start service2", 1, "Failed to start service2")

		failedServices := []string{}
		for _, service := range services {
			err := r.StartService(ctx, mockConn, facts, service)
			if err != nil {
				failedServices = append(failedServices, service)
			}
		}

		assert.Contains(t, failedServices, "service2", "service2 should have failed")
		assert.Len(t, failedServices, 1, "Only service2 should have failed")

		// Test recovery - restart failed services
		for _, service := range failedServices {
			// Setup success for recovery
			mockConn.SetupCommandResponse(fmt.Sprintf("systemctl start %s", service), "Started successfully", "", 0)
			
			err := r.StartService(ctx, mockConn, facts, service)
			assert.NoError(t, err, fmt.Sprintf("Recovery of %s should succeed", service))
		}
	})
}

// TestConcurrentOperations tests concurrent operation safety
func TestConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("ConcurrentFileOperations", func(t *testing.T) {
		numGoroutines := 10
		resultChan := make(chan error, numGoroutines)

		// Test concurrent file operations
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				filePath := fmt.Sprintf("/tmp/concurrent-test-%d", index)
				content := []byte(fmt.Sprintf("content-%d", index))
				
				// Write, read, then remove
				err := r.WriteFile(ctx, mockConn, content, filePath, "0644", false)
				if err != nil {
					resultChan <- err
					return
				}
				
				readContent, err := r.ReadFile(ctx, mockConn, filePath)
				if err != nil {
					resultChan <- err
					return
				}
				
				if string(readContent) != string(content) {
					resultChan <- fmt.Errorf("content mismatch for file %s", filePath)
					return
				}
				
				err = r.Remove(ctx, mockConn, filePath, false)
				resultChan <- err
			}(i)
		}

		// Wait for all operations to complete
		for i := 0; i < numGoroutines; i++ {
			err := <-resultChan
			assert.NoError(t, err, "Concurrent file operation should succeed")
		}
	})

	t.Run("ConcurrentServiceOperations", func(t *testing.T) {
		facts, err := r.GatherFacts(ctx, mockConn)
		require.NoError(t, err)

		services := []string{"service1", "service2", "service3", "service4", "service5"}
		resultChan := make(chan error, len(services))

		// Test concurrent service operations
		for _, service := range services {
			go func(svc string) {
				// Start, check status, then stop
				err := r.StartService(ctx, mockConn, facts, svc)
				if err != nil {
					resultChan <- err
					return
				}
				
				_, err = r.IsServiceActive(ctx, mockConn, facts, svc)
				if err != nil {
					resultChan <- err
					return
				}
				
				err = r.StopService(ctx, mockConn, facts, svc)
				resultChan <- err
			}(service)
		}

		// Wait for all operations to complete
		for range services {
			err := <-resultChan
			assert.NoError(t, err, "Concurrent service operation should succeed")
		}
	})
}

// Benchmark tests for validation operations
func BenchmarkValidationOperations(b *testing.B) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	b.Run("ConfigValidation", func(b *testing.B) {
		configData := `
apiVersion: kubexm.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
spec:
  roleGroups:
    master:
      hosts: ["master1.example.com"]
  kubernetes:
    version: v1.28.0`

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			config, _ := config.ParseFromString(configData)
			if config != nil {
				_ = v1alpha1.Validate_Cluster(config)
			}
		}
	})

	b.Run("FactsGathering", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.GatherFacts(ctx, mockConn)
		}
	})
}