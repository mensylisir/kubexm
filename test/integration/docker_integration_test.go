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

// TestDockerIntegration tests Docker operations through the runner interface
func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	// Setup mock OS with Docker support
	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("DockerImageOperations", func(t *testing.T) {
		// Test Docker image listing
		images, err := r.ListImages(ctx, mockConn, false)
		assert.NoError(t, err, "ListImages should succeed")
		// In mock environment, this might return empty or mock data
		assert.NotNil(t, images, "Images list should not be nil")

		// Test pulling an image (mock will simulate success)
		err = r.PullImage(ctx, mockConn, "nginx:latest")
		assert.NoError(t, err, "PullImage should succeed")

		// Test removing an image
		err = r.RemoveImage(ctx, mockConn, "nginx:latest", false)
		assert.NoError(t, err, "RemoveImage should succeed")
	})

	t.Run("DockerContainerOperations", func(t *testing.T) {
		// Test container listing
		containers, err := r.ListContainers(ctx, mockConn, true)
		assert.NoError(t, err, "ListContainers should succeed")
		assert.NotNil(t, containers, "Containers list should not be nil")

		// Test container creation and management
		containerConfig := runner.ContainerConfig{
			Image:  "nginx:latest",
			Name:   "test-nginx",
			Ports:  []string{"80:80"},
			Env:    []string{"ENV=test"},
			Detach: true,
		}

		containerID, err := r.CreateContainer(ctx, mockConn, containerConfig)
		assert.NoError(t, err, "CreateContainer should succeed")
		assert.NotEmpty(t, containerID, "Container ID should not be empty")

		// Test starting container
		err = r.StartContainer(ctx, mockConn, containerID)
		assert.NoError(t, err, "StartContainer should succeed")

		// Test container status
		status, err := r.GetContainerStatus(ctx, mockConn, containerID)
		assert.NoError(t, err, "GetContainerStatus should succeed")
		_ = status // Status depends on mock implementation

		// Test stopping container
		err = r.StopContainer(ctx, mockConn, containerID, 10*time.Second)
		assert.NoError(t, err, "StopContainer should succeed")

		// Test removing container
		err = r.RemoveContainer(ctx, mockConn, containerID, false)
		assert.NoError(t, err, "RemoveContainer should succeed")
	})

	t.Run("DockerNetworkOperations", func(t *testing.T) {
		// Test network listing
		networks, err := r.ListNetworks(ctx, mockConn)
		assert.NoError(t, err, "ListNetworks should succeed")
		assert.NotNil(t, networks, "Networks list should not be nil")

		// Test network creation
		networkConfig := runner.NetworkConfig{
			Name:   "test-network",
			Driver: "bridge",
			Subnet: "172.20.0.0/16",
		}

		networkID, err := r.CreateNetwork(ctx, mockConn, networkConfig)
		assert.NoError(t, err, "CreateNetwork should succeed")
		assert.NotEmpty(t, networkID, "Network ID should not be empty")

		// Test network removal
		err = r.RemoveNetwork(ctx, mockConn, networkID)
		assert.NoError(t, err, "RemoveNetwork should succeed")
	})

	t.Run("DockerVolumeOperations", func(t *testing.T) {
		// Test volume listing
		volumes, err := r.ListVolumes(ctx, mockConn)
		assert.NoError(t, err, "ListVolumes should succeed")
		assert.NotNil(t, volumes, "Volumes list should not be nil")

		// Test volume creation
		volumeConfig := runner.VolumeConfig{
			Name:   "test-volume",
			Driver: "local",
		}

		volumeName, err := r.CreateVolume(ctx, mockConn, volumeConfig)
		assert.NoError(t, err, "CreateVolume should succeed")
		assert.NotEmpty(t, volumeName, "Volume name should not be empty")

		// Test volume removal
		err = r.RemoveVolume(ctx, mockConn, volumeName, false)
		assert.NoError(t, err, "RemoveVolume should succeed")
	})

	t.Run("DockerErrorHandling", func(t *testing.T) {
		// Test with invalid image
		mockConn.SetupCommandError("docker pull invalid:tag", 1, "Error response from daemon: pull access denied")

		err := r.PullImage(ctx, mockConn, "invalid:tag")
		assert.Error(t, err, "Invalid image pull should fail")

		// Test with invalid container operation
		mockConn.SetupCommandError("docker start nonexistent", 1, "Error: No such container: nonexistent")

		err = r.StartContainer(ctx, mockConn, "nonexistent")
		assert.Error(t, err, "Starting nonexistent container should fail")
	})
}

// TestDockerComposeIntegration tests Docker Compose operations
func TestDockerComposeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	t.Run("ComposeOperations", func(t *testing.T) {
		composeFile := "/tmp/docker-compose.yml"
		composeContent := `version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
  db:
    image: mysql:5.7
    environment:
      MYSQL_ROOT_PASSWORD: secret`

		// Setup mock file content
		mockConn.SetFileContent(composeFile, []byte(composeContent))

		// Test compose up
		err := r.ComposeUp(ctx, mockConn, composeFile, true) // detached mode
		assert.NoError(t, err, "Compose up should succeed")

		// Test compose status
		status, err := r.ComposeStatus(ctx, mockConn, composeFile)
		assert.NoError(t, err, "Compose status should succeed")
		_ = status // Status depends on mock

		// Test compose down
		err = r.ComposeDown(ctx, mockConn, composeFile, false) // don't remove volumes
		assert.NoError(t, err, "Compose down should succeed")
	})
}

// Benchmark tests for Docker operations
func BenchmarkDockerOperations(b *testing.B) {
	mockConn := NewMockConnector()
	r := runner.NewRunner()
	ctx := context.Background()

	mockConn.SetupMockOS("ubuntu", "20.04", "amd64")

	b.Run("ListImages", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.ListImages(ctx, mockConn, false)
		}
	})

	b.Run("ListContainers", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.ListContainers(ctx, mockConn, true)
		}
	})
}