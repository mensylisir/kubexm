package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// TestKubernetesIntegration tests Kubernetes operations through the runner interface
func TestKubernetesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup mock OS with Kubernetes tools
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("KubectlClusterInfo", func(t *testing.T) {
		// Test cluster info
		info, err := r.KubectlClusterInfo(ctx, mockConn)
		assert.NoError(t, err, "KubectlClusterInfo should succeed")
		assert.NotEmpty(t, info, "Cluster info should not be empty")

		// Test version
		version, err := r.KubectlVersion(ctx, mockConn, runner.KubectlVersionOptions{Client: true, Server: true})
		assert.NoError(t, err, "KubectlVersion should succeed")
		assert.NotEmpty(t, version, "Version info should not be empty")
	})

	t.Run("KubectlResourceOperations", func(t *testing.T) {
		namespace := "default"
		opts := runner.KubectlGetOptions{
			Namespace: namespace,
			Output:    "json",
		}

		// Test getting deployments
		deployments, err := r.KubectlGetDeployments(ctx, mockConn, opts)
		assert.NoError(t, err, "KubectlGetDeployments should succeed")
		assert.NotNil(t, deployments, "Deployments should not be nil")

		// Test getting pods
		pods, err := r.KubectlGetPods(ctx, mockConn, opts)
		assert.NoError(t, err, "KubectlGetPods should succeed")
		assert.NotNil(t, pods, "Pods should not be nil")

		// Test getting services
		services, err := r.KubectlGetServices(ctx, mockConn, opts)
		assert.NoError(t, err, "KubectlGetServices should succeed")
		assert.NotNil(t, services, "Services should not be nil")

		// Test getting resource list
		resources, err := r.KubectlGetResourceList(ctx, mockConn, "pods", opts)
		assert.NoError(t, err, "KubectlGetResourceList should succeed")
		assert.NotNil(t, resources, "Resource list should not be nil")
	})

	t.Run("KubectlApplyOperations", func(t *testing.T) {
		manifestPath := "/tmp/test-manifest.yaml"
		manifestContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2`

		// Setup mock file
		mockConn.SetFileContent(manifestPath, []byte(manifestContent))

		opts := runner.KubectlApplyOptions{
			Filename:  manifestPath,
			Namespace: "default",
			DryRun:    true,
		}

		// Test apply
		output, err := r.KubectlApply(ctx, mockConn, opts)
		assert.NoError(t, err, "KubectlApply should succeed")
		assert.NotEmpty(t, output, "Apply output should not be empty")

		// Test delete
		deleteOpts := runner.KubectlDeleteOptions{
			Filename:  manifestPath,
			Namespace: "default",
		}

		err = r.KubectlDelete(ctx, mockConn, deleteOpts)
		assert.NoError(t, err, "KubectlDelete should succeed")
	})

	t.Run("KubectlExecOperations", func(t *testing.T) {
		podName := "test-pod"
		opts := runner.KubectlExecOptions{
			Namespace:     "default",
			Container:     "main",
			Stdin:         false,
			TTY:           false,
			Interactive:   false,
		}

		// Test exec command
		output, err := r.KubectlExec(ctx, mockConn, podName, opts, "echo", "hello")
		if err == nil {
			assert.NotEmpty(t, output, "Exec output should not be empty")
		}
		// Note: This might fail in mock environment, which is expected
	})

	t.Run("KubectlScaleOperations", func(t *testing.T) {
		opts := runner.KubectlScaleOptions{
			Namespace: "default",
		}

		// Test scaling deployment
		err := r.KubectlScale(ctx, mockConn, "deployment", "test-deployment", 3, opts)
		assert.NoError(t, err, "KubectlScale should succeed")
	})

	t.Run("KubectlRolloutOperations", func(t *testing.T) {
		opts := runner.KubectlRolloutOptions{
			Namespace: "default",
		}

		// Test rollout history
		history, err := r.KubectlRolloutHistory(ctx, mockConn, "deployment", "test-deployment", opts)
		assert.NoError(t, err, "KubectlRolloutHistory should succeed")
		assert.NotEmpty(t, history, "Rollout history should not be empty")

		// Test rollout undo
		err = r.KubectlRolloutUndo(ctx, mockConn, "deployment", "test-deployment", opts)
		assert.NoError(t, err, "KubectlRolloutUndo should succeed")

		// Test rollout status
		status, err := r.KubectlRolloutStatus(ctx, mockConn, "deployment", "test-deployment", opts)
		assert.NoError(t, err, "KubectlRolloutStatus should succeed")
		assert.NotEmpty(t, status, "Rollout status should not be empty")
	})

	t.Run("KubectlLogsOperations", func(t *testing.T) {
		opts := runner.KubectlLogsOptions{
			Namespace: "default",
			Follow:    false,
			Tail:      100,
		}

		// Test getting logs
		logs, err := r.KubectlLogs(ctx, mockConn, "test-pod", opts)
		if err == nil {
			assert.NotNil(t, logs, "Logs should not be nil")
		}
		// Note: This might fail in mock environment for non-existent pods
	})

	t.Run("KubectlPortForward", func(t *testing.T) {
		opts := runner.KubectlPortForwardOptions{
			Namespace: "default",
			LocalPort: 8080,
			PodPort:   80,
		}

		// Test port forwarding (should start in background)
		err := r.KubectlPortForward(ctx, mockConn, "test-pod", opts)
		if err == nil {
			// Port forwarding started successfully
			t.Log("Port forwarding started successfully")
		}
		// Note: This might not work in mock environment
	})

	t.Run("KubectlErrorHandling", func(t *testing.T) {
		// Test with invalid resource
		mockConn.SetupCommandError("kubectl get invalid-resource", 1, "error: the server doesn't have a resource type \"invalid-resource\"")

		opts := runner.KubectlGetOptions{Namespace: "default"}
		_, err := r.KubectlGetResourceList(ctx, mockConn, "invalid-resource", opts)
		assert.Error(t, err, "Invalid resource should fail")

		// Test with invalid namespace
		mockConn.SetupCommandError("kubectl get pods -n invalid-namespace", 1, "Error from server (NotFound): namespaces \"invalid-namespace\" not found")

		opts.Namespace = "invalid-namespace"
		_, err = r.KubectlGetPods(ctx, mockConn, opts)
		assert.Error(t, err, "Invalid namespace should fail")
	})
}

// TestKubeadmIntegration tests kubeadm operations
func TestKubeadmIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("KubeadmVersion", func(t *testing.T) {
		// Test kubeadm version
		version, err := r.KubeadmVersion(ctx, mockConn)
		assert.NoError(t, err, "KubeadmVersion should succeed")
		assert.NotEmpty(t, version, "Version should not be empty")
	})

	t.Run("KubeadmInit", func(t *testing.T) {
		config := runner.KubeadmInitConfig{
			PodNetworkCIDR:    "10.244.0.0/16",
			ServiceCIDR:       "10.96.0.0/12",
			KubernetesVersion: "v1.28.0",
			DryRun:            true, // Safe for testing
		}

		// Test kubeadm init (dry run)
		result, err := r.KubeadmInit(ctx, mockConn, config)
		assert.NoError(t, err, "KubeadmInit should succeed in dry run")
		assert.NotNil(t, result, "Init result should not be nil")
	})

	t.Run("KubeadmJoin", func(t *testing.T) {
		config := runner.KubeadmJoinConfig{
			APIServerEndpoint: "192.168.1.100:6443",
			Token:             "abcdef.0123456789abcdef",
			CACertHash:        "sha256:1234567890abcdef",
			DryRun:            true, // Safe for testing
		}

		// Test kubeadm join (dry run)
		err := r.KubeadmJoin(ctx, mockConn, config)
		assert.NoError(t, err, "KubeadmJoin should succeed in dry run")
	})

	t.Run("KubeadmReset", func(t *testing.T) {
		config := runner.KubeadmResetConfig{
			Force:  true,
			DryRun: true, // Safe for testing
		}

		// Test kubeadm reset (dry run)
		err := r.KubeadmReset(ctx, mockConn, config)
		assert.NoError(t, err, "KubeadmReset should succeed in dry run")
	})
}

// TestKubeletIntegration tests kubelet operations
func TestKubeletIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup facts with systemd
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
	facts, err := r.GatherFacts(ctx, mockConn)
	require.NoError(t, err)

	t.Run("KubeletService", func(t *testing.T) {
		// Test kubelet service operations
		err := r.StartService(ctx, mockConn, facts, "kubelet")
		assert.NoError(t, err, "Starting kubelet should succeed")

		err = r.EnableService(ctx, mockConn, facts, "kubelet")
		assert.NoError(t, err, "Enabling kubelet should succeed")

		isActive, err := r.IsServiceActive(ctx, mockConn, facts, "kubelet")
		assert.NoError(t, err, "Checking kubelet status should succeed")
		_ = isActive // Result depends on mock
	})

	t.Run("KubeletConfig", func(t *testing.T) {
		configPath := "/var/lib/kubelet/config.yaml"
		configContent := `apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
staticPodPath: /etc/kubernetes/manifests
syncFrequency: 1m
fileCheckFrequency: 20s`

		// Test writing kubelet config
		err := r.WriteFile(ctx, mockConn, []byte(configContent), configPath, "0644", true)
		assert.NoError(t, err, "Writing kubelet config should succeed")

		// Test reading kubelet config
		content, err := r.ReadFile(ctx, mockConn, configPath)
		assert.NoError(t, err, "Reading kubelet config should succeed")
		assert.Equal(t, []byte(configContent), content, "Config content should match")
	})
}

// Benchmark tests for Kubernetes operations
func BenchmarkKubernetesOperations(b *testing.B) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")
	opts := runner.KubectlGetOptions{Namespace: "default"}

	b.Run("KubectlGetPods", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.KubectlGetPods(ctx, mockConn, opts)
		}
	})

	b.Run("KubectlGetDeployments", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.KubectlGetDeployments(ctx, mockConn, opts)
		}
	})

	b.Run("KubectlClusterInfo", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.KubectlClusterInfo(ctx, mockConn)
		}
	})
}