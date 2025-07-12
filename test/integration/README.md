# Integration Tests

This directory contains integration tests for the Kubexm project. These tests verify the interaction between different components and simulate real-world usage scenarios.

## Test Structure

```
test/integration/
├── README.md                        # This file
├── mock_connector.go                # Mock connector for safe testing
├── pipeline_integration_test.go     # Pipeline execution tests
├── runner_integration_test.go       # Runner component tests
├── docker_integration_test.go       # Docker operations tests
├── kubernetes_integration_test.go   # Kubernetes operations tests
├── security_validation_test.go      # Security and validation tests
└── ssh_real_test.go                 # Real SSH connection tests
```

## Test Categories

### Pipeline Integration Tests (`pipeline_integration_test.go`)

Tests the complete pipeline execution flow:

- **TestPipelineExecutionIntegration**: Tests full pipeline planning and execution
- **TestPipelineConfigurationValidation**: Tests pipeline with various configurations
- **TestPipelineModuleDependencies**: Verifies correct module execution order
- **TestPipelineErrorHandling**: Tests error handling in pipeline execution
- **TestPipelineParallelExecution**: Tests parallel task execution
- **TestPipelineConfigFromFile**: Tests loading configuration from YAML files
- **TestPipelineCleanup**: Tests pipeline cleanup operations

### Runner Integration Tests (`runner_integration_test.go`)

Tests the runner package integration:

- **TestRunnerIntegration**: Basic runner operations (facts, files, commands)
- **TestRunnerPackageManagement**: Package installation/removal/queries
- **TestRunnerServiceManagement**: Service start/stop/enable/disable operations
- **TestRunnerUserManagement**: User and group management
- **TestRunnerNetworking**: Network-related operations (ports, hostname, hosts file)
- **TestRunnerErrorHandling**: Error handling scenarios
- **TestRunnerConcurrency**: Concurrent operations testing

### Docker Integration Tests (`docker_integration_test.go`)

Tests Docker operations through the runner interface:

- **TestDockerIntegration**: Complete Docker operations testing
  - **DockerImageOperations**: Image listing, pulling, and removal
  - **DockerContainerOperations**: Container creation, management, and lifecycle
  - **DockerNetworkOperations**: Network creation, listing, and management
  - **DockerVolumeOperations**: Volume creation, listing, and management
  - **DockerErrorHandling**: Error scenarios and recovery
- **TestDockerComposeIntegration**: Docker Compose operations testing
- **BenchmarkDockerOperations**: Performance benchmarks for Docker operations

### Kubernetes Integration Tests (`kubernetes_integration_test.go`)

Tests Kubernetes operations through the runner interface:

- **TestKubernetesIntegration**: Complete Kubernetes operations testing
  - **KubectlClusterInfo**: Cluster information and version checking
  - **KubectlResourceOperations**: Resource management (pods, deployments, services)
  - **KubectlApplyOperations**: Manifest application and deletion
  - **KubectlExecOperations**: Pod command execution
  - **KubectlScaleOperations**: Resource scaling
  - **KubectlRolloutOperations**: Deployment rollouts and history
  - **KubectlLogsOperations**: Log retrieval and monitoring
  - **KubectlPortForward**: Port forwarding operations
  - **KubectlErrorHandling**: Error scenarios and recovery
- **TestKubeadmIntegration**: Kubeadm cluster management testing
- **TestKubeletIntegration**: Kubelet service and configuration testing
- **BenchmarkKubernetesOperations**: Performance benchmarks for Kubernetes operations

### Security and Validation Tests (`security_validation_test.go`)

Tests security validations and system compliance:

- **TestSecurityValidation**: Security-related operations testing
  - **SSHKeyValidation**: SSH key generation and validation
  - **CertificateValidation**: TLS certificate operations and validation
  - **FilePermissionsAudit**: File permission and ownership auditing
  - **NetworkSecurityAudit**: Network security and firewall status
  - **ProcessAudit**: Running process monitoring and suspicious activity detection
- **TestConfigurationValidation**: Configuration validation scenarios
- **TestResourceValidation**: System resource validation and requirements checking
- **TestErrorRecovery**: Error recovery and rollback mechanisms testing
- **TestConcurrentOperations**: Concurrent operation safety testing
- **BenchmarkValidationOperations**: Performance benchmarks for validation operations

### Real SSH Connection Tests (`ssh_real_test.go`)

Tests real SSH connections using provided test credentials:

- **TestSSHRealConnection**: Real SSH connection testing
  - **密码认证连接测试**: Password authentication testing
  - **私钥认证连接测试**: Private key authentication testing  
  - **Sudo用户连接测试**: Sudo user connection testing
- **TestSSHRunnerOperations**: SSH runner operations testing
  - **基本命令执行**: Basic command execution
  - **系统信息收集**: System information gathering
  - **文件操作测试**: File operations testing
  - **目录操作测试**: Directory operations testing
  - **sudo权限测试**: Sudo permission testing
- **TestSSHPerformance**: SSH performance testing
- **TestSSHErrorHandling**: SSH error handling and recovery
- **BenchmarkSSHOperations**: Performance benchmarks for SSH operations

**Test Configuration:**
- Host: 192.168.31.34:2222
- Root user: root / rootpassword
- Sudo user: testuser / testpassword  
- Private key: /home/mensyli1/workspace/kubexm/test_id_rsa

## Mock Infrastructure

### MockConnector

The `MockConnector` provides a safe, controllable environment for testing:

- **Command Simulation**: Configurable responses for any command
- **File System Simulation**: Mock file and directory operations
- **Error Simulation**: Controllable error conditions
- **Performance Testing**: Configurable delays and timeouts
- **Execution Tracking**: Records all executed commands for verification

#### Usage Example

