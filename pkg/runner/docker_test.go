package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks" // Ensure this path is correct
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
			return opts.Sudo && opts.Timeout == DefaultDockerPullTimeout
		})).Return([]byte(""), []byte("Status: Image is up to date for alpine:latest"), nil).Once()

		err := r.PullImage(ctx, mockConn, imageName)
		assert.NoError(t, err)
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
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
			// Simulate work that exceeds the context deadline
			deadline, ok := timeoutCtx.Deadline()
			if ok {
				time.Sleep(time.Until(deadline) + 50*time.Millisecond) // Sleep just past the deadline
			} else {
				time.Sleep(100 * time.Millisecond) // Fallback sleep
			}
		}).Return(nil, []byte("pulling..."), context.DeadlineExceeded).Once()


		err := r.PullImage(timeoutCtx, mockConn, imageName)

		assert.Error(t, err)
		// Check if the error is context.DeadlineExceeded or wraps it
		// This depends on how the connector.Exec and its underlying mechanisms propagate context errors.
		// For this test, we assume it might be wrapped.
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
			t.Errorf("expected error to be or wrap context.DeadlineExceeded, got %v", err)
		}
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
			return opts.Sudo && opts.Timeout == DefaultDockerInspectTimeout
		})).Return(nil, nil, nil).Once()

		exists, err := r.ImageExists(ctx, mockConn, imageName)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Success_NotExists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "nonexistent/image:latest"
		expectedCmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))

		// Simulate connector.CommandError for "not found"
		cmdErr := &connector.CommandError{
			ExitCode: 1, // Typical exit code for "not found"
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
		mockConn := mocks.NewConnector(t)
		exists, err := r.ImageExists(ctx, mockConn, "  ")
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "imageName cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
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
		// Note: The CreatedAt field format from Docker CLI can be inconsistent or locale-dependent.
		// For robust parsing, `docker image inspect <id> --format '{{.Created}}'` is better for timestamps.
		// Here, we simulate a common JSON output for `docker images --format {{json .}}`.
		// The `Created` field in `image.Summary` expects a Unix timestamp (int64).
		// If `CreatedAt` is provided by the JSON, `ListImages` attempts to parse it.
		// Let's assume a parsable `CreatedAt` for this test.
		testTime := time.Now().Add(-24 * time.Hour)
		testTimeStr := testTime.Format("2006-01-02 15:04:05 -0700 MST") // Example format
		jsonOutput := fmt.Sprintf(`{"ID":"img1_id","Repository":"repo1","Tag":"latest","CreatedSince":"1 day ago", "CreatedAt": "%s", "Size":"100MB"}`, testTimeStr) + "\n"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()
		images, err := r.ListImages(ctx, mockConn, false)
		assert.NoError(t, err)
		assert.Len(t, images, 1)
		if len(images) == 1 {
			assert.Equal(t, "img1_id", images[0].ID)
			assert.Equal(t, []string{"repo1:latest"}, images[0].RepoTags)
			expectedSizeBytes, _ := parseDockerSize("100MB")
			assert.Equal(t, expectedSizeBytes, images[0].Size)
			// Check if CreatedAt was parsed correctly into the Created field (Unix timestamp)
			// Allow for minor discrepancies if parsing involves time zones or rounding.
			assert.InDelta(t, testTime.Unix(), images[0].Created, 1, "Parsed 'Created' timestamp mismatch")
		}
	})

	t.Run("Success_MultipleImages_WithAllFlag", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker images --all --format {{json .}}"
		time1 := time.Now().Add(-48 * time.Hour)
		time1Str := time1.Format("2006-01-02 15:04:05 -0700 MST")
		time2 := time.Now().Add(-72 * time.Hour)
		time2Str := time2.Format("2006-01-02 15:04:05 -0700 MST")


		jsonOutput := fmt.Sprintf(`{"ID":"img1_id","Repository":"repo1","Tag":"latest","CreatedSince":"2 days ago", "CreatedAt":"%s", "Size":"100MB"}`, time1Str) + "\n" +
			fmt.Sprintf(`{"ID":"img2_id","Repository":"repo2","Tag":"v1.0","CreatedSince":"3 days ago", "CreatedAt":"%s", "Size":"1.2GB"}`, time2Str) + "\n" +
			`{"ID":"img3_id","Repository":"<none>","Tag":"<none>","CreatedSince":"4 weeks ago", "CreatedAt":"", "Size":"500B"}` + "\n" // No CreatedAt for one
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		images, err := r.ListImages(ctx, mockConn, true)
		assert.NoError(t, err)
		assert.Len(t, images, 3)
		if len(images) == 3 {
			assert.Equal(t, "img1_id", images[0].ID)
			assert.Equal(t, []string{"repo1:latest"}, images[0].RepoTags)
			expectedSizeBytes1, _ := parseDockerSize("100MB")
			assert.Equal(t, expectedSizeBytes1, images[0].Size)
			assert.InDelta(t, time1.Unix(), images[0].Created, 1)


			assert.Equal(t, "img2_id", images[1].ID)
			assert.Equal(t, []string{"repo2:v1.0"}, images[1].RepoTags)
			expectedSizeBytes2, _ := parseDockerSize("1.2GB")
			assert.Equal(t, expectedSizeBytes2, images[1].Size)
			assert.InDelta(t, time2.Unix(), images[1].Created, 1)

			assert.Equal(t, "img3_id", images[2].ID)
			assert.Empty(t, images[2].RepoTags) // <none>:<none> results in empty RepoTags
			expectedSizeBytes3, _ := parseDockerSize("500B")
			assert.Equal(t, expectedSizeBytes3, images[2].Size)
			assert.Equal(t, int64(0), images[2].Created) // No CreatedAt, so Created should be 0
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
			return opts.Sudo && opts.Timeout == DefaultDockerRMTimeout
		})).Return([]byte("Untagged: alpine:latest\nDeleted: sha256:...\n"), nil, nil).Once()
		err := r.RemoveImage(ctx, mockConn, imageName, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Forced", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		imageName := "myimage:inuse"
		expectedCmd := fmt.Sprintf("docker rmi -f %s", shellEscape(imageName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerRMTimeout
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
		// Check if the error message contains the essence of "conflict" or "unable to remove" from stderr
		assert.True(t, strings.Contains(err.Error(), "conflict: unable to remove") || strings.Contains(err.Error(), simulatedError.Error()),
			"Error should indicate conflict or wrap the original error")
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
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

// TestBuildImage contains specific unit tests for the BuildImage method.
func TestBuildImage(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	defaultImageNameTag := "myimage:latest"
	defaultContextPath := "/remote/context" // Assuming context is a directory path
	defaultDockerfilePath := ""             // Empty means use Dockerfile at root of contextPath

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		// Docker build command structure: docker build [OPTIONS] PATH | URL | -
		// Here, contextPath is PATH.
		expectedCmd := fmt.Sprintf("docker build -t %s %s", shellEscape(defaultImageNameTag), shellEscape(defaultContextPath))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerBuildTimeout
		})).Return([]byte("Successfully built..."), nil, nil).Once()

		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, nil)
		assert.NoError(t, err)
	})

	t.Run("Success_WithDockerfileAndBuildArgs", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		dockerfilePath := "/remote/context/custom.Dockerfile" // Dockerfile path
		buildArgs := map[string]string{"VERSION": "1.0", "EMPTY_ARG": "", "ARG_WITH_SPACE": "value with space"}

		// Construct the expected command string carefully, especially with build args
		var expectedCmdParts []string
		expectedCmdParts = append(expectedCmdParts, "docker", "build")
		expectedCmdParts = append(expectedCmdParts, "-f", shellEscape(dockerfilePath))
		expectedCmdParts = append(expectedCmdParts, "-t", shellEscape(defaultImageNameTag))

		// Order of build-args might not be guaranteed, so check for their presence
		expectedArg1 := fmt.Sprintf("--build-arg %s", shellEscape("VERSION=1.0"))
		expectedArg2 := fmt.Sprintf("--build-arg %s", shellEscape("EMPTY_ARG="))
		expectedArg3 := fmt.Sprintf("--build-arg %s", shellEscape("ARG_WITH_SPACE=value with space"))


		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			// Basic check for command prefix and suffix
			hasPrefix := strings.HasPrefix(cmd, "docker build")
			hasSuffix := strings.HasSuffix(cmd, shellEscape(defaultContextPath))
			hasFileFlag := strings.Contains(cmd, fmt.Sprintf("-f %s", shellEscape(dockerfilePath)))
			hasTagFlag := strings.Contains(cmd, fmt.Sprintf("-t %s", shellEscape(defaultImageNameTag)))
			hasArg1 := strings.Contains(cmd, expectedArg1)
			hasArg2 := strings.Contains(cmd, expectedArg2)
			hasArg3 := strings.Contains(cmd, expectedArg3)
			return hasPrefix && hasSuffix && hasFileFlag && hasTagFlag && hasArg1 && hasArg2 && hasArg3
		}), mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo
		})).Return([]byte("Successfully built..."), nil, nil).Once()

		err := r.BuildImage(ctx, mockConn, dockerfilePath, defaultImageNameTag, defaultContextPath, buildArgs)
		assert.NoError(t, err)
	})


	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker build -t %s %s", shellEscape(defaultImageNameTag), shellEscape(defaultContextPath))
		simulatedError := errors.New("build failed")
		simulatedStdout := "Step 1/2 : FROM alpine\n ---> abcdef123456" // Example stdout
		simulatedStderr := "Error: The command '/bin/sh -c unknown-command' returned a non-zero code: 127"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(simulatedStdout), []byte(simulatedStderr), simulatedError).Once()

		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build image")
		assert.Contains(t, err.Error(), simulatedError.Error())
		assert.Contains(t, err.Error(), "Stdout: "+simulatedStdout) // Check for stdout in error
		assert.Contains(t, err.Error(), "Stderr: "+simulatedStderr) // Check for stderr in error
	})

	t.Run("Failure_EmptyImageNameTag", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, " ", defaultContextPath, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "imageNameAndTag cannot be empty")
	})

	t.Run("Failure_EmptyContextPath", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, " ", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "contextPath cannot be empty")
	})

	t.Run("Failure_EmptyBuildArgKey", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		invalidBuildArgs := map[string]string{"": "value"}
		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, invalidBuildArgs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "buildArg key cannot be empty")
	})


	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.BuildImage(ctx, nil, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestCreateContainer contains specific unit tests for the CreateContainer method.
func TestCreateContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	defaultImage := "alpine:latest"
	defaultContainerID := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6" // Example 64 char ID

	baseOptions := ContainerCreateOptions{ImageName: defaultImage}

	t.Run("Success_Minimal", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := baseOptions
		expectedCmd := fmt.Sprintf("docker create %s", shellEscape(defaultImage))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(execOpts *connector.ExecOptions) bool {
			return execOpts.Sudo && execOpts.Timeout == DefaultDockerCreateTimeout
		})).Return([]byte(defaultContainerID+"\n"), nil, nil).Once() // Docker create usually adds a newline

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})

	t.Run("Success_WithNameAndPorts", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName:     defaultImage,
			ContainerName: "my-container",
			Ports: []ContainerPortMapping{
				{HostPort: "8080", ContainerPort: "80"},                                // 8080:80
				{HostIP: "127.0.0.1", HostPort: "9090", ContainerPort: "90", Protocol: "udp"}, // 127.0.0.1:9090:90/udp
			},
		}
		// Command construction can be complex, match key parts
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "docker create") &&
				strings.Contains(cmd, fmt.Sprintf("--name %s", shellEscape("my-container"))) &&
				strings.Contains(cmd, fmt.Sprintf("-p %s", shellEscape("8080:80"))) &&
				strings.Contains(cmd, fmt.Sprintf("-p %s", shellEscape("127.0.0.1:9090:90/udp"))) &&
				strings.HasSuffix(cmd, shellEscape(defaultImage))
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(defaultContainerID), nil, nil).Once()

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})

	t.Run("Success_WithVolumesAndEnv", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName: defaultImage,
			Volumes: []ContainerMount{
				{Source: "/host/path", Destination: "/container/path"},             // /host/path:/container/path
				{Source: "named-volume", Destination: "/data", Mode: "ro"},       // named-volume:/data:ro
				{Source: "/another/host/path", Destination: "/another/container/path", Mode: "z"}, // Test with 'z' option
			},
			EnvVars: []string{"FOO=bar", "BAZ=qux", "EMPTY_VAL="},
		}

		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "docker create") &&
				strings.Contains(cmd, fmt.Sprintf("-v %s", shellEscape("/host/path:/container/path"))) &&
				strings.Contains(cmd, fmt.Sprintf("-v %s", shellEscape("named-volume:/data:ro"))) &&
				strings.Contains(cmd, fmt.Sprintf("-v %s", shellEscape("/another/host/path:/another/container/path:z"))) &&
				strings.Contains(cmd, fmt.Sprintf("-e %s", shellEscape("FOO=bar"))) &&
				strings.Contains(cmd, fmt.Sprintf("-e %s", shellEscape("BAZ=qux"))) &&
				strings.Contains(cmd, fmt.Sprintf("-e %s", shellEscape("EMPTY_VAL="))) &&
				strings.HasSuffix(cmd, shellEscape(defaultImage))
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(defaultContainerID), nil, nil).Once()

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})

	t.Run("Success_WithEntrypointAndCommand", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName:  defaultImage,
			Entrypoint: []string{"/app/custom-entry"},      // Entrypoint is a single command path
			Command:    []string{"arg1", "val with space"}, // Command is args for the entrypoint
		}
		// Expected: docker create --entrypoint '/app/custom-entry' alpine:latest 'arg1' 'val with space'
		expectedCmd := fmt.Sprintf("docker create --entrypoint %s %s %s %s",
			shellEscape("/app/custom-entry"),
			shellEscape(defaultImage),
			shellEscape("arg1"),
			shellEscape("val with space"))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(defaultContainerID), nil, nil).Once()

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})

	t.Run("Success_WithOnlyCommandNoEntrypointOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName:  defaultImage,
			Command:    []string{"echo", "hello world"},
		}
		// Expected: docker create alpine:latest 'echo' 'hello world'
		expectedCmd := fmt.Sprintf("docker create %s %s %s",
			shellEscape(defaultImage),
			shellEscape("echo"),
			shellEscape("hello world"))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(defaultContainerID), nil, nil).Once()

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})


	t.Run("Success_WithRestartPolicyPrivilegedAutoRemove", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName:     defaultImage,
			RestartPolicy: "on-failure:3",
			Privileged:    true,
			AutoRemove:    true,
		}
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "docker create") &&
				strings.Contains(cmd, fmt.Sprintf("--restart %s", shellEscape("on-failure:3"))) &&
				strings.Contains(cmd, "--privileged") &&
				strings.Contains(cmd, "--rm") &&
				strings.HasSuffix(cmd, shellEscape(defaultImage))
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(defaultContainerID), nil, nil).Once()

		id, err := r.CreateContainer(ctx, mockConn, opts)
		assert.NoError(t, err)
		assert.Equal(t, defaultContainerID, id)
	})


	t.Run("Failure_EmptyImageName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{ImageName: " "}
		_, err := r.CreateContainer(ctx, mockConn, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "options.ImageName cannot be empty")
	})

	t.Run("Failure_InvalidVolumeSpec", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := ContainerCreateOptions{
			ImageName: defaultImage,
			Volumes: []ContainerMount{
				{Source: "", Destination: "/container/path"}, // Empty source
			},
		}
		_, err := r.CreateContainer(ctx, mockConn, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "volume source and destination cannot be empty")
	})


	t.Run("Failure_NilConnector", func(t *testing.T) {
		opts := baseOptions
		_, err := r.CreateContainer(ctx, nil, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})

	t.Run("Failure_DockerCommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := baseOptions
		expectedCmd := fmt.Sprintf("docker create %s", shellEscape(defaultImage))
		simulatedError := errors.New("docker create failed")
		simulatedStderr := "Error response from daemon: Something went wrong"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.CreateContainer(ctx, mockConn, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create container")
		assert.Contains(t, err.Error(), simulatedError.Error())
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := baseOptions
		expectedCmd := fmt.Sprintf("docker create %s", shellEscape(defaultImage))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte("  \n"), nil, nil).Once() // Empty/whitespace ID

		_, err := r.CreateContainer(ctx, mockConn, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker create succeeded but returned an empty container ID")
	})
}

