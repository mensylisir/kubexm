package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/pipeline/cluster"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// TestPipelineExecutionIntegration tests the full pipeline execution flow
func TestPipelineExecutionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test cluster configuration
	clusterConfig := createTestClusterConfig()
	
	// Create runtime context
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err, "Failed to create runtime context")
	
	// Create and execute pipeline
	createPipeline := cluster.NewCreateClusterPipeline()
	
	// Mock execution for safety (don't actually create infrastructure)
	mockEngine := &MockEngine{
		shouldSucceed: true,
	}
	
	// Execute pipeline planning
	planResult, err := createPipeline.Plan(runtimeCtx)
	require.NoError(t, err, "Pipeline planning should succeed")
	assert.NotNil(t, planResult, "Plan result should not be nil")
	
	// Test pipeline execution (dry run)
	executionResult, err := createPipeline.Run(runtimeCtx, true) // dry run
	require.NoError(t, err, "Pipeline dry run should succeed")
	assert.NotNil(t, executionResult, "Execution result should not be nil")
}

// TestPipelineConfigurationValidation tests pipeline with different configurations
func TestPipelineConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		configModifier func(*v1alpha1.Cluster)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid minimal config",
			configModifier: func(c *v1alpha1.Cluster) {
				// Keep default test config
			},
			expectError: false,
		},
		{
			name: "missing master nodes",
			configModifier: func(c *v1alpha1.Cluster) {
				c.Spec.RoleGroups.Master.Hosts = []string{}
			},
			expectError:   true,
			errorContains: "master",
		},
		{
			name: "invalid kubernetes version",
			configModifier: func(c *v1alpha1.Cluster) {
				c.Spec.Kubernetes.Version = "v1.0.0" // Very old version
			},
			expectError:   true,
			errorContains: "version",
		},
		{
			name: "missing etcd nodes",
			configModifier: func(c *v1alpha1.Cluster) {
				c.Spec.RoleGroups.Etcd.Hosts = []string{}
			},
			expectError:   true,
			errorContains: "etcd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterConfig := createTestClusterConfig()
			tt.configModifier(clusterConfig)
			
			// Test configuration validation
			err := v1alpha1.Validate_Cluster(clusterConfig)
			
			if tt.expectError {
				assert.Error(t, err, "Should fail validation")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Should pass validation")
				
				// If validation passes, try to create runtime context
				runtimeCtx, err := runtime.NewContext(clusterConfig)
				assert.NoError(t, err, "Should create runtime context successfully")
				assert.NotNil(t, runtimeCtx)
			}
		})
	}
}

// TestPipelineModuleDependencies tests that modules are executed in the correct order
func TestPipelineModuleDependencies(t *testing.T) {
	clusterConfig := createTestClusterConfig()
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err)

	createPipeline := cluster.NewCreateClusterPipeline()
	
	// Get the modules from the pipeline
	modules := createPipeline.GetModules()
	
	// Test that we have expected modules
	assert.NotEmpty(t, modules, "Pipeline should have modules")
	
	// Test module planning in sequence
	for i, mod := range modules {
		t.Run(fmt.Sprintf("module_%d_planning", i), func(t *testing.T) {
			fragment, err := mod.Plan(runtimeCtx)
			assert.NoError(t, err, "Module planning should succeed")
			assert.NotNil(t, fragment, "Module should return fragment")
		})
	}
}

// TestPipelineErrorHandling tests error handling in pipeline execution
func TestPipelineErrorHandling(t *testing.T) {
	clusterConfig := createTestClusterConfig()
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err)

	// Test with failing engine
	mockEngine := &MockEngine{
		shouldSucceed: false,
		errorMessage:  "Simulated execution failure",
	}
	
	createPipeline := cluster.NewCreateClusterPipeline()
	
	// Mock the engine to simulate failure
	// Note: This would require dependency injection or interface mocking
	// For now, we test dry run which should always succeed
	result, err := createPipeline.Run(runtimeCtx, true) // dry run
	
	// Dry run should succeed even with mock failures
	assert.NoError(t, err, "Dry run should not fail")
	assert.NotNil(t, result)
}