```go
// Create mock connector
mockConn := NewMockConnector()

// Configure OS information
mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

// Configure command responses
mockConn.SetupCommandResponse("echo test", "test", "", 0)
mockConn.SetupCommandError("invalid-cmd", 127, "command not found")

// Use with runner
r := runner.NewRunner()
output, err := r.Run(ctx, mockConn, "echo test", false)
```

## Running Integration Tests

### Run All Integration Tests

```bash
go test ./test/integration/... -v
```

### Run Specific Test Categories

```bash
# Pipeline tests only
go test ./test/integration/... -run TestPipeline -v

# Runner tests only
go test ./test/integration/... -run TestRunner -v

# Docker tests only
go test ./test/integration/... -run TestDocker -v

# Kubernetes tests only
go test ./test/integration/... -run TestKubernetes -v

# Security and validation tests only
go test ./test/integration/... -run TestSecurity -v
go test ./test/integration/... -run TestConfiguration -v
go test ./test/integration/... -run TestResource -v

# Real SSH connection tests (requires test host)
go test ./test/integration/... -run TestSSH -v

# Error handling tests only
go test ./test/integration/... -run ErrorHandling -v

# Concurrency tests only
go test ./test/integration/... -run Concurrency -v
```

### Run with Different Test Modes

```bash
# Skip long-running tests
go test ./test/integration/... -short

# Skip real SSH tests (use environment variable)
KUBEXM_SKIP_REAL_SSH_TESTS=1 go test ./test/integration/... -v

# Run with race detection
go test ./test/integration/... -race

# Run with coverage
go test ./test/integration/... -cover -coverprofile=coverage.out
```

### Performance Benchmarks

```bash
# Run all benchmarks
go test ./test/integration/... -bench=. -benchmem

# Run specific benchmarks
go test ./test/integration/... -bench=BenchmarkPipeline
go test ./test/integration/... -bench=BenchmarkRunner
go test ./test/integration/... -bench=BenchmarkDocker
go test ./test/integration/... -bench=BenchmarkKubernetes
go test ./test/integration/... -bench=BenchmarkValidation

# Run SSH benchmarks (requires test host)
KUBEXM_SKIP_REAL_SSH_TESTS="" go test ./test/integration/... -bench=BenchmarkSSH
```

## Test Configuration

### Environment Variables

- `KUBEXM_TEST_TIMEOUT`: Override default test timeout
- `KUBEXM_TEST_VERBOSE`: Enable verbose test output
- `KUBEXM_TEST_REAL_INFRA`: Enable tests against real infrastructure (use with caution)
- `KUBEXM_SKIP_REAL_SSH_TESTS`: Skip real SSH connection tests (set to "1" to skip)

### Test Tags

Use build tags to control test execution:

```bash
# Run only safe mock tests (default)
go test ./test/integration/... -tags=mock

# Run tests that may modify system (requires special setup)
go test ./test/integration/... -tags=system
```

## Safety Features

### Mock-Only Execution (Default)

By default, most integration tests use mock connectors and do not:
- Execute real commands on the host system
- Modify actual files or directories
- Install or remove real packages
- Start or stop real services
- Create or modify real network configurations

### Real SSH Connection Tests

The `ssh_real_test.go` file contains tests that make actual SSH connections to a test host:
- **Warning**: These tests will execute real commands on the target system
- Use environment variable `KUBEXM_SKIP_REAL_SSH_TESTS=1` to skip these tests
- Requires proper test host setup with the configured credentials
- Test host configuration:
  - Host: 192.168.31.34:2222
  - Root user: root/rootpassword
  - Sudo user: testuser/testpassword
  - Private key: /home/mensyli1/workspace/kubexm/test_id_rsa

### Error Simulation

Tests include comprehensive error simulation:
- Command execution failures
- Network timeouts
- Permission denied scenarios
- Resource exhaustion conditions
- Malformed input handling

### Resource Management

Tests are designed to:
- Complete within reasonable time limits
- Clean up all mock resources (and test files on real systems)
- Avoid resource leaks
- Handle concurrent execution safely

## Adding New Integration Tests

### Guidelines

1. **Use Mock Connectors**: Always use `MockConnector` for safety
2. **Test Real Scenarios**: Simulate actual deployment workflows
3. **Include Error Cases**: Test both success and failure paths
4. **Document Behavior**: Clearly describe what each test validates
5. **Performance Awareness**: Include benchmarks for critical paths

### Test Template

```go
func TestNewFeatureIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup
    mockConn := NewMockConnector()
    mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
    
    // Test implementation
    t.Run("success_case", func(t *testing.T) {
        // Test successful operation
    })
    
    t.Run("error_case", func(t *testing.T) {
        // Test error handling
    })
    
    t.Run("concurrent_case", func(t *testing.T) {
        // Test concurrent operations
    })
}
```

## Troubleshooting

### Common Issues

1. **Test Timeouts**: Increase timeout or use `-short` flag
2. **Mock Configuration**: Ensure mock responses match expected commands
3. **Race Conditions**: Use proper synchronization in concurrent tests
4. **Resource Cleanup**: Verify all test resources are properly cleaned up

### Debugging

```bash
# Enable verbose output
go test ./test/integration/... -v -run TestSpecific

# Debug with delve
dlv test ./test/integration/...

# Check test coverage
go test ./test/integration/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Contributing

When adding integration tests:

1. Follow existing test patterns
2. Use descriptive test names
3. Include both positive and negative test cases
4. Add appropriate benchmarks for performance-critical code
5. Update this README if adding new test categories
6. Ensure tests are safe and use mocks appropriately

## Security Considerations

- Tests use mock connectors to prevent accidental system modification
- No real credentials or sensitive data should be used in tests
- Network operations are simulated to prevent external dependencies
- File operations are contained within mock file systems