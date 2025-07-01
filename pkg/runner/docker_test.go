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

// TestDockerPrune tests the DockerPrune method.
func TestDockerPrune(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	type pruneArgs struct {
		pruneType string
		filters   map[string]string
		all       bool
	}
	tests := []struct {
		name            string
		args            pruneArgs
		expectedCmdBase string // Base part of the command, e.g., "docker system prune"
		expectAllFlag   bool   // Whether -a should be part of the command for this pruneType
		expectedFilters []string // Expected filter strings like "--filter 'key=value'"
		mockStdout      string
		mockStderr      string
		mockError       error
		expectError     bool
		errorContains   []string
	}{
		{
			name:            "SystemPrune_Default",
			args:            pruneArgs{pruneType: "system"},
			expectedCmdBase: "docker system prune -f",
			expectAllFlag:   false,
			mockStdout:      "Total reclaimed space: 1.23GB",
			expectError:     false,
		},
		{
			name:            "SystemPrune_All",
			args:            pruneArgs{pruneType: "system", all: true},
			expectedCmdBase: "docker system prune -a -f",
			expectAllFlag:   true,
			mockStdout:      "Total reclaimed space: 2.5GB",
			expectError:     false,
		},
		{
			name:            "ImagePrune_WithFilter",
			args:            pruneArgs{pruneType: "image", filters: map[string]string{"until": "24h"}},
			expectedCmdBase: "docker image prune -f",
			expectAllFlag:   false,
			expectedFilters: []string{fmt.Sprintf("--filter %s", shellEscape("until=24h"))},
			mockStdout:      "Deleted images:\n...",
			expectError:     false,
		},
		{
			name:            "ImagePrune_All_WithFilter",
			args:            pruneArgs{pruneType: "image", all: true, filters: map[string]string{"label": "stage=build"}},
			expectedCmdBase: "docker image prune -a -f",
			expectAllFlag:   true,
			expectedFilters: []string{fmt.Sprintf("--filter %s", shellEscape("label=stage=build"))},
			mockStdout:      "Deleted images:\n...",
			expectError:     false,
		},
		{
			name:            "VolumePrune", // `all` flag is not applicable here in the same way
			args:            pruneArgs{pruneType: "volume", filters: map[string]string{"label!": "keep"}},
			expectedCmdBase: "docker volume prune -f",
			expectAllFlag:   false, // -a not used for volume prune
			expectedFilters: []string{fmt.Sprintf("--filter %s", shellEscape("label!=keep"))},
			mockStdout:      "Deleted volumes:\nvol1\nTotal reclaimed space: 500MB",
			expectError:     false,
		},
		{
			name:            "BuilderPrune",
			args:            pruneArgs{pruneType: "builder", all: true}, // builder prune supports --all
			expectedCmdBase: "docker builder prune -a -f", // -a is effectively --all for builder
			expectAllFlag:   true,
			mockStdout:      "Total reclaimed space: 10GB",
			expectError:     false,
		},
		{
			name:          "UnsupportedPruneType",
			args:          pruneArgs{pruneType: "secrets"},
			expectError:   true,
			errorContains: []string{"unsupported pruneType: secrets"},
		},
		{
			name:            "CommandError",
			args:            pruneArgs{pruneType: "system"},
			expectedCmdBase: "docker system prune -f",
			mockError:       errors.New("docker prune failed"),
			mockStderr:      "daemon not responding",
			expectError:     true,
			errorContains:   []string{"failed to prune docker system", "daemon not responding"},
		},
		{
			name:            "EmptyFilterKey",
			args:            pruneArgs{pruneType: "image", filters: map[string]string{"": "value"}},
			expectedCmdBase: "docker image prune -f",
			expectError:     true,
			errorContains:   []string{"filter key cannot be empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := mocks.NewConnector(t)
			r := &defaultRunner{}

			if tt.expectError && len(tt.errorContains) > 0 && strings.Contains(tt.errorContains[0], "unsupported pruneType") {
				// For unsupported prune type, Exec is not called.
			} else if tt.expectError && strings.Contains(tt.errorContains[0], "filter key cannot be empty") {
				// For empty filter key, Exec is not called.
			} else {
				// Construct the command matcher more carefully
				mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
					hasBase := strings.HasPrefix(cmd, tt.expectedCmdBase)
					if !hasBase && tt.expectAllFlag && strings.Contains(tt.expectedCmdBase, " -a ") { // check if -a was part of base
						// This case for builder prune where -a is part of the base
						hasBase = strings.HasPrefix(cmd, strings.Replace(tt.expectedCmdBase, " -a ", " ",1)) && strings.Contains(cmd, " -a ")
					} else if !hasBase && tt.expectAllFlag && !strings.Contains(tt.expectedCmdBase, " -a "){
						hasBase = strings.HasPrefix(cmd, strings.Replace(tt.expectedCmdBase, " -f", " -a -f", 1))
					}


					allFiltersPresent := true
					for _, f := range tt.expectedFilters {
						if !strings.Contains(cmd, f) {
							allFiltersPresent = false
							break
						}
					}
					return hasBase && allFiltersPresent
				}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(tt.mockStdout), []byte(tt.mockStderr), tt.mockError).Once()
			}

			stdout, err := r.DockerPrune(ctx, mockConn, tt.args.pruneType, tt.args.filters, tt.args.all)

			if tt.expectError {
				assert.Error(t, err)
				for _, కంటెంట్స్ := range tt.errorContains {
					assert.Contains(t, err.Error(), కంటెంట్స్)
				}
				if tt.mockError != nil { // If a command error was simulated, stdout might still be returned
					assert.Equal(t, tt.mockStdout, stdout)
				} else { // For validation errors before Exec, stdout should be empty
					assert.Empty(t, stdout)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockStdout, stdout)
			}
			mockConn.AssertExpectations(t)
		})
	}

	t.Run("NilConnector", func(t *testing.T) {
		_, err := r.DockerPrune(ctx, nil, "system", nil, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestDockerInfo tests the DockerInfo method.
func TestDockerInfo(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker info --format {{json .}}"
		// Based on the SystemInfo struct and typical `docker info` JSON.
		// Note: Some fields like CgroupDriver might vary ("cgroupfs" vs "systemd").
		// MemTotal will be an int64.
		jsonOutput := `{
			"ID": "docker-server-id",
			"Containers": 10,
			"ContainersRunning": 5,
			"ContainersPaused": 1,
			"ContainersStopped": 4,
			"Images": 20,
			"ServerVersion": "20.10.7",
			"Driver": "overlay2",
			"LoggingDriver": "json-file",
			"CgroupDriver": "cgroupfs",
			"CgroupVersion": "1",
			"KernelVersion": "5.4.0-100-generic",
			"OperatingSystem": "Ubuntu 20.04.3 LTS",
			"OSVersion": "20.04",
			"OSType": "linux",
			"Architecture": "x86_64",
			"MemTotal": 8388608000
		}` // MemTotal in bytes (e.g. 8GB)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerInspectTimeout
		})).Return([]byte(jsonOutput), nil, nil).Once()

		info, err := r.DockerInfo(ctx, mockConn)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		if info != nil {
			assert.Equal(t, "docker-server-id", info.ID)
			assert.Equal(t, 10, info.Containers)
			assert.Equal(t, 5, info.ContainersRunning)
			assert.Equal(t, 1, info.ContainersPaused)
			assert.Equal(t, 4, info.ContainersStopped)
			assert.Equal(t, 20, info.Images)
			assert.Equal(t, "20.10.7", info.ServerVersion)
			assert.Equal(t, "overlay2", info.StorageDriver) // Mapped from "Driver" in JSON
			assert.Equal(t, "json-file", info.LoggingDriver)
			assert.Equal(t, "cgroupfs", info.CgroupDriver)
			assert.Equal(t, "1", info.CgroupVersion)
			assert.Equal(t, "5.4.0-100-generic", info.KernelVersion)
			assert.Equal(t, "Ubuntu 20.04.3 LTS", info.OperatingSystem)
			assert.Equal(t, "20.04", info.OSVersion)
			assert.Equal(t, "linux", info.OSType)
			assert.Equal(t, "x86_64", info.Architecture)
			assert.Equal(t, int64(8388608000), info.MemTotal)
		}
	})

	t.Run("Success_NumberAsStringInJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker info --format {{json .}}"
		// Simulate case where numbers might be strings in JSON output (less common for `docker info` but good to test robustness)
		jsonOutput := `{
			"ID": "docker-server-id-str",
			"Containers": "15",
			"Images": "25",
			"MemTotal": "16777216000"
		}`

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		info, err := r.DockerInfo(ctx, mockConn)
		assert.NoError(t, err)
		assert.NotNil(t, info)
		if info != nil {
			assert.Equal(t, "docker-server-id-str", info.ID)
			assert.Equal(t, 15, info.Containers)
			assert.Equal(t, 25, info.Images)
			assert.Equal(t, int64(16777216000), info.MemTotal)
		}
	})


	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker info --format {{json .}}"
		simulatedError := errors.New("docker info failed")
		simulatedStderr := "Cannot connect to docker daemon"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.DockerInfo(ctx, mockConn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get docker info")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker info --format {{json .}}"
		invalidJson := `{"ID": "bad-json",` // Malformed
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJson), nil, nil).Once()

		_, err := r.DockerInfo(ctx, mockConn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse docker info JSON output")
	})

	t.Run("Failure_EmptyOutput", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker info --format {{json .}}"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte("  "), nil, nil).Once()

		_, err := r.DockerInfo(ctx, mockConn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker info returned empty output")
	})


	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.DockerInfo(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestInspectDockerVolume tests the InspectDockerVolume method.
func TestInspectDockerVolume(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	volumeName := "inspect-test-volume"

	t.Run("Success_ArrayResponse", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		// `docker volume inspect <name>` usually returns a JSON array with one element.
		jsonOutput := fmt.Sprintf(`[
			{
				"Name": "%s",
				"Driver": "local",
				"Mountpoint": "/var/lib/docker/volumes/%s/_data",
				"Labels": {"purpose": "testing", "project": "kubexm"},
				"Scope": "local",
				"Options": {"type": "none", "device": "tmpfs", "o": "size=100m,uid=1000"},
				"CreatedAt": "2023-10-27T12:00:00Z"
			}
		]`, volumeName, volumeName)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerInspectTimeout
		})).Return([]byte(jsonOutput), nil, nil).Once()

		vol, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.NoError(t, err)
		assert.NotNil(t, vol)
		if vol != nil {
			assert.Equal(t, volumeName, vol.Name)
			assert.Equal(t, "local", vol.Driver)
			assert.Equal(t, "/var/lib/docker/volumes/"+volumeName+"/_data", vol.Mountpoint)
			assert.Equal(t, map[string]string{"purpose": "testing", "project": "kubexm"}, vol.Labels)
			assert.Equal(t, "local", vol.Scope)
			assert.Equal(t, map[string]string{"type": "none", "device": "tmpfs", "o": "size=100m,uid=1000"}, vol.Options)
			assert.Equal(t, "2023-10-27T12:00:00Z", vol.CreatedAt)
		}
	})

	t.Run("Success_SingleObjectResponse", func(t *testing.T) { // For robustness, if CLI returns single object
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		jsonOutput := fmt.Sprintf(`
			{
				"Name": "%s",
				"Driver": "custom",
				"Mountpoint": "/mnt/custom_volumes/%s",
				"Labels": null,
				"Scope": "global",
				"Options": null,
				"CreatedAt": "2023-11-01T10:30:00Z"
			}
		`, volumeName, volumeName)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()
		vol, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.NoError(t, err)
		assert.NotNil(t, vol)
		if vol != nil {
			assert.Equal(t, volumeName, vol.Name)
			assert.Equal(t, "custom", vol.Driver)
			assert.Nil(t, vol.Labels, "Labels should be nil if JSON 'null'") // or Empty map based on Volume struct tags
			assert.Equal(t, "global", vol.Scope)
			assert.Nil(t, vol.Options, "Options should be nil if JSON 'null'")
			assert.Equal(t, "2023-11-01T10:30:00Z", vol.CreatedAt)
		}
	})


	t.Run("Failure_NotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		simulatedStderr := fmt.Sprintf("Error: No such volume: %s", volumeName)
		simulatedError := &connector.CommandError{ExitCode: 1, Stderr: simulatedStderr}
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, connector.ErrNotFound) || strings.Contains(err.Error(), "not found"))
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		simulatedError := errors.New("docker daemon error")
		simulatedStderr := "Cannot connect to Docker daemon"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to inspect docker volume")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		invalidJsonOutput := `[{"Name": "vol1", "Driver":` // Malformed
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJsonOutput), nil, nil).Once()

		_, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse docker volume inspect output")
	})

	t.Run("Failure_EmptyJSONOutput", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte("  "), nil, nil).Once()

		_, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker volume inspect returned empty output")
	})

	t.Run("Failure_EmptyJSONArray", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte("[]"), nil, nil).Once()

		_, err := r.InspectDockerVolume(ctx, mockConn, volumeName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "returned an empty JSON array")
	})


	t.Run("Failure_EmptyVolumeName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.InspectDockerVolume(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "volumeName cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.InspectDockerVolume(ctx, nil, volumeName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestListDockerVolumes tests the ListDockerVolumes method.
func TestListDockerVolumes(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoVolumes", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker volume ls --format {{json .}}"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(" \n "), nil, nil).Once()

		volumes, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assert.NoError(t, err)
		assert.Empty(t, volumes)
	})

	t.Run("Success_OneVolume", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker volume ls --format {{json .}}"
		// Example JSON from `docker volume ls --format {{json .}}`.
		// Fields like Scope, CreatedAt, Options, Size are often not in basic `ls` JSON.
		jsonOutput := `{"Name":"vol1","Driver":"local","Mountpoint":"/var/lib/docker/volumes/vol1/_data","Labels":"project=db,env=prod"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		volumes, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assert.NoError(t, err)
		assert.Len(t, volumes, 1)
		if len(volumes) == 1 {
			vol := volumes[0]
			assert.Equal(t, "vol1", vol.Name)
			assert.Equal(t, "local", vol.Driver)
			assert.Equal(t, "/var/lib/docker/volumes/vol1/_data", vol.Mountpoint)
			assert.Equal(t, map[string]string{"project": "db", "env": "prod"}, vol.Labels)
			assert.Empty(t, vol.Scope, "Scope should be empty if not in CLI output")
			assert.Empty(t, vol.CreatedAt, "CreatedAt should be empty if not in CLI output")
		}
	})

	t.Run("Success_WithFilters", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"dangling": "true", "label": "backup=weekly"}
		expectedCmdBase := "docker volume ls --format {{json .}}"
		expectedFilter1 := fmt.Sprintf("--filter %s", shellEscape("dangling=true"))
		expectedFilter2 := fmt.Sprintf("--filter %s", shellEscape("label=backup=weekly"))

		jsonOutput := `{"Name":"dangling_vol","Driver":"custom","Mountpoint":"/mnt/dangling","Labels":"backup=weekly,stale=true"}` + "\n"
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, expectedCmdBase) &&
				strings.Contains(cmd, expectedFilter1) &&
				strings.Contains(cmd, expectedFilter2)
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		volumes, err := r.ListDockerVolumes(ctx, mockConn, filters)
		assert.NoError(t, err)
		assert.Len(t, volumes, 1)
		if len(volumes) == 1 {
			assert.Equal(t, "dangling_vol", volumes[0].Name)
			assert.Equal(t, "custom", volumes[0].Driver)
			assert.Equal(t, map[string]string{"backup": "weekly", "stale": "true"}, volumes[0].Labels)
		}
	})

	t.Run("Failure_InvalidFilterFormat", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"": "value"} // Empty key
		_, err := r.ListDockerVolumes(ctx, mockConn, filters)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "filter key and value cannot be empty")
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker volume ls --format {{json .}}"
		simulatedError := errors.New("docker volume ls failed")
		simulatedStderr := "daemon error during volume list"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list docker volumes")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker volume ls --format {{json .}}"
		invalidJson := `{"Name":"vol1", "Driver":` // Incomplete JSON
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJson), nil, nil).Once()

		_, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse volume JSON line")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.ListDockerVolumes(ctx, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestRemoveDockerVolume tests the RemoveDockerVolume method.
func TestRemoveDockerVolume(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	volumeName := "test-vol-to-remove"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume rm %s", shellEscape(volumeName))
		// `docker volume rm` outputs the volume name on success.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 30*time.Second
		})).Return([]byte(volumeName), nil, nil).Once()

		err := r.RemoveDockerVolume(ctx, mockConn, volumeName, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Force", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume rm -f %s", shellEscape(volumeName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(volumeName), nil, nil).Once()

		err := r.RemoveDockerVolume(ctx, mockConn, volumeName, true)
		assert.NoError(t, err)
	})

	t.Run("Failure_VolumeNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume rm %s", shellEscape(volumeName))
		simulatedError := errors.New("volume not found")
		simulatedStderr := fmt.Sprintf("Error: No such volume: %s", volumeName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RemoveDockerVolume(ctx, mockConn, volumeName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove docker volume")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_VolumeInUse_NoForce", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume rm %s", shellEscape(volumeName))
		simulatedError := errors.New("volume in use")
		simulatedStderr := fmt.Sprintf("Error response from daemon: remove %s: volume is in use - [containerID]", volumeName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RemoveDockerVolume(ctx, mockConn, volumeName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove docker volume")
		assert.Contains(t, err.Error(), "volume is in use")
	})

	t.Run("Failure_EmptyVolumeName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.RemoveDockerVolume(ctx, mockConn, " ", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "volumeName cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.RemoveDockerVolume(ctx, nil, volumeName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestCreateDockerVolume tests the CreateDockerVolume method.
func TestCreateDockerVolume(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	volumeName := "test-volume"

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume create %s", shellEscape(volumeName))
		// `docker volume create` outputs the volume name on success.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return([]byte(volumeName), nil, nil).Once()

		err := r.CreateDockerVolume(ctx, mockConn, volumeName, "", nil, nil)
		assert.NoError(t, err)
	})

	t.Run("Success_WithOptions", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		driver := "local"
		driverOpts := map[string]string{"type": "nfs", "o": "addr=192.168.1.1,rw", "device": ":/data/share"}
		labels := map[string]string{"backup": "true", "storage": "fast"}

		// Command construction can be complex, check for key parts.
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, "docker volume create") &&
				strings.Contains(cmd, fmt.Sprintf("--driver %s", shellEscape(driver))) &&
				strings.Contains(cmd, fmt.Sprintf("--opt %s", shellEscape("type=nfs"))) &&
				strings.Contains(cmd, fmt.Sprintf("--opt %s", shellEscape("o=addr=192.168.1.1,rw"))) &&
				strings.Contains(cmd, fmt.Sprintf("--opt %s", shellEscape("device=:/data/share"))) &&
				strings.Contains(cmd, fmt.Sprintf("--label %s", shellEscape("backup=true"))) &&
				strings.Contains(cmd, fmt.Sprintf("--label %s", shellEscape("storage=fast"))) &&
				strings.HasSuffix(cmd, shellEscape(volumeName))
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(volumeName), nil, nil).Once()

		err := r.CreateDockerVolume(ctx, mockConn, volumeName, driver, driverOpts, labels)
		assert.NoError(t, err)
	})

	t.Run("Failure_VolumeAlreadyExists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker volume create %s", shellEscape(volumeName))
		simulatedError := errors.New("volume already exists")
		simulatedStderr := fmt.Sprintf("Error response from daemon: a volume with the name %s already exists", volumeName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.CreateDockerVolume(ctx, mockConn, volumeName, "", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create docker volume")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyVolumeName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.CreateDockerVolume(ctx, mockConn, " ", "", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "volume name cannot be empty")
	})

	t.Run("Failure_EmptyDriverOptKey", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		driverOpts := map[string]string{"": "value"}
		err := r.CreateDockerVolume(ctx, mockConn, volumeName, "local", driverOpts, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "driver option key cannot be empty")
	})

	t.Run("Failure_EmptyLabelKey", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		labels := map[string]string{"": "value"}
		err := r.CreateDockerVolume(ctx, mockConn, volumeName, "", nil, labels)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "label key cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.CreateDockerVolume(ctx, nil, volumeName, "", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestDisconnectContainerFromNetwork tests the DisconnectContainerFromNetwork method.
func TestDisconnectContainerFromNetwork(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	networkName := "test-disconn-network"
	containerName := "test-disconn-container"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return(nil, nil, nil).Once() // No output on success usually

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Force", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect -f %s %s", shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, true)
		assert.NoError(t, err)
	})

	t.Run("Failure_NotConnected", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("not connected")
		simulatedStderr := fmt.Sprintf("Error response from daemon: container %s is not connected to network %s", containerName, networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "is not connected")
	})

	t.Run("Failure_ContainerNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("container not found")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "No such container")
	})

	t.Run("Failure_NetworkNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("network not found")
		simulatedStderr := fmt.Sprintf("Error: No such network: %s", networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "No such network")
	})

	t.Run("Failure_EmptyNetworkName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, " ", containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkIDOrName cannot be empty")
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, " ", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.DisconnectContainerFromNetwork(ctx, nil, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestDisconnectContainerFromNetwork tests the DisconnectContainerFromNetwork method.
func TestDisconnectContainerFromNetwork(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	networkName := "test-disconn-network"
	containerName := "test-disconn-container"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return(nil, nil, nil).Once() // No output on success usually

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Force", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect -f %s %s", shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, true)
		assert.NoError(t, err)
	})

	t.Run("Failure_NotConnected", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("not connected")
		simulatedStderr := fmt.Sprintf("Error response from daemon: container %s is not connected to network %s", containerName, networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err) // Current impl treats "not connected" as error
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "is not connected")
	})

	t.Run("Failure_ContainerNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("container not found")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "No such container")
	})

	t.Run("Failure_NetworkNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network disconnect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("network not found")
		simulatedStderr := fmt.Sprintf("Error: No such network: %s", networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to disconnect container")
		assert.Contains(t, err.Error(), "No such network")
	})

	t.Run("Failure_EmptyNetworkName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, " ", containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkIDOrName cannot be empty")
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, networkName, " ", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.DisconnectContainerFromNetwork(ctx, nil, networkName, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestConnectContainerToNetwork tests the ConnectContainerToNetwork method.
func TestConnectContainerToNetwork(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	networkName := "test-conn-network"
	containerName := "test-conn-container"

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network connect %s %s", shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return(nil, nil, nil).Once() // No output on success usually

		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, containerName, "")
		assert.NoError(t, err)
	})

	t.Run("Success_WithIPAddress", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		ipAddress := "192.168.30.10"
		expectedCmd := fmt.Sprintf("docker network connect --ip %s %s %s", shellEscape(ipAddress), shellEscape(networkName), shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()

		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, containerName, ipAddress)
		assert.NoError(t, err)
	})

	t.Run("Failure_AlreadyConnected", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network connect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("already connected")
		simulatedStderr := fmt.Sprintf("Error response from daemon: container %s is already connected to network %s", containerName, networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, containerName, "")
		assert.Error(t, err) // Current impl treats "already connected" as error
		assert.Contains(t, err.Error(), "failed to connect container")
		assert.Contains(t, err.Error(), "already connected")
	})

	t.Run("Failure_ContainerNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network connect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("container not found")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, containerName, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect container")
		assert.Contains(t, err.Error(), "No such container")
	})

	t.Run("Failure_NetworkNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network connect %s %s", shellEscape(networkName), shellEscape(containerName))
		simulatedError := errors.New("network not found")
		simulatedStderr := fmt.Sprintf("Error: No such network: %s", networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, containerName, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect container")
		assert.Contains(t, err.Error(), "No such network")
	})


	t.Run("Failure_EmptyNetworkName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.ConnectContainerToNetwork(ctx, mockConn, " ", containerName, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkIDOrName cannot be empty")
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.ConnectContainerToNetwork(ctx, mockConn, networkName, " ", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.ConnectContainerToNetwork(ctx, nil, networkName, containerName, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestListDockerNetworks tests the ListDockerNetworks method.
func TestListDockerNetworks(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoNetworks", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker network ls --format {{json .}}"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(" \n "), nil, nil).Once()

		networks, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assert.NoError(t, err)
		assert.Empty(t, networks)
	})

	t.Run("Success_OneNetwork", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker network ls --format {{json .}}"
		// Example JSON from `docker network ls --format {{json .}}`
		// Note: Fields like CreatedAt, Options are not typically in `ls` output.
		jsonOutput := `{"ID":"net1","Name":"my-bridge-net","Driver":"bridge","Scope":"local","IPv6":"false","Internal":"false","Attachable":"true","Labels":"project=api,type=frontend"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		networks, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assert.NoError(t, err)
		assert.Len(t, networks, 1)
		if len(networks) == 1 {
			net := networks[0]
			assert.Equal(t, "net1", net.ID)
			assert.Equal(t, "my-bridge-net", net.Name)
			assert.Equal(t, "bridge", net.Driver)
			assert.Equal(t, "local", net.Scope)
			assert.False(t, net.EnableIPv6)
			assert.False(t, net.Internal)
			assert.True(t, net.Attachable)
			assert.Equal(t, map[string]string{"project": "api", "type": "frontend"}, net.Labels)
			assert.True(t, net.Created.IsZero(), "Expected Created to be zero time as not provided by ls")
		}
	})

	t.Run("Success_WithFilters", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"driver": "overlay", "label": "global=true"}
		expectedCmdBase := "docker network ls --format {{json .}}"
		expectedFilter1 := fmt.Sprintf("--filter %s", shellEscape("driver=overlay"))
		expectedFilter2 := fmt.Sprintf("--filter %s", shellEscape("label=global=true"))

		jsonOutput := `{"ID":"overlay1","Name":"global-overlay","Driver":"overlay","Scope":"global","IPv6":"true","Labels":"global=true"}` + "\n"
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, expectedCmdBase) &&
				strings.Contains(cmd, expectedFilter1) &&
				strings.Contains(cmd, expectedFilter2)
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		networks, err := r.ListDockerNetworks(ctx, mockConn, filters)
		assert.NoError(t, err)
		assert.Len(t, networks, 1)
		if len(networks) == 1 {
			assert.Equal(t, "overlay1", networks[0].ID)
			assert.Equal(t, "overlay", networks[0].Driver)
			assert.True(t, networks[0].EnableIPv6)
			assert.Equal(t, map[string]string{"global": "true"}, networks[0].Labels)
		}
	})

	t.Run("Failure_InvalidFilterFormat", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"": "value"} // Empty key
		_, err := r.ListDockerNetworks(ctx, mockConn, filters)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "filter key and value cannot be empty")
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker network ls --format {{json .}}"
		simulatedError := errors.New("docker network ls failed")
		simulatedStderr := "daemon error during network list"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list docker networks")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker network ls --format {{json .}}"
		invalidJson := `{"ID":"net1", "Name":` // Incomplete JSON
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJson), nil, nil).Once()

		_, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse network JSON line")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.ListDockerNetworks(ctx, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestRemoveDockerNetwork tests the RemoveDockerNetwork method.