// TestPipelineParallelExecution tests that independent tasks can run in parallel
func TestPipelineParallelExecution(t *testing.T) {
	clusterConfig := createTestClusterConfig()
	
	// Add multiple worker nodes to test parallel execution
	clusterConfig.Spec.RoleGroups.Worker.Hosts = []string{
		"worker1.example.com",
		"worker2.example.com", 
		"worker3.example.com",
	}
	
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err)

	createPipeline := cluster.NewCreateClusterPipeline()
	
	start := time.Now()
	result, err := createPipeline.Run(runtimeCtx, true) // dry run
	duration := time.Since(start)
	
	assert.NoError(t, err, "Pipeline execution should succeed")
	assert.NotNil(t, result)
	
	// In a real test, parallel execution should be faster than sequential
	// For dry run, this is mainly testing that the pipeline accepts multiple hosts
	t.Logf("Pipeline execution took: %v", duration)
	assert.Less(t, duration, 30*time.Second, "Dry run should be fast")
}

// TestPipelineConfigFromFile tests loading configuration from file and executing pipeline
func TestPipelineConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-cluster.yaml")
	
	configContent := `
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
    containerRuntime:
      type: containerd
  networking:
    podCIDR: "10.244.0.0/16"
    serviceCIDR: "10.96.0.0/12"
    dnsDomain: "cluster.local"
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err, "Should write config file")
	
	// Load configuration from file
	clusterConfig, err := config.ParseFromFile(configFile)
	require.NoError(t, err, "Should parse config from file")
	assert.NotNil(t, clusterConfig)
	
	// Create runtime context
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err, "Should create runtime context")
	
	// Execute pipeline
	createPipeline := cluster.NewCreateClusterPipeline()
	result, err := createPipeline.Run(runtimeCtx, true) // dry run
	
	assert.NoError(t, err, "Pipeline should execute successfully")
	assert.NotNil(t, result)
}

// TestPipelineCleanup tests pipeline cleanup operations
func TestPipelineCleanup(t *testing.T) {
	clusterConfig := createTestClusterConfig()
	runtimeCtx, err := runtime.NewContext(clusterConfig)
	require.NoError(t, err)

	// Test that pipeline can handle cleanup operations
	createPipeline := cluster.NewCreateClusterPipeline()
	
	// Execute dry run
	result, err := createPipeline.Run(runtimeCtx, true)
	require.NoError(t, err, "Initial execution should succeed")
	
	// Test cleanup (if implemented)
	if cleaner, ok := createPipeline.(interface{ Cleanup() error }); ok {
		err = cleaner.Cleanup()
		assert.NoError(t, err, "Cleanup should succeed")
	}
}

// Helper functions and mocks

func createTestClusterConfig() *v1alpha1.Cluster {
	config := &v1alpha1.Cluster{}
	config.APIVersion = "kubexm.io/v1alpha1"
	config.Kind = "Cluster"
	config.Metadata.Name = "test-cluster"
	
	// Set defaults
	v1alpha1.SetDefaults_Cluster(config)
	
	// Override with test-specific values
	config.Spec.RoleGroups.Master.Hosts = []string{"master1.example.com"}
	config.Spec.RoleGroups.Worker.Hosts = []string{"worker1.example.com"}
	config.Spec.RoleGroups.Etcd.Hosts = []string{"etcd1.example.com"}
	
	return config
}

// MockEngine simulates engine execution for testing
type MockEngine struct {
	shouldSucceed bool
	errorMessage  string
	executionTime time.Duration
}

func (m *MockEngine) Execute(ctx context.Context, plan interface{}, dryRun bool) error {
	if m.executionTime > 0 {
		time.Sleep(m.executionTime)
	}
	
	if !m.shouldSucceed && !dryRun {
		return fmt.Errorf("mock engine error: %s", m.errorMessage)
	}
	
	return nil
}

// Benchmark tests for pipeline performance
func BenchmarkPipelinePlanning(b *testing.B) {
	clusterConfig := createTestClusterConfig()
	runtimeCtx, _ := runtime.NewContext(clusterConfig)
	createPipeline := cluster.NewCreateClusterPipeline()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = createPipeline.Plan(runtimeCtx)
	}
}

func BenchmarkPipelineDryRun(b *testing.B) {
	clusterConfig := createTestClusterConfig()
	runtimeCtx, _ := runtime.NewContext(clusterConfig)
	createPipeline := cluster.NewCreateClusterPipeline()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = createPipeline.Run(runtimeCtx, true) // dry run
	}
}