// TestContainerExists tests the ContainerExists method.
func TestContainerExists(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_Exists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "my-existing-container"
		expectedCmd := fmt.Sprintf("docker inspect %s > /dev/null 2>&1", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()

		exists, err := r.ContainerExists(ctx, mockConn, containerName)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Success_NotExists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "non-existent-container"
		expectedCmd := fmt.Sprintf("docker inspect %s > /dev/null 2>&1", shellEscape(containerName))
		cmdErr := &connector.CommandError{ExitCode: 1, Stderr: "Error: No such object: " + containerName}
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(cmdErr.Stderr), cmdErr).Once()

		exists, err := r.ContainerExists(ctx, mockConn, containerName)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Failure_CommandError_Other", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "some-container"
		expectedCmd := fmt.Sprintf("docker inspect %s > /dev/null 2>&1", shellEscape(containerName))
		simulatedError := errors.New("docker daemon error")
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte("daemon not responding"), simulatedError).Once()

		exists, err := r.ContainerExists(ctx, mockConn, containerName)
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "failed to check if container")
	})

	t.Run("Failure_EmptyContainerNameOrID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.ContainerExists(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.False(t, false) // exists should be false on error
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.ContainerExists(ctx, nil, "some-container")
		assert.Error(t, err)
		assert.False(t, false) // exists should be false on error
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestStartContainer contains specific unit tests for the StartContainer method.
func TestStartContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-stopped-container"
		expectedCmd := fmt.Sprintf("docker start %s", shellEscape(containerID))

		// Docker start outputs the container name/ID on success to stdout.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerStartTimeout
		})).Return([]byte(containerID), nil, nil).Once()

		err := r.StartContainer(ctx, mockConn, containerID)
		assert.NoError(t, err)
	})

	t.Run("AlreadyStarted_NoError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-running-container"
		expectedCmd := fmt.Sprintf("docker start %s", shellEscape(containerID))

		// Docker start CLI often exits 0 even if already started,
		// but might print the ID to stdout or a warning to stderr.
		// If it exits 0, our function should not error.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).
			Return([]byte(containerID), []byte(fmt.Sprintf("Warning: container %s already started", containerID)), nil).Once()

		err := r.StartContainer(ctx, mockConn, containerID)
		assert.NoError(t, err, "Starting an already started container should not be an error if CLI exits 0")
	})

	t.Run("NonExistentContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "no-such-container"
		expectedCmd := fmt.Sprintf("docker start %s", shellEscape(containerID))
		simulatedError := errors.New("docker start error")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerID)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.StartContainer(ctx, mockConn, containerID)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to start container")
			assert.Contains(t, err.Error(), simulatedStderr) // The original stderr should be part of the error message
		}
	})

	t.Run("EmptyContainerNameOrID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.StartContainer(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		err := r.StartContainer(ctx, nil, "some-container")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestStopContainer contains specific unit tests for the StopContainer method.
func TestStopContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-running-container"
		// DefaultDockerStopGracePeriod is used by docker stop,
		// DefaultDockerStopExecTimeout is for the Exec call itself.
		expectedCmd := fmt.Sprintf("docker stop %s", shellEscape(containerID)) // No -t, so Docker uses its default.

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerStopExecTimeout // Our wrapper's exec timeout
		})).Return([]byte(containerID), nil, nil).Once() // Docker stop outputs container ID on success

		err := r.StopContainer(ctx, mockConn, containerID, nil) // nil for timeoutSeconds means use default
		assert.NoError(t, err)
	})

	t.Run("Success_WithTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-other-container"
		timeoutDuration := 5 * time.Second // Specific grace period for 'docker stop -t'
		expectedCmd := fmt.Sprintf("docker stop -t %d %s", int(timeoutDuration.Seconds()), shellEscape(containerID))

		// The Exec call's timeout should be the grace period + a buffer
		expectedExecTimeout := timeoutDuration + (30 * time.Second)


		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == expectedExecTimeout
		})).Return([]byte(containerID), nil, nil).Once()

		err := r.StopContainer(ctx, mockConn, containerID, &timeoutDuration)
		assert.NoError(t, err)
	})

	t.Run("Success_WithZeroTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-quickstop-container"
		timeoutDuration := 0 * time.Second
		expectedCmd := fmt.Sprintf("docker stop -t %d %s", int(timeoutDuration.Seconds()), shellEscape(containerID))
		expectedExecTimeout := timeoutDuration + (30 * time.Second)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == expectedExecTimeout
		})).Return([]byte(containerID), nil, nil).Once()

		err := r.StopContainer(ctx, mockConn, containerID, &timeoutDuration)
		assert.NoError(t, err)
	})


	t.Run("NonExistentOrAlreadyStoppedContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "no-such-container-or-stopped"
		expectedCmd := fmt.Sprintf("docker stop %s", shellEscape(containerID))
		simulatedError := errors.New("docker stop error")
		// Docker might output "No such container" to stderr and exit non-zero.
		// Or, if already stopped, it might output the ID and exit 0 (depends on Docker version).
		// Let's assume it errors for a non-existent one.
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerID)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerStopExecTimeout
		})).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.StopContainer(ctx, mockConn, containerID, nil)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to stop container")
			assert.Contains(t, err.Error(), simulatedStderr)
		}
	})

	t.Run("EmptyContainerNameOrID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.StopContainer(ctx, mockConn, " ", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		err := r.StopContainer(ctx, nil, "some-container", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestRestartContainer contains specific unit tests for the RestartContainer method.
func TestRestartContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "my-container-to-restart"
		// DefaultDockerRestartGracePeriod is used by docker restart if -t is not specified.
		// DefaultDockerRestartExecTimeout is for the Exec call itself.
		expectedCmd := fmt.Sprintf("docker restart %s", shellEscape(containerID))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			// The exec timeout should be our predefined DefaultDockerRestartExecTimeout
			return opts.Sudo && opts.Timeout == DefaultDockerRestartExecTimeout
		})).Return([]byte(containerID), nil, nil).Once() // Docker restart outputs container ID on success

		err := r.RestartContainer(ctx, mockConn, containerID, nil) // nil for timeoutSeconds
		assert.NoError(t, err)
	})

	t.Run("Success_WithTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "another-container-for-restart"
		timeoutDuration := 5 * time.Second // Specific grace period for 'docker restart -t'
		expectedCmd := fmt.Sprintf("docker restart -t %d %s", int(timeoutDuration.Seconds()), shellEscape(containerID))

		// The Exec call's timeout should be the grace period + a buffer (10s as per implementation)
		expectedExecTimeout := timeoutDuration + (10 * time.Second)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == expectedExecTimeout
		})).Return([]byte(containerID), nil, nil).Once()

		err := r.RestartContainer(ctx, mockConn, containerID, &timeoutDuration)
		assert.NoError(t, err)
	})

	t.Run("Success_WithZeroTimeoutOverride", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "quick-restart-container"
		timeoutDuration := 0 * time.Second
		expectedCmd := fmt.Sprintf("docker restart -t %d %s", int(timeoutDuration.Seconds()), shellEscape(containerID))
		expectedExecTimeout := timeoutDuration + (10 * time.Second)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == expectedExecTimeout
		})).Return([]byte(containerID), nil, nil).Once()

		err := r.RestartContainer(ctx, mockConn, containerID, &timeoutDuration)
		assert.NoError(t, err)
	})


	t.Run("NonExistentContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerID := "no-such-container-for-restart"
		expectedCmd := fmt.Sprintf("docker restart %s", shellEscape(containerID)) // No -t specified
		simulatedError := errors.New("docker restart error")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerID)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerRestartExecTimeout
		})).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RestartContainer(ctx, mockConn, containerID, nil)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to restart container")
			assert.Contains(t, err.Error(), simulatedStderr)
		}
	})

	t.Run("EmptyContainerNameOrID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.RestartContainer(ctx, mockConn, "  ", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("NilConnector", func(t *testing.T) {
		err := r.RestartContainer(ctx, nil, "some-container-for-restart", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestDockerMethodStubs_Remaining asserts that remaining Docker methods currently return "not implemented".
// This test should be updated as methods are implemented.
func TestDockerMethodStubs_Remaining(t *testing.T) {
	r := &defaultRunner{}
	// Use a real mock connector instance for methods that might require it, even if just for validation.
	mockConn := mocks.NewConnector(t)
	ctx := context.Background()

	// Helper to check for "not implemented" error
	assertNotImplemented := func(t *testing.T, err error, methodName string) {
		assert.Error(t, err, "Expected an error for %s", methodName)
		if err != nil { // Check err is not nil before asserting its content
			assert.Contains(t, err.Error(), "not implemented: "+methodName, "Error message for %s should indicate 'not implemented'", methodName)
		}
	}
	// Specific helpers for methods returning values other than just error
	assertNotImplementedString := func(t *testing.T, val string, err error, methodName string) {
		assert.Empty(t, val, "Expected string value to be empty for not implemented method %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedSlice := func(t *testing.T, val interface{}, err error, methodName string) {
		// Using testify's IsNil for broader slice/map/chan/ptr check
		assert.Nil(t, val, "Expected slice/map value to be nil for not implemented method %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedChan := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected chan value to be nil for not implemented method %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedPtr := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected pointer value to be nil for not implemented method %s", methodName)
		assertNotImplemented(t, err, methodName)
	}


	// List of methods to test for "not implemented"
	// These are placeholders and should be removed or moved to their own TestXxx functions as they get implemented.
	t.Run("RemoveContainer", func(t *testing.T) {
		err := r.RemoveContainer(ctx, mockConn, "some-container", false, false)
		assertNotImplemented(t, err, "RemoveContainer")
	})
	t.Run("ListContainers", func(t *testing.T) {
		val, err := r.ListContainers(ctx, mockConn, false, nil)
		assertNotImplementedSlice(t, val, err, "ListContainers")
	})
	t.Run("GetContainerLogs", func(t *testing.T) {
		val, err := r.GetContainerLogs(ctx, mockConn, "some-container", ContainerLogOptions{})
		assertNotImplementedString(t, val, err, "GetContainerLogs")
	})
	t.Run("GetContainerStats", func(t *testing.T) {
		val, err := r.GetContainerStats(ctx, mockConn, "some-container", false)
		assertNotImplementedChan(t, val, err, "GetContainerStats") // val is <-chan *container.StatsResponse
	})
	t.Run("InspectContainer", func(t *testing.T) {
		val, err := r.InspectContainer(ctx, mockConn, "some-container")
		assertNotImplementedPtr(t, val, err, "InspectContainer") // val is *container.InspectResponse
	})
	t.Run("PauseContainer", func(t *testing.T) {
		err := r.PauseContainer(ctx, mockConn, "some-container")
		assertNotImplemented(t, err, "PauseContainer")
	})
	t.Run("UnpauseContainer", func(t *testing.T) {
		err := r.UnpauseContainer(ctx, mockConn, "some-container")
		assertNotImplemented(t, err, "UnpauseContainer")
	})
	t.Run("ExecInContainer", func(t *testing.T) {
		val, err := r.ExecInContainer(ctx, mockConn, "some-container", []string{"ls"}, "", "", false)
		assertNotImplementedString(t, val, err, "ExecInContainer")
	})

	// Docker Network Methods
	t.Run("CreateDockerNetwork", func(t *testing.T) {
		err := r.CreateDockerNetwork(ctx, mockConn, "my-net", "", "", "", nil)
		assertNotImplemented(t, err, "CreateDockerNetwork")
	})
	t.Run("RemoveDockerNetwork", func(t *testing.T) {
		err := r.RemoveDockerNetwork(ctx, mockConn, "my-net")
		assertNotImplemented(t, err, "RemoveDockerNetwork")
	})
	t.Run("ListDockerNetworks", func(t *testing.T) {
		val, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerNetworks") // val is []NetworkResource
	})
	t.Run("ConnectContainerToNetwork", func(t *testing.T) {
		err := r.ConnectContainerToNetwork(ctx, mockConn, "my-net", "my-container", "")
		assertNotImplemented(t, err, "ConnectContainerToNetwork")
	})
	t.Run("DisconnectContainerFromNetwork", func(t *testing.T) {
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, "my-net", "my-container", false)
		assertNotImplemented(t, err, "DisconnectContainerFromNetwork")
	})

	// Docker Volume Methods
	t.Run("CreateDockerVolume", func(t *testing.T) {
		err := r.CreateDockerVolume(ctx, mockConn, "my-vol", "", nil, nil)
		assertNotImplemented(t, err, "CreateDockerVolume")
	})
	t.Run("RemoveDockerVolume", func(t *testing.T) {
		err := r.RemoveDockerVolume(ctx, mockConn, "my-vol", false)
		assertNotImplemented(t, err, "RemoveDockerVolume")
	})
	t.Run("ListDockerVolumes", func(t *testing.T) {
		val, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerVolumes") // val is []*Volume
	})
	t.Run("InspectDockerVolume", func(t *testing.T) {
		val, err := r.InspectDockerVolume(ctx, mockConn, "my-vol")
		assertNotImplementedPtr(t, val, err, "InspectDockerVolume") // val is *Volume
	})

	// Docker System Methods
	t.Run("DockerInfo", func(t *testing.T) {
		val, err := r.DockerInfo(ctx, mockConn)
		assertNotImplementedPtr(t, val, err, "DockerInfo") // val is *SystemInfo
	})
	t.Run("DockerPrune", func(t *testing.T) {
		val, err := r.DockerPrune(ctx, mockConn, "", nil, false)
		assertNotImplementedString(t, val, err, "DockerPrune")
	})

	// GetDockerServerVersion, CheckDockerInstalled, EnsureDockerService, CheckDockerRequirement,
	// PruneDockerBuildCache, GetHostArchitecture, ResolveDockerImage, DockerSave, DockerLoad
	// are already implemented and have their own tests or are utilities not part of the primary Docker operation stubs.
	// So they are not checked here for "not implemented".
}