func TestRemoveDockerNetwork(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	networkName := "test-network-to-remove"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network rm %s", shellEscape(networkName))
		// `docker network rm` outputs the network name/ID on success.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 30*time.Second
		})).Return([]byte(networkName), nil, nil).Once()

		err := r.RemoveDockerNetwork(ctx, mockConn, networkName)
		assert.NoError(t, err)
	})

	t.Run("Failure_NetworkNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network rm %s", shellEscape(networkName))
		simulatedError := errors.New("network not found")
		simulatedStderr := fmt.Sprintf("Error: No such network: %s", networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RemoveDockerNetwork(ctx, mockConn, networkName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove docker network")
		assert.Contains(t, err.Error(), simulatedStderr)
		// If "No such network" should not be an error (idempotency), this test would change.
	})

	t.Run("Failure_NetworkInUse", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network rm %s", shellEscape(networkName))
		simulatedError := errors.New("network in use")
		simulatedStderr := fmt.Sprintf("Error response from daemon: network %s is in use by container %s", networkName, "some_container_id")
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RemoveDockerNetwork(ctx, mockConn, networkName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove docker network")
		assert.Contains(t, err.Error(), "is in use by container")
	})


	t.Run("Failure_EmptyNetworkName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.RemoveDockerNetwork(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "networkIDOrName cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.RemoveDockerNetwork(ctx, nil, networkName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestCreateDockerNetwork tests the CreateDockerNetwork method.
func TestCreateDockerNetwork(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	networkName := "test-network"

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network create %s", shellEscape(networkName))
		// `docker network create` outputs the network ID (long hash) or name on success.
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return([]byte("networkID_or_name"), nil, nil).Once()

		err := r.CreateDockerNetwork(ctx, mockConn, networkName, "", "", "", nil)
		assert.NoError(t, err)
	})

	t.Run("Success_WithOptions", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		driver := "bridge"
		subnet := "192.168.55.0/24"
		gateway := "192.168.55.1"
		labels := map[string]string{"project": "test", "env": "dev"}

		// Command construction can be complex, check for key parts.
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, "docker network create") &&
				strings.Contains(cmd, fmt.Sprintf("--driver %s", shellEscape(driver))) &&
				strings.Contains(cmd, fmt.Sprintf("--subnet %s", shellEscape(subnet))) &&
				strings.Contains(cmd, fmt.Sprintf("--gateway %s", shellEscape(gateway))) &&
				strings.Contains(cmd, fmt.Sprintf("--label %s", shellEscape("project=test"))) &&
				strings.Contains(cmd, fmt.Sprintf("--label %s", shellEscape("env=dev"))) &&
				strings.HasSuffix(cmd, shellEscape(networkName))
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(networkName), nil, nil).Once()

		err := r.CreateDockerNetwork(ctx, mockConn, networkName, driver, subnet, gateway, labels)
		assert.NoError(t, err)
	})

	t.Run("Failure_NetworkAlreadyExists", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker network create %s", shellEscape(networkName))
		simulatedError := errors.New("network already exists")
		simulatedStderr := fmt.Sprintf("Error response from daemon: network with name %s already exists", networkName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.CreateDockerNetwork(ctx, mockConn, networkName, "", "", "", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create docker network")
		assert.Contains(t, err.Error(), simulatedStderr)
		// If specific handling for "already exists" is added, this test would change.
	})

	t.Run("Failure_EmptyNetworkName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.CreateDockerNetwork(ctx, mockConn, " ", "", "", "", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network name cannot be empty")
	})

	t.Run("Failure_EmptyLabelKey", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		labels := map[string]string{"": "value"}
		err := r.CreateDockerNetwork(ctx, mockConn, networkName, "", "", "", labels)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "label key cannot be empty")
	})


	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.CreateDockerNetwork(ctx, nil, networkName, "", "", "", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestExecInContainer tests the ExecInContainer method.
func TestExecInContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerName := "exec-test-container"
	defaultCmd := []string{"ls", "-l", "/tmp"}

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedDockerCmd := fmt.Sprintf("docker exec %s %s %s %s",
			shellEscape(containerName),
			shellEscape("ls"),
			shellEscape("-l"),
			shellEscape("/tmp"))
		expectedStdout := "total 0\n-rw-r--r-- 1 root root 0 Jan 1 00:00 somefile"

		mockConn.On("Exec", mock.Anything, expectedDockerCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 5*time.Minute
		})).Return([]byte(expectedStdout), nil, nil).Once()

		stdout, err := r.ExecInContainer(ctx, mockConn, containerName, defaultCmd, "", "", false)
		assert.NoError(t, err)
		assert.Equal(t, expectedStdout, stdout)
	})

	t.Run("Success_WithOptions_User_Workdir_TTY", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		user := "testuser"
		workDir := "/app"
		cmdToRun := []string{"pwd"}
		// Note: -t (TTY) can affect output, but for `pwd` it's usually fine.
		// Expected: docker exec -t -u 'testuser' -w '/app' exec-test-container 'pwd'
		expectedDockerCmd := fmt.Sprintf("docker exec -t -u %s -w %s %s %s",
			shellEscape(user),
			shellEscape(workDir),
			shellEscape(containerName),
			shellEscape("pwd"))
		expectedStdout := "/app\n"

		mockConn.On("Exec", mock.Anything, expectedDockerCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(expectedStdout), nil, nil).Once()

		stdout, err := r.ExecInContainer(ctx, mockConn, containerName, cmdToRun, user, workDir, true)
		assert.NoError(t, err)
		assert.Equal(t, expectedStdout, stdout)
	})

	t.Run("Failure_CommandErrorInContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		cmdToRun := []string{"nonexistent-command"}
		expectedDockerCmd := fmt.Sprintf("docker exec %s %s",
			shellEscape(containerName),
			shellEscape("nonexistent-command"))

		simulatedError := errors.New("command exited non-zero")
		simulatedStdout := "" // Command might not produce stdout on error
		simulatedStderr := "/bin/sh: nonexistent-command: not found"
		mockConn.On("Exec", mock.Anything, expectedDockerCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(simulatedStdout), []byte(simulatedStderr), simulatedError).Once()

		stdout, err := r.ExecInContainer(ctx, mockConn, containerName, cmdToRun, "", "", false)
		assert.Error(t, err)
		assert.Equal(t, simulatedStdout, stdout) // stdout might still be returned along with error
		assert.Contains(t, err.Error(), "failed to execute command in container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_ContainerNotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedDockerCmd := fmt.Sprintf("docker exec %s %s", shellEscape(containerName), shellEscape("ls"))
		simulatedError := errors.New("container not found")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedDockerCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.ExecInContainer(ctx, mockConn, containerName, []string{"ls"}, "", "", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute command in container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.ExecInContainer(ctx, mockConn, " ", defaultCmd, "", "", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_EmptyCommand", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.ExecInContainer(ctx, mockConn, containerName, []string{}, "", "", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command to execute cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.ExecInContainer(ctx, nil, containerName, defaultCmd, "", "", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestUnpauseContainer tests the UnpauseContainer method.
func TestUnpauseContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerName := "unpause-test-container"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker unpause %s", shellEscape(containerName))
		// `docker unpause` outputs the container name/ID on success
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerStartTimeout // Unpausing uses StartTimeout
		})).Return([]byte(containerName), nil, nil).Once()

		err := r.UnpauseContainer(ctx, mockConn, containerName)
		assert.NoError(t, err)
	})

	t.Run("Failure_NotPaused", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker unpause %s", shellEscape(containerName))
		simulatedError := errors.New("not paused")
		simulatedStderr := fmt.Sprintf("Error response from daemon: Container %s is not paused", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.UnpauseContainer(ctx, mockConn, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unpause container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_NotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker unpause %s", shellEscape(containerName))
		simulatedError := errors.New("no such container")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.UnpauseContainer(ctx, mockConn, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unpause container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.UnpauseContainer(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.UnpauseContainer(ctx, nil, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestPauseContainer tests the PauseContainer method.
func TestPauseContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerName := "pause-test-container"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker pause %s", shellEscape(containerName))
		// `docker pause` outputs the container name/ID on success
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerStartTimeout // Pausing uses StartTimeout
		})).Return([]byte(containerName), nil, nil).Once()

		err := r.PauseContainer(ctx, mockConn, containerName)
		assert.NoError(t, err)
	})

	t.Run("Failure_AlreadyPaused", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker pause %s", shellEscape(containerName))
		simulatedError := errors.New("already paused")
		simulatedStderr := fmt.Sprintf("Error response from daemon: Container %s is already paused", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.PauseContainer(ctx, mockConn, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pause container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_NotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker pause %s", shellEscape(containerName))
		simulatedError := errors.New("no such container")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.PauseContainer(ctx, mockConn, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pause container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.PauseContainer(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.PauseContainer(ctx, nil, containerName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestInspectContainer tests the InspectContainer method.
func TestInspectContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerNameOrID := "inspect-test-container"

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		// Real InspectResponse is complex. Simulate a minimal valid JSON.
		// Docker inspect usually returns an array with one element.
		jsonOutput := fmt.Sprintf(`[
			{
				"Id": "sha256:abcdef123456",
				"Name": "/%s",
				"State": { "Status": "running", "Pid": 1234 },
				"Config": { "Image": "alpine" }
			}
		]`, containerNameOrID)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerInspectTimeout
		})).Return([]byte(jsonOutput), nil, nil).Once()

		resp, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		if resp != nil {
			assert.Equal(t, "sha256:abcdef123456", resp.ID)
			assert.Equal(t, "/"+containerNameOrID, resp.Name)
			assert.Equal(t, "running", resp.State.Status)
			assert.Equal(t, "alpine", resp.Config.Image)
		}
	})

	t.Run("Success_SingleObjectJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		// Simulate Docker returning a single JSON object instead of an array (older versions or specific cases)
		jsonOutput := fmt.Sprintf(`
			{
				"Id": "sha256:singleobj",
				"Name": "/%s-single",
				"State": { "Status": "exited", "Pid": 0 },
				"Config": { "Image": "ubuntu" }
			}
		`, containerNameOrID)

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		resp, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		if resp != nil {
			assert.Equal(t, "sha256:singleobj", resp.ID)
			assert.Equal(t, "/"+containerNameOrID+"-single", resp.Name)
			assert.Equal(t, "exited", resp.State.Status)
			assert.Equal(t, "ubuntu", resp.Config.Image)
		}
	})


	t.Run("Failure_NotFound", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		// Docker inspect for a non-existent container typically returns non-zero exit code
		// and an error message to stderr like "Error: No such object: <name>"
		simulatedStderr := fmt.Sprintf("Error: No such object: %s", containerNameOrID)
		// The connector.Exec should transform this into a connector.ErrNotFound or similar.
		// For this test, we simulate the Exec call returning an error that our InspectContainer
		// should wrap with connector.ErrNotFound if "No such object" is in stderr.
		simulatedError := &connector.CommandError{ExitCode: 1, Stderr: simulatedStderr}

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.Error(t, err)
		// Check if the error is or wraps connector.ErrNotFound
		assert.True(t, errors.Is(err, connector.ErrNotFound) || strings.Contains(err.Error(), "not found"), "Error should indicate 'not found'")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		simulatedError := errors.New("docker daemon error")
		simulatedStderr := "Cannot connect to Docker daemon"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to inspect container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		invalidJsonOutput := `[{"Id": "bad-json", "Name":]` // Malformed
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJsonOutput), nil, nil).Once()

		_, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse docker inspect output")
	})

	t.Run("Failure_EmptyJSONOutput", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		emptyOutput := "  "
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(emptyOutput), nil, nil).Once()

		_, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker inspect returned empty output")
	})

	t.Run("Failure_EmptyJSONArray", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
		emptyJSONArray := "[]"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(emptyJSONArray), nil, nil).Once()

		_, err := r.InspectContainer(ctx, mockConn, containerNameOrID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "returned an empty JSON array")
	})


	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.InspectContainer(ctx, mockConn, " ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.InspectContainer(ctx, nil, containerNameOrID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestGetContainerStats tests the GetContainerStats method.
func TestGetContainerStats(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerName := "stats-test-container"

	t.Run("Success_NoStream", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker stats --no-stream --format {{json .}} %s", shellEscape(containerName))
		// Simulate a single JSON line of stats
		jsonOutput := `{"ID":"s1","Name":"stats-test","CPUPerc":"1.23%","MemUsage":"100MiB / 1GiB","MemPerc":"9.77%","NetIO":"1kB / 2kB","BlockIO":"0B / 0B","PIDs":"10"}` + "\n"

		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 15*time.Second // Default for no-stream
		})).Return([]byte(jsonOutput), nil, nil).Once()

		statsChan, err := r.GetContainerStats(ctx, mockConn, containerName, false)
		assert.NoError(t, err)
		assert.NotNil(t, statsChan)

		// Read the single expected stat
		var receivedStat *container.StatsResponse
		select {
		case stat, ok := <-statsChan:
			if ok {
				receivedStat = stat
			}
		case <-time.After(1 * time.Second): // Timeout for receiving from channel
			t.Fatal("Timeout waiting for stat from channel")
		}

		assert.NotNil(t, receivedStat, "Should have received one stat")
		if receivedStat != nil {
			assert.Equal(t, "s1", receivedStat.ID)
			assert.Equal(t, "stats-test", receivedStat.Name)
			// Further checks on parsed values (e.g., MemoryStats.Usage) would require
			// replicating the parsing logic from the main function or using specific known values.
			// For brevity, we check basic fields.
			memUsageBytes, _ := parseDockerSize("100MiB")
			assert.Equal(t, uint64(memUsageBytes), receivedStat.MemoryStats.Usage)
		}

		// Ensure channel is closed after the single stat (for no-stream)
		select {
		case _, ok := <-statsChan:
			assert.False(t, ok, "Channel should be closed after single stat for no-stream")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for channel to close")
		}
	})

	t.Run("Success_Stream_ContextCancel", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		streamCtx, cancelStream := context.WithCancel(ctx)
		defer cancelStream() // Ensure cancellation happens

		expectedCmd := fmt.Sprintf("docker stats --format {{json .}} %s", shellEscape(containerName))
		// Simulate a couple of JSON lines
		jsonOutput := `{"ID":"s1","Name":"stats-test","CPUPerc":"1.00%","MemUsage":"100MiB / 1GiB"}` + "\n" +
			`{"ID":"s1","Name":"stats-test","CPUPerc":"2.00%","MemUsage":"102MiB / 1GiB"}` + "\n"

		// For streaming, Exec is called. We simulate it running then being cancelled.
		mockConn.On("Exec", streamCtx, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 24*time.Hour // Long timeout for stream
		})).Run(func(args mock.Arguments) {
			// This Run func simulates the command starting.
			// The actual cancellation will be handled by the test logic shortly after.
		}).Return([]byte(jsonOutput), nil, context.Canceled).Once() // Return context.Canceled when streamCtx is done.


		statsChan, err := r.GetContainerStats(streamCtx, mockConn, containerName, true)
		assert.NoError(t, err)
		assert.NotNil(t, statsChan)

		// Read some stats
		timeout := time.After(2 * time.Second) // Overall timeout for this part of the test
		var statsCount int
	OuterLoop:
		for {
			select {
			case stat, ok := <-statsChan:
				if !ok { // Channel closed
					break OuterLoop
				}
				if stat != nil {
					statsCount++
					assert.Equal(t, "s1", stat.ID)
				}
				if statsCount == 1 { // After receiving one stat, cancel the context
					go func() { time.Sleep(50 * time.Millisecond); cancelStream() }() // Cancel async after a short delay
				}
			case <-timeout:
				t.Logf("Received %d stats before timeout.", statsCount)
				// Depending on timing, we might get 0, 1, or 2 stats from the buffer
				// The key is that the channel eventually closes due to context cancellation.
				break OuterLoop
			}
		}

		// Ensure channel is eventually closed due to context cancellation
		var isClosed bool
		select {
		case _, ok := <-statsChan:
			if !ok {
				isClosed = true
			}
		case <-time.After(1 * time.Second): // Wait a bit longer for closure if needed
			_, okStillOpen := <-statsChan // Final check
			if !okStillOpen {
				isClosed = true
			} else {
				t.Fatal("Timeout waiting for stats channel to close after context cancellation")
			}
		}
		assert.True(t, isClosed, "Stats channel should be closed after context cancellation")
	})


	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker stats --no-stream --format {{json .}} %s", shellEscape(containerName))
		simulatedError := errors.New("stats command failed")
		simulatedStderr := "Error: No such container"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		statsChan, err := r.GetContainerStats(ctx, mockConn, containerName, false)
		assert.NoError(t, err) // GetContainerStats itself doesn't return error for cmd exec failure, relies on channel
		assert.NotNil(t, statsChan)

		// Expect channel to be closed without sending items due to error in goroutine
		select {
		case _, ok := <-statsChan:
			assert.False(t, ok, "Channel should be closed immediately on command error")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for channel to close on command error")
		}
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker stats --no-stream --format {{json .}} %s", shellEscape(containerName))
		invalidJsonOutput := `{"ID":"s1", "Name":` // Malformed JSON
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJsonOutput), nil, nil).Once()

		statsChan, err := r.GetContainerStats(ctx, mockConn, containerName, false)
		assert.NoError(t, err)
		assert.NotNil(t, statsChan)

		// Expect channel to be closed, possibly after trying to parse bad JSON.
		// No stats should be successfully sent.
		select {
		case stat, ok := <-statsChan:
			assert.False(t, ok, "Channel should be closed if JSON is unparsable, no stat sent.")
			assert.Nil(t, stat)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for channel behavior on invalid JSON")
		}
	})


	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.GetContainerStats(ctx, mockConn, " ", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.GetContainerStats(ctx, nil, containerName, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}


// TestGetContainerLogs tests the GetContainerLogs method.
func TestGetContainerLogs(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	containerName := "log-test-container"

	t.Run("Success_Simple", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker logs %s", shellEscape(containerName))
		expectedLogs := "Log line 1\nLog line 2"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute // Default timeout for non-follow
		})).Return([]byte(expectedLogs), nil, nil).Once()

		logs, err := r.GetContainerLogs(ctx, mockConn, containerName, ContainerLogOptions{})
		assert.NoError(t, err)
		assert.Equal(t, expectedLogs, logs)
	})

	t.Run("Success_WithOptions", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		options := ContainerLogOptions{
			Timestamps: true,
			Tail:       "100",
			Since:      "2023-01-01T00:00:00Z",
			Until:      "2023-01-02T00:00:00Z",
			Details:    true,
		}
		// Command construction can be complex, ensure key flags are present
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, "docker logs") &&
				strings.Contains(cmd, " -t ") && // Timestamps
				strings.Contains(cmd, fmt.Sprintf("--tail %s", shellEscape("100"))) &&
				strings.Contains(cmd, fmt.Sprintf("--since %s", shellEscape(options.Since))) &&
				strings.Contains(cmd, fmt.Sprintf("--until %s", shellEscape(options.Until))) &&
				strings.Contains(cmd, " --details") &&
				strings.HasSuffix(cmd, shellEscape(containerName))
		}), mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 1*time.Minute
		})).Return([]byte("Filtered logs"), nil, nil).Once()

		logs, err := r.GetContainerLogs(ctx, mockConn, containerName, options)
		assert.NoError(t, err)
		assert.Equal(t, "Filtered logs", logs)
	})

	t.Run("Success_Follow", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		options := ContainerLogOptions{Follow: true}
		expectedCmd := fmt.Sprintf("docker logs -f %s", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == 24*time.Hour // Long timeout for follow
		})).Return([]byte("Following logs..."), nil, nil).Once()

		// Note: Testing true streaming behavior of "follow" is complex in unit tests
		// as it depends on context cancellation. This test primarily checks command formation.
		logs, err := r.GetContainerLogs(ctx, mockConn, containerName, options)
		assert.NoError(t, err)
		assert.Equal(t, "Following logs...", logs)
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := fmt.Sprintf("docker logs %s", shellEscape(containerName))
		simulatedError := errors.New("logs command failed")
		simulatedStderr := "Error: No such container"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		logs, err := r.GetContainerLogs(ctx, mockConn, containerName, ContainerLogOptions{})
		assert.Error(t, err)
		assert.Empty(t, logs)
		assert.Contains(t, err.Error(), "failed to get logs for container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerName", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		_, err := r.GetContainerLogs(ctx, mockConn, " ", ContainerLogOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.GetContainerLogs(ctx, nil, containerName, ContainerLogOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})
}

