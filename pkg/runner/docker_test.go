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
		assert.True(t, errors.Is(err, simulatedError) || strings.Contains(string(simulatedStderr), "conflict: unable to remove repository reference"), "Error should wrap simulated error or stderr should indicate conflict")
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
	defaultContextPath := "/remote/context"
	defaultDockerfilePath := ""
	defaultBuildArgs := map[string]string{"VERSION": "1.0", "EMPTY_ARG": ""}


	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker build -t %s %s", shellEscape(defaultImageNameTag), shellEscape(defaultContextPath))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 60*time.Minute
		})).Return([]byte("Successfully built..."), nil, nil).Once()

		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, nil)
		assert.NoError(t, err)
	})

	t.Run("Success_WithDockerfileAndBuildArgs", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		dockerfilePath := "/remote/context/custom.Dockerfile"
		arg1 := shellEscape("VERSION=1.0")
		arg2 := shellEscape("EMPTY_ARG=")

		expectedCmdPart1 := fmt.Sprintf("docker build -f %s -t %s", shellEscape(dockerfilePath), shellEscape(defaultImageNameTag))

		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, expectedCmdPart1) &&
				   strings.Contains(cmd, "--build-arg "+arg1) &&
				   strings.Contains(cmd, "--build-arg "+arg2) &&
				   strings.HasSuffix(cmd, shellEscape(defaultContextPath))
		}), mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo
		})).Return([]byte("Successfully built..."), nil, nil).Once()

		err := r.BuildImage(ctx, mockConn, dockerfilePath, defaultImageNameTag, defaultContextPath, defaultBuildArgs)
		assert.NoError(t, err)
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker build -t %s %s", shellEscape(defaultImageNameTag), shellEscape(defaultContextPath))
		simulatedError := errors.New("build failed")
		simulatedStdout := "Step 1/2 : FROM alpine\n ---> abcdef123456"
		simulatedStderr := "Error: The command '/bin/sh -c unknown-command' returned a non-zero code: 127"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(simulatedStdout), []byte(simulatedStderr), simulatedError).Once()

		err := r.BuildImage(ctx, mockConn, defaultDockerfilePath, defaultImageNameTag, defaultContextPath, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build image")
		assert.Contains(t, err.Error(), simulatedError.Error())
		assert.Contains(t, err.Error(), "Stdout: "+simulatedStdout)
		assert.Contains(t, err.Error(), "Stderr: "+simulatedStderr)
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

} // End of TestBuildImage


// TestCreateContainer contains specific unit tests for the CreateContainer method.
func TestCreateContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	defaultImage := "alpine:latest"
	defaultContainerID := "a1b2c3d4e5f6"

	baseOptions := ContainerCreateOptions{ImageName: defaultImage}

	t.Run("Success_Minimal", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		opts := baseOptions
		expectedCmd := fmt.Sprintf("docker create %s", shellEscape(defaultImage))

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(execOpts *connector.ExecOptions) bool {
			return execOpts.Sudo && execOpts.Timeout == 1*time.Minute
		})).Return([]byte(defaultContainerID+"\n"), nil, nil).Once()

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
				{HostPort: "8080", ContainerPort: "80"},
				{HostIP: "127.0.0.1", HostPort: "9090", ContainerPort: "90", Protocol: "udp"},
			},
		}
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "docker create") &&
				strings.Contains(cmd, "--name 'my-container'") &&
				strings.Contains(cmd, "-p '8080:80'") &&
				strings.Contains(cmd, "-p '127.0.0.1:9090:90/udp'") &&
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
				{Source: "/host/path", Destination: "/container/path"},
				{Source: "named-volume", Destination: "/data", Mode: "ro"},
			},
			EnvVars: []string{"FOO=bar", "BAZ=qux"},
		}

		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "docker create") &&
				strings.Contains(cmd, "-v '/host/path:/container/path'") &&
				strings.Contains(cmd, "-v 'named-volume:/data:ro'") &&
				strings.Contains(cmd, "-e 'FOO=bar'") &&
				strings.Contains(cmd, "-e 'BAZ=qux'") &&
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
			Entrypoint: []string{"/app/custom-entry"},
			Command:    []string{"arg1", "val with space"},
		}
		expectedCmd := fmt.Sprintf("docker create --entrypoint '/app/custom-entry' %s 'arg1' 'val with space'", shellEscape(defaultImage))
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
				strings.Contains(cmd, "--restart 'on-failure:3'") &&
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

} // End TestCreateContainer


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
	// assertNotImplementedBool is removed as ContainerExists now has its own Test function
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
	// PullImage, ImageExists, ListImages, RemoveImage, BuildImage, CreateContainer, ContainerExists
	// now have dedicated TestXxx functions.
	// Keep stubs here ONLY for methods not yet having their own TestXxx function.
	// t.Run("ContainerExists", ...) was removed as it has its own TestContainerExists

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
} // End of TestDockerMethodStubs_Remaining
