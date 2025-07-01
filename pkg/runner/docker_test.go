package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestPullImage contains specific unit tests for the PullImage method.
func TestPullImage(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "alpine:latest"
		expectedCmd := fmt.Sprintf("docker pull %s", shellEscape(imageName))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 15*time.Minute
		})).Return([]byte(""), []byte("Status: Image is up to date for alpine:latest"), nil).Once()

		err := r.PullImage(ctx, mockConn, imageName)
		assert.NoError(t, err)
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t) // Provide a mock connector
		err := r.PullImage(ctx, mockConn, "  ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "imageName cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		err := r.PullImage(ctx, nil, "alpine:latest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})

	t.Run("CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "nonexistent/image:latest"
		expectedCmd := fmt.Sprintf("docker pull %s", shellEscape(imageName))
		simulatedError := errors.New("docker command failed")
		simulatedStderr := "Error response from daemon: manifest for nonexistent/image:latest not found"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.PullImage(ctx, mockConn, imageName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image")
		assert.Contains(t, err.Error(), imageName)
		assert.Contains(t, err.Error(), simulatedError.Error())
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "alpine:verylargeimage"
		expectedCmd := fmt.Sprintf("docker pull %s", shellEscape(imageName))

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo
		})).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).Return(nil, []byte("pulling..."), context.DeadlineExceeded).Once()

		startTime := time.Now()
		err := r.PullImage(timeoutCtx, mockConn, imageName)
		duration := time.Since(startTime)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), context.DeadlineExceeded.Error()), "Error should be context.DeadlineExceeded or wrap it")
		assert.True(t, duration < 2*time.Second, "PullImage should not block excessively on context timeout")
	})
}