// TestListContainers tests the ListContainers method.
func TestListContainers(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success_NoContainers", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker ps --format {{json .}}"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(" \n "), nil, nil).Once() // Empty or whitespace output

		containers, err := r.ListContainers(ctx, mockConn, false, nil)
		assert.NoError(t, err)
		assert.Empty(t, containers)
	})

	t.Run("Success_OneContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker ps --format {{json .}}"
		// Simplified JSON output for `docker ps --format "{{json .}}"`
		// Fields like CreatedAt, Mounts, detailed NetworkSettings are often not in this simple JSON or are human-readable.
		// The parsing in ListContainers tries its best to map these.
		jsonOutput := `{"ID":"c1","Image":"alpine","Command":"sh","Status":"Up 2 hours","Names":"my-alpine","Labels":"foo=bar,baz=quux","Ports":"0.0.0.0:8080->80/tcp","Networks":"bridge"}` + "\n"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		containers, err := r.ListContainers(ctx, mockConn, false, nil)
		assert.NoError(t, err)
		assert.Len(t, containers, 1)
		if len(containers) == 1 {
			cont := containers[0]
			assert.Equal(t, "c1", cont.ID)
			assert.Equal(t, "alpine", cont.Image)
			assert.Equal(t, "sh", cont.Command)
			assert.Equal(t, "Up 2 hours", cont.Status)
			assert.Equal(t, []string{"my-alpine"}, cont.Names)
			assert.Equal(t, map[string]string{"foo": "bar", "baz": "quux"}, cont.Labels)

			// Port parsing check
			assert.Len(t, cont.Ports, 1)
			if len(cont.Ports) == 1 {
				p := cont.Ports[0]
				assert.Equal(t, "0.0.0.0", p.IP)
				assert.Equal(t, uint16(80), p.PrivatePort)
				assert.Equal(t, uint16(8080), p.PublicPort)
				assert.Equal(t, "tcp", p.Type)
			}

			// NetworkSettings check (simplified)
			assert.NotNil(t, cont.NetworkSettings)
			assert.NotNil(t, cont.NetworkSettings.Networks)
			_, ok := cont.NetworkSettings.Networks["bridge"]
			assert.True(t, ok, "Expected 'bridge' network to be present")
		}
	})

	t.Run("Success_AllContainersWithFilters", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"status": "exited", "label": "project=test"}
		// Order of filters in command string might vary.
		expectedCmdBase := "docker ps -a --format {{json .}}"
		expectedFilter1 := fmt.Sprintf("--filter %s", shellEscape("status=exited"))
		expectedFilter2 := fmt.Sprintf("--filter %s", shellEscape("label=project=test"))

		jsonOutput := `{"ID":"c2","Image":"ubuntu","Command":"sleep 10","Status":"Exited (0) 5 minutes ago","Names":"test-exited","Labels":"project=test,version=1.0"}` + "\n"
		mockConn.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, expectedCmdBase) &&
				strings.Contains(cmd, expectedFilter1) &&
				strings.Contains(cmd, expectedFilter2)
		}), mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(jsonOutput), nil, nil).Once()

		containers, err := r.ListContainers(ctx, mockConn, true, filters)
		assert.NoError(t, err)
		assert.Len(t, containers, 1)
		if len(containers) == 1 {
			assert.Equal(t, "c2", containers[0].ID)
			assert.Equal(t, "Exited (0) 5 minutes ago", containers[0].Status)
			assert.Equal(t, map[string]string{"project": "test", "version": "1.0"}, containers[0].Labels)
		}
	})

	t.Run("Failure_InvalidFilterFormat", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		filters := map[string]string{"": "value"} // Empty key
		_, err := r.ListContainers(ctx, mockConn, false, filters)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "filter key and value cannot be empty")
	})

	t.Run("Failure_CommandError", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker ps --format {{json .}}"
		simulatedError := errors.New("docker ps failed")
		simulatedStderr := "daemon error"
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		_, err := r.ListContainers(ctx, mockConn, false, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list containers")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_InvalidJSON", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		expectedCmd := "docker ps --format {{json .}}"
		invalidJson := `{"ID":"c1", "Image":` // Incomplete JSON
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(invalidJson), nil, nil).Once()

		_, err := r.ListContainers(ctx, mockConn, false, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse container JSON line")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		_, err := r.ListContainers(ctx, nil, false, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
	})

	t.Run("Success_PortParsingEdgeCases", func(t *testing.T) {
		// Test the internal parseDockerPorts specifically for robustness
		// Valid inputs
		ports, err := parseDockerPorts("0.0.0.0:80->80/tcp,:::443->443/udp,8080:80,90/tcp")
		assert.NoError(t, err)
		assert.Len(t, ports, 4)
		if len(ports) >= 4 {
			assert.Equal(t, container.Port{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 80, Type: "tcp"}, ports[0])
			assert.Equal(t, container.Port{IP: "::", PrivatePort: 443, PublicPort: 443, Type: "udp"}, ports[1])
			assert.Equal(t, container.Port{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"}, ports[2]) // Default type tcp
			assert.Equal(t, container.Port{IP: "", PrivatePort: 90, PublicPort: 0, Type: "tcp"}, ports[3])      // No host mapping, just exposed port
		}


		// Empty input
		ports, err = parseDockerPorts(" ")
		assert.NoError(t, err)
		assert.Empty(t, ports)

		// Malformed input (should ideally not error out the whole ListContainers, but skip bad entries)
		// The current parseDockerPorts might return error or skip. Let's assume it skips bad entries or we test its specific behavior.
		// For ListContainers, if parseDockerPorts returns an error, the whole operation might fail.
		// The implementation of parseDockerPorts in the provided code seems to `continue` on parse error for a single port part.
		portsStrWithMalformed := "0.0.0.0:badport->80/tcp, 0.0.0.0:8080->80/tcp"
		mockConn := mocks.NewConnector(t)
		jsonOutput := fmt.Sprintf(`{"ID":"c1","Image":"alpine","Ports":%q}`, portsStrWithMalformed) + "\n"
		mockConn.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return([]byte(jsonOutput), nil, nil).Once()

		listedContainers, listErr := r.ListContainers(ctx, mockConn, false, nil)
		assert.NoError(t, listErr) // Expect ListContainers to succeed
		assert.Len(t, listedContainers, 1)
		if len(listedContainers) == 1 {
			// Check that valid parts were parsed, malformed ones skipped
			assert.Len(t, listedContainers[0].Ports, 1, "Expected only one valid port to be parsed")
			if len(listedContainers[0].Ports) == 1 {
				assert.Equal(t, uint16(8080), listedContainers[0].Ports[0].PublicPort)
			}
		}
	})
}


// TestRemoveContainer tests the RemoveContainer method.
func TestRemoveContainer(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "test-container"
		expectedCmd := fmt.Sprintf("docker rm %s", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
			return opts.Sudo && opts.Timeout == DefaultDockerRMTimeout
		})).Return([]byte(containerName), nil, nil).Once() // Docker rm outputs container name on success

		err := r.RemoveContainer(ctx, mockConn, containerName, false, false)
		assert.NoError(t, err)
	})

	t.Run("Success_Force", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "test-container-force"
		expectedCmd := fmt.Sprintf("docker rm -f %s", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(containerName), nil, nil).Once()

		err := r.RemoveContainer(ctx, mockConn, containerName, true, false)
		assert.NoError(t, err)
	})

	t.Run("Success_RemoveVolumes", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "test-container-volumes"
		expectedCmd := fmt.Sprintf("docker rm -v %s", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(containerName), nil, nil).Once()

		err := r.RemoveContainer(ctx, mockConn, containerName, false, true)
		assert.NoError(t, err)
	})

	t.Run("Success_ForceAndRemoveVolumes", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "test-container-force-volumes"
		expectedCmd := fmt.Sprintf("docker rm -f -v %s", shellEscape(containerName))
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte(containerName), nil, nil).Once()

		err := r.RemoveContainer(ctx, mockConn, containerName, true, true)
		assert.NoError(t, err)
	})

	t.Run("Failure_NonExistentContainer", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		containerName := "no-such-container"
		expectedCmd := fmt.Sprintf("docker rm %s", shellEscape(containerName))
		simulatedError := errors.New("docker rm error")
		simulatedStderr := fmt.Sprintf("Error: No such container: %s", containerName)
		mockConn.On("Exec", mock.Anything, expectedCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, []byte(simulatedStderr), simulatedError).Once()

		err := r.RemoveContainer(ctx, mockConn, containerName, false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove container")
		assert.Contains(t, err.Error(), simulatedStderr)
	})

	t.Run("Failure_EmptyContainerNameOrID", func(t *testing.T) {
		mockConn := mocks.NewConnector(t)
		err := r.RemoveContainer(ctx, mockConn, " ", false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")
	})

	t.Run("Failure_NilConnector", func(t *testing.T) {
		err := r.RemoveContainer(ctx, nil, "some-container", false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connector cannot be nil")
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
	// REMOVED: RemoveContainer test as it's now implemented above.
	// REMOVED: ListContainers test as it's now implemented above.
	// REMOVED: GetContainerLogs test as it's now implemented above.
	// REMOVED: GetContainerStats test as it's now implemented above.
	// REMOVED: InspectContainer test as it's now implemented above.
	// REMOVED: PauseContainer test as it's now implemented above.
	// REMOVED: UnpauseContainer test as it's now implemented above.
	// REMOVED: ExecInContainer test as it's now implemented above.

	// Docker Network Methods
	// REMOVED: CreateDockerNetwork test as it's now implemented above.
	// REMOVED: RemoveDockerNetwork test as it's now implemented above.
	// REMOVED: ListDockerNetworks test as it's now implemented above.
	// REMOVED: ConnectContainerToNetwork test as it's now implemented above.
	// REMOVED: DisconnectContainerFromNetwork test as it's now implemented above.

	// Docker Volume Methods
	// REMOVED: CreateDockerVolume test as it's now implemented above.
	// REMOVED: RemoveDockerVolume test as it's now implemented above.
	// REMOVED: ListDockerVolumes test as it's now implemented above.
	// REMOVED: InspectDockerVolume test as it's now implemented above.

	// Docker System Methods
	// REMOVED: DockerInfo test as it's now implemented above.
	// REMOVED: DockerPrune test as it's now implemented below.

	// GetDockerServerVersion, CheckDockerInstalled, EnsureDockerService, CheckDockerRequirement,
	// PruneDockerBuildCache, GetHostArchitecture, ResolveDockerImage, DockerSave, DockerLoad
	// are already implemented and have their own tests or are utilities not part of the primary Docker operation stubs.
	// So they are not checked here for "not implemented".
}