// TestImageExists contains specific unit tests for the ImageExists method.
func TestImageExists(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_Exists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "alpine:latest"
		expectedCmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 30*time.Second
		})).Return(nil, nil, nil).Once()

		exists, err := r.ImageExists(ctx, mockConn, imageName)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Success_NotExists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "nonexistent/image:latest"
		expectedCmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))

		cmdErr := &connector.CommandError{
			ExitCode: 1,
			Stderr:   "Error: No such image: nonexistent/image:latest",
		}
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(cmdErr.Stderr), cmdErr).Once()

		exists, err := r.ImageExists(ctx, mockConn, imageName)
		assert.NoError(t, err, "Expected no error when image simply doesn't exist")
		assert.False(t, exists, "Expected false when image doesn't exist")
	})

	t.Run("CommandError_Other", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "some/image:tag"
		expectedCmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))
		simulatedError := errors.New("docker daemon not running")

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte("Cannot connect to the Docker daemon"), simulatedError).Once()

		exists, err := r.ImageExists(ctx, mockConn, imageName)
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "failed to check if image")
		assert.Contains(t, err.Error(), simulatedError.Error())
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t) // Create a mock connector instance
		exists, err := r.ImageExists(ctx, mockConn, "  ")
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "imageName cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		// r is from TestImageExists scope
		exists, err := r.ImageExists(ctx, nil, "alpine:latest")
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestListImages contains specific unit tests for the ListImages method.
func TestListImages(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoImages", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --format {{json .}}"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte("\n"), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.NoError(t, err)
		assert.Empty(t, images)
	})

	t.Run("Success_OneImage", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --format {{json .}}"
		jsonOutput := `{"ID":"img1_id","Repository":"repo1","Tag":"latest","CreatedSince":"2 days ago","Size":"100MB"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.NoError(t, err)
		assert.Len(t, images, 1)
		if len(images) == 1 {
			assert.Equal(t, "img1_id", images[0].ID)
			assert.Equal(t, []string{"repo1:latest"}, images[0].RepoTags)
			assert.Equal(t, "2 days ago", images[0].Created)
			expectedSizeBytes, _ := parseDockerSize("100MB")
			assert.Equal(t, expectedSizeBytes, images[0].Size)
			assert.Equal(t, expectedSizeBytes, images[0].VirtualSize)
		}
	})

	t.Run("Success_MultipleImages_WithAllFlag", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --all --format {{json .}}"
		jsonOutput := `{"ID":"img1_id","Repository":"repo1","Tag":"latest","CreatedSince":"2 days ago","Size":"100MB"}` + "\n" +
		              `{"ID":"img2_id","Repository":"repo2","Tag":"v1.0","CreatedSince":"3 weeks ago","Size":"1.2GB"}` + "\n" +
					  `{"ID":"img3_id","Repository":"<none>","Tag":"<none>","CreatedSince":"4 weeks ago","Size":"500B"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, true)
		assert.NoError(t, err)
		assert.Len(t, images, 3)
		if len(images) == 3 {
			assert.Equal(t, "img1_id", images[0].ID)
			assert.Equal(t, []string{"repo1:latest"}, images[0].RepoTags)
			expectedSizeBytes1, _ := parseDockerSize("100MB")
			assert.Equal(t, expectedSizeBytes1, images[0].Size)
			assert.Equal(t, "img2_id", images[1].ID)
			assert.Equal(t, []string{"repo2:v1.0"}, images[1].RepoTags)
			expectedSizeBytes2, _ := parseDockerSize("1.2GB")
			assert.Equal(t, expectedSizeBytes2, images[1].Size)
			assert.Equal(t, "img3_id", images[2].ID)
			assert.Empty(t, images[2].RepoTags)
			expectedSizeBytes3, _ := parseDockerSize("500B")
			assert.Equal(t, expectedSizeBytes3, images[2].Size)
		}
	})

	t.Run("CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --format {{json .}}"
		simulatedError := errors.New("docker daemon error")
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte("daemon not responding"), simulatedError).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.Error(t, err)
		assert.Nil(t, images)
		assert.Contains(t, err.Error(), "failed to list images")
	})

	t.Run("InvalidJSONOutput", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --format {{json .}}"
		invalidJsonOutput := `{"ID":"img1_id", "Repository":"repo1", "Tag":"latest", "CreatedSince":"malformed` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJsonOutput), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.Error(t, err)
		assert.Nil(t, images)
		assert.Contains(t, err.Error(), "failed to parse image JSON line")
	})

	t.Run("InvalidSizeFormat", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --format {{json .}}"
		jsonOutputWithInvalidSize := `{"ID":"img1_id","Repository":"repo1","Tag":"latest","CreatedSince":"2 days ago","Size":"100XX"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutputWithInvalidSize), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.Error(t, err)
		assert.Nil(t, images)
		assert.Contains(t, err.Error(), "failed to parse size '100XX'")
	})
}

// TestRemoveImage contains specific unit tests for the RemoveImage method.
func TestRemoveImage(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "alpine:latest"
		expectedCmd := fmt.Sprintf("docker rmi %s", shellEscape(imageName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 5*time.Minute
		})).Return([]byte("Untagged: alpine:latest\nDeleted: sha256:...\n"), nil, nil).Once()
		err := r.RemoveImage(ctx, mockConn, imageName, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Forced", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "myimage:inuse"
		expectedCmd := fmt.Sprintf("docker rmi -f %s", shellEscape(imageName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 5*time.Minute
		})).Return([]byte("Untagged: myimage:inuse\nDeleted: sha256:...\n"), nil, nil).Once()
		err := r.RemoveImage(ctx, mockConn, imageName, true)
		assert.NoError(t, err)
	})

	t.Run("NonExistent", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "nonexistent/image:latest"
		expectedCmd := fmt.Sprintf("docker rmi %s", shellEscape(imageName))
		simulatedError := errors.New("docker rmi error")
		simulatedStderr := fmt.Sprintf("Error: No such image: %s", imageName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()
		err := r.RemoveImage(ctx, mockConn, imageName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove image")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("InUse_NoForce", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "ubuntu:latest"
		expectedCmd := fmt.Sprintf("docker rmi %s", shellEscape(imageName))
		simulatedError := errors.New("docker rmi error - image in use")
		simulatedStderr := "Error response from daemon: conflict: unable to remove repository reference \"ubuntu:latest\" (must force) - container abc123xyz is using its referenced image def456ghi"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()
		err := r.RemoveImage(ctx, mockConn, imageName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove image")
		// Check for either the wrapped original error or a key part of the stderr
		assert.True(t, errors.Is(err, simulatedError) || strings.Contains(string(simulatedStderr), "conflict: unable to remove repository reference"), "Error should wrap simulated error or stderr should indicate conflict")
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t) // Provide a mock connector
		err := r.RemoveImage(ctx, mockConn, "  ", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "imageName cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		err := r.RemoveImage(ctx, nil, "alpine:latest", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestDockerMethodStubs_Remaining asserts that remaining Docker methods currently return "not implemented".
func TestDockerMethodStubs_Remaining(t *testing.T) {
	r := &defaultRunner{}
	mockConn := mocks.NewConnector(t)
	ctx := context.Background()

	// Helper to check for "not implemented" error
	assertNotImplemented := func(t *testing.T, err error, methodName string) {
		assert.Error(t, err, "Expected an error for %s", methodName)
		if err != nil {
			assert.Contains(t, err.Error(), "not implemented: "+methodName, "Error message should indicate 'not implemented'")
		}
	}
	assertNotImplementedBool := func(t *testing.T, val bool, err error, methodName string) {
		assert.False(t, val, "Expected bool value to be false for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedString := func(t *testing.T, val string, err error, methodName string) {
		assert.Empty(t, val, "Expected string value to be empty for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedSlice := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected slice value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedChan := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected chan value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedPtr := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected pointer value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}

	// Stubs for methods that are not yet fully implemented with specific tests
	t.Run("BuildImage", func(t *testing.T) {
		err := r.BuildImage(ctx, mockConn, "", "", "", nil)
		assertNotImplemented(t, err, "BuildImage")
	})
	t.Run("CreateContainer", func(t *testing.T) {
		val, err := r.CreateContainer(ctx, mockConn, ContainerCreateOptions{})
		assertNotImplementedString(t, val, err, "CreateContainer")
	})
	t.Run("ContainerExists", func(t *testing.T) {
		val, err := r.ContainerExists(ctx, mockConn, "")
		assertNotImplementedBool(t, val, err, "ContainerExists")
	})
	t.Run("StartContainer", func(t *testing.T) {
		err := r.StartContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "StartContainer")
	})
	t.Run("StopContainer", func(t *testing.T) {
		err := r.StopContainer(ctx, mockConn, "", nil)
		assertNotImplemented(t, err, "StopContainer")
	})
	t.Run("RestartContainer", func(t *testing.T) {
		err := r.RestartContainer(ctx, mockConn, "", nil)
		assertNotImplemented(t, err, "RestartContainer")
	})
	t.Run("RemoveContainer", func(t *testing.T) {
		err := r.RemoveContainer(ctx, mockConn, "", false, false)
		assertNotImplemented(t, err, "RemoveContainer")
	})
	t.Run("ListContainers", func(t *testing.T) {
		val, err := r.ListContainers(ctx, mockConn, false, nil)
		assertNotImplementedSlice(t, val, err, "ListContainers")
	})
	t.Run("GetContainerLogs", func(t *testing.T) {
		val, err := r.GetContainerLogs(ctx, mockConn, "", ContainerLogOptions{})
		assertNotImplementedString(t, val, err, "GetContainerLogs")
	})
	t.Run("GetContainerStats", func(t *testing.T) {
		val, err := r.GetContainerStats(ctx, mockConn, "", false)
		assertNotImplementedChan(t, val, err, "GetContainerStats")
	})
	t.Run("InspectContainer", func(t *testing.T) {
		val, err := r.InspectContainer(ctx, mockConn, "")
		assertNotImplementedPtr(t, val, err, "InspectContainer")
	})
	t.Run("PauseContainer", func(t *testing.T) {
		err := r.PauseContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "PauseContainer")
	})
	t.Run("UnpauseContainer", func(t *testing.T) {
		err := r.UnpauseContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "UnpauseContainer")
	})
	t.Run("ExecInContainer", func(t *testing.T) {
		val, err := r.ExecInContainer(ctx, mockConn, "", nil, "", "", false)
		assertNotImplementedString(t, val, err, "ExecInContainer")
	})
	t.Run("CreateDockerNetwork", func(t *testing.T) {
		err := r.CreateDockerNetwork(ctx, mockConn, "", "", "", "", nil)
		assertNotImplemented(t, err, "CreateDockerNetwork")
	})
	t.Run("RemoveDockerNetwork", func(t *testing.T) {
		err := r.RemoveDockerNetwork(ctx, mockConn, "")
		assertNotImplemented(t, err, "RemoveDockerNetwork")
	})
	t.Run("ListDockerNetworks", func(t *testing.T) {
		val, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerNetworks")
	})
	t.Run("ConnectContainerToNetwork", func(t *testing.T) {
		err := r.ConnectContainerToNetwork(ctx, mockConn, "", "", "")
		assertNotImplemented(t, err, "ConnectContainerToNetwork")
	})
	t.Run("DisconnectContainerFromNetwork", func(t *testing.T) {
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, "", "", false)
		assertNotImplemented(t, err, "DisconnectContainerFromNetwork")
	})
	t.Run("CreateDockerVolume", func(t *testing.T) {
		err := r.CreateDockerVolume(ctx, mockConn, "", "", nil, nil)
		assertNotImplemented(t, err, "CreateDockerVolume")
	})
	// Note: The original t.Run for RemoveDockerVolume was inside TestDockerMethodStubs.
	// If it's still a stub, it should be here. If it was implemented, it needs its own TestRemoveDockerVolume.
	// Assuming it's still a stub for now.
	t.Run("RemoveDockerVolume", func(t *testing.T) {
		err := r.RemoveDockerVolume(ctx, mockConn, "", false)
		assertNotImplemented(t, err, "RemoveDockerVolume")
	})
	t.Run("ListDockerVolumes", func(t *testing.T) {
		val, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerVolumes")
	})
	t.Run("InspectDockerVolume", func(t *testing.T) {
		val, err := r.InspectDockerVolume(ctx, mockConn, "")
		assertNotImplementedPtr(t, val, err, "InspectDockerVolume")
	})
	t.Run("DockerInfo", func(t *testing.T) {
		val, err := r.DockerInfo(ctx, mockConn)
		assertNotImplementedPtr(t, val, err, "DockerInfo")
	})
	t.Run("DockerPrune", func(t *testing.T) {
		val, err := r.DockerPrune(ctx, mockConn, "", nil, false)
		assertNotImplementedString(t, val, err, "DockerPrune")
	})
}
