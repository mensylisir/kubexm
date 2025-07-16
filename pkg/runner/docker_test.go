package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	//lint:ignore SA1019 we need to use this for now
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func TestDefaultRunner_PullImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "alpine:latest"

	// Test case 1: Successful pull
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pull %s", shellEscape(imageName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.PullImage(ctx, mockConn, imageName)
	assert.NoError(t, err)

	// Test case 2: Docker command execution fails
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pull %s", shellEscape(imageName)), gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	err = runner.PullImage(ctx, mockConn, imageName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")

	// Test case 3: Empty image name
	err = runner.PullImage(ctx, mockConn, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "imageName cannot be empty")

	// Test case 4: Nil connector
	err = runner.PullImage(ctx, nil, imageName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

func TestDefaultRunner_ImageExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "nginx:latest"
	cmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))

	// Test case 1: Image exists
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.ImageExists(ctx, mockConn, imageName)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test case 2: Image does not exist
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("Error: No such image: nginx:latest"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.ImageExists(ctx, mockConn, imageName)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test case 3: Docker command execution fails with unexpected error
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	exists, err = runner.ImageExists(ctx, mockConn, imageName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "exec error")
}

func TestDefaultRunner_ListImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	img1JSON := `{"ID":"imgid1","Repository":"repo1","Tag":"latest","Size":"100MB", "CreatedAt": "2023-01-01 10:00:00 +0000 UTC"}`
	img2JSON := `{"ID":"imgid2","Repository":"repo2","Tag":"v1.0","Size":"2.5GB", "CreatedAt": "2023-02-01 10:00:00 +0000 UTC"}`

	expectedImg1 := image.Summary{
		ID:       "imgid1",
		RepoTags: []string{"repo1:latest"},
		Size:     100 * 1024 * 1024, VirtualSize: 100 * 1024 * 1024,
		Created: time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC).Unix(),
	}
	expectedImg2 := image.Summary{
		ID:       "imgid2",
		RepoTags: []string{"repo2:v1.0"},
		Size:     int64(2.5 * 1024 * 1024 * 1024), VirtualSize: int64(2.5 * 1024 * 1024 * 1024),
		Created: time.Date(2023, 2, 1, 10, 0, 0, 0, time.UTC).Unix(),
	}

	mockConn.EXPECT().Exec(ctx, "docker images --format {{json .}}", gomock.Any()).
		Return([]byte(img1JSON+"\n"+img2JSON), []byte{}, nil).Times(1)
	images, err := runner.ListImages(ctx, mockConn, false)
	assert.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Contains(t, images, expectedImg1)
	assert.Contains(t, images, expectedImg2)

	mockConn.EXPECT().Exec(ctx, "docker images --all --format {{json .}}", gomock.Any()).
		Return([]byte(img1JSON), []byte{}, nil).Times(1)
	images, err = runner.ListImages(ctx, mockConn, true)
	assert.NoError(t, err)
	assert.Len(t, images, 1)
	assert.Equal(t, expectedImg1, images[0])

	mockConn.EXPECT().Exec(ctx, "docker images --format {{json .}}", gomock.Any()).
		Return(nil, []byte("docker error"), fmt.Errorf("exec error")).Times(1)
	images, err = runner.ListImages(ctx, mockConn, false)
	assert.Error(t, err)
	assert.Nil(t, images)

	mockConn.EXPECT().Exec(ctx, "docker images --format {{json .}}", gomock.Any()).
		Return([]byte("this is not json"), []byte{}, nil).Times(1)
	images, err = runner.ListImages(ctx, mockConn, false)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "failed to parse image JSON line")

	invalidSizeJSON := `{"ID":"imgid3","Repository":"repo3","Tag":"badsize","Size":"1XXMB"}`
	mockConn.EXPECT().Exec(ctx, "docker images --format {{json .}}", gomock.Any()).
		Return([]byte(invalidSizeJSON), []byte{}, nil).Times(1)
	images, err = runner.ListImages(ctx, mockConn, false)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "failed to parse size")
}

func TestDefaultRunner_RemoveImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "busybox:latest"

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rmi %s", shellEscape(imageName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.RemoveImage(ctx, mockConn, imageName, false)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rmi -f %s", shellEscape(imageName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.RemoveImage(ctx, mockConn, imageName, true)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rmi %s", shellEscape(imageName)), gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	err = runner.RemoveImage(ctx, mockConn, imageName, false)
	assert.Error(t, err)
}

func TestDefaultRunner_BuildImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	dockerfilePath := "Dockerfile.test"
	imageNameAndTag := "myimage:test"
	contextPath := "."
	buildArgs := map[string]string{"VERSION": "1.0", "EMPTY_ARG": ""}

	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
			assert.Contains(t, cmd, fmt.Sprintf("docker build -f %s", shellEscape(dockerfilePath)))
			assert.Contains(t, cmd, fmt.Sprintf("-t %s", shellEscape(imageNameAndTag)))
			assert.Contains(t, cmd, fmt.Sprintf("--build-arg %s", shellEscape("VERSION=1.0")))
			assert.Contains(t, cmd, fmt.Sprintf("--build-arg %s", shellEscape("EMPTY_ARG=")))
			assert.Contains(t, cmd, shellEscape(contextPath))
			return nil, []byte{}, nil
		}).Times(1)
	err := runner.BuildImage(ctx, mockConn, dockerfilePath, imageNameAndTag, contextPath, buildArgs)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).Return(nil, []byte("build error"), fmt.Errorf("exec error")).Times(1)
	err = runner.BuildImage(ctx, mockConn, dockerfilePath, imageNameAndTag, contextPath, buildArgs)
	assert.Error(t, err)
}

func TestDefaultRunner_CreateContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	opts := ContainerCreateOptions{
		ImageName:     "alpine",
		ContainerName: "test-alpine",
		Ports:         []ContainerPortMapping{{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		Volumes:       []ContainerMount{{Source: "/tmp", Destination: "/data", Mode: "ro"}},
		EnvVars:       []string{"MODE=test", "DEBUG="},
		Entrypoint:    []string{"/bin/sh"},
		Command:       []string{"-c", "echo hello"},
		RestartPolicy: "on-failure:3",
		Privileged:    true,
		AutoRemove:    true,
	}
	expectedContainerID := "newcontainerid123"

	mockConn.EXPECT().Exec(ctx, gomock.All(
		gomock.Cond(func(x interface{}) bool { return strings.HasPrefix(x.(string), "docker create") }),
		gomock.Cond(func(x interface{}) bool { return strings.Contains(x.(string), shellEscape(opts.ImageName)) }),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("--name %s", shellEscape(opts.ContainerName)))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("-p %s", shellEscape("8080:80/tcp")))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("-v %s", shellEscape("/tmp:/data:ro")))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("-e %s", shellEscape("MODE=test")))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("-e %s", shellEscape("DEBUG=")))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("--entrypoint %s", shellEscape("/bin/sh")))
		}),
		gomock.Cond(func(x interface{}) bool {
			return strings.Contains(x.(string), fmt.Sprintf("--restart %s", shellEscape("on-failure:3")))
		}),
		gomock.Cond(func(x interface{}) bool { return strings.Contains(x.(string), "--privileged") }),
		gomock.Cond(func(x interface{}) bool { return strings.Contains(x.(string), "--rm") }),
		gomock.Cond(func(x interface{}) bool {
			return strings.HasSuffix(x.(string), fmt.Sprintf("%s %s", shellEscape("-c"), shellEscape("echo hello")))
		}),
	), gomock.Any()).Return([]byte(expectedContainerID), []byte{}, nil).Times(1)

	id, err := runner.CreateContainer(ctx, mockConn, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedContainerID, id)

	mockConn.EXPECT().Exec(ctx, gomock.Any(gomock.Cond(func(x interface{}) bool { return strings.HasPrefix(x.(string), "docker create") })), gomock.Any()).
		Return(nil, []byte("create error"), fmt.Errorf("exec error")).Times(1)
	id, err = runner.CreateContainer(ctx, mockConn, opts)
	assert.Error(t, err)
	assert.Empty(t, id)
}

func TestDefaultRunner_ContainerExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerNameOrID := "my-container"
	cmd := fmt.Sprintf("docker inspect %s > /dev/null 2>&1", shellEscape(containerNameOrID))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.ContainerExists(ctx, mockConn, containerNameOrID)
	assert.NoError(t, err)
	assert.True(t, exists)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("Error: No such object"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.ContainerExists(ctx, mockConn, containerNameOrID)
	assert.NoError(t, err)
	assert.False(t, exists)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	exists, err = runner.ContainerExists(ctx, mockConn, containerNameOrID)
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestDefaultRunner_StartContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerNameOrID := "sleepy-container"
	cmd := fmt.Sprintf("docker start %s", shellEscape(containerNameOrID))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(containerNameOrID), []byte{}, nil).Times(1)
	err := runner.StartContainer(ctx, mockConn, containerNameOrID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("Error: No such container"), fmt.Errorf("exec error")).Times(1)
	err = runner.StartContainer(ctx, mockConn, containerNameOrID)
	assert.Error(t, err)
}

func TestDefaultRunner_StopContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerNameOrID := "running-container"

	cmdDefault := fmt.Sprintf("docker stop %s", shellEscape(containerNameOrID))
	mockConn.EXPECT().Exec(ctx, cmdDefault, gomock.Any()).Return([]byte(containerNameOrID), []byte{}, nil).Times(1)
	err := runner.StopContainer(ctx, mockConn, containerNameOrID, nil)
	assert.NoError(t, err)

	timeout := 5 * time.Second
	cmdWithTimeout := fmt.Sprintf("docker stop -t 5 %s", shellEscape(containerNameOrID))
	mockConn.EXPECT().Exec(ctx, cmdWithTimeout, gomock.Any()).Return([]byte(containerNameOrID), []byte{}, nil).Times(1)
	err = runner.StopContainer(ctx, mockConn, containerNameOrID, &timeout)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmdDefault, gomock.Any()).Return(nil, []byte("Error: No such container"), fmt.Errorf("exec error")).Times(1)
	err = runner.StopContainer(ctx, mockConn, containerNameOrID, nil)
	assert.Error(t, err)
}

func TestDefaultRunner_RestartContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerNameOrID := "my-app-container"

	cmdDefault := fmt.Sprintf("docker restart %s", shellEscape(containerNameOrID))
	mockConn.EXPECT().Exec(ctx, cmdDefault, gomock.Any()).Return([]byte(containerNameOrID), []byte{}, nil).Times(1)
	err := runner.RestartContainer(ctx, mockConn, containerNameOrID, nil)
	assert.NoError(t, err)

	timeout := 3 * time.Second
	cmdWithTimeout := fmt.Sprintf("docker restart -t 3 %s", shellEscape(containerNameOrID))
	mockConn.EXPECT().Exec(ctx, cmdWithTimeout, gomock.Any()).Return([]byte(containerNameOrID), []byte{}, nil).Times(1)
	err = runner.RestartContainer(ctx, mockConn, containerNameOrID, &timeout)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmdDefault, gomock.Any()).Return(nil, []byte("Error: No such container"), fmt.Errorf("exec error")).Times(1)
	err = runner.RestartContainer(ctx, mockConn, containerNameOrID, nil)
	assert.Error(t, err)
}

func TestDefaultRunner_RemoveContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	containerID := "test-container"

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err := runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, false)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f -v %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, true)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("Error: No such container: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, false)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("Error: No such container: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such container")

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")

	err = runner.RemoveContainer(ctx, mockConn, "", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")

	err = runner.RemoveContainer(ctx, nil, containerID, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

func TestDefaultRunner_ListContainers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	sampleContainer1JSON := `{"ID":"id1","Image":"image1","Command":"cmd1","CreatedAt":"2023-01-01 10:00:00 +0000 UTC","Names":"name1","Labels":"k1=v1,k2=v2","Mounts":"/mnt1,/mnt2","Networks":"net1","Ports":"80/tcp","Status":"Up 2 hours"}`
	sampleContainer2JSON := `{"ID":"id2","Image":"image2","Command":"cmd2","CreatedAt":"2023-01-02 12:00:00 +0000 UTC","Names":"name2,altName2","Labels":"k3=v3","Mounts":"","Networks":"net2","Ports":"0.0.0.0:8080->80/tcp","Status":"Exited (0) 1 day ago"}`

	expectedContainer1 := ContainerInfo{
		ID: "id1", Image: "image1", Command: "cmd1",
		Created: time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC).Unix(),
		Names:   []string{"name1"}, Labels: map[string]string{"k1": "v1", "k2": "v2"},
		Mounts: []ContainerMount{{Source: "/mnt1"}, {Source: "/mnt2"}},
		Ports:  []ContainerPortMapping{{ContainerPort: "80", Protocol: "tcp"}},
		State:  "running", Status: "Up 2 hours",
	}
	expectedContainer2 := ContainerInfo{
		ID: "id2", Image: "image2", Command: "cmd2",
		Created: time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).Unix(),
		Names:   []string{"name2", "altName2"}, Labels: map[string]string{"k3": "v3"},
		Ports: []ContainerPortMapping{{HostIP: "0.0.0.0", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		State: "exited", Status: "Exited (0) 1 day ago",
	}

	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return([]byte(sampleContainer1JSON+"\n"+sampleContainer2JSON), []byte{}, nil).Times(1)
	containers, err := runner.ListContainers(ctx, mockConn, false, nil)
	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	assert.Equal(t, expectedContainer1, containers[0])
	assert.Equal(t, expectedContainer2, containers[1])

	filters := map[string]string{"status": "running"}
	expectedCmdWithFilters := "docker ps --all --filter 'status=running' --format {{json .}}"
	mockConn.EXPECT().Exec(ctx, expectedCmdWithFilters, gomock.Any()).
		Return([]byte(sampleContainer1JSON), []byte{}, nil).Times(1)
	containers, err = runner.ListContainers(ctx, mockConn, true, filters)
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, expectedContainer1, containers[0])

	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return(nil, []byte("docker error"), fmt.Errorf("exec error")).Times(1)
	containers, err = runner.ListContainers(ctx, mockConn, false, nil)
	assert.Error(t, err)
	assert.Nil(t, containers)

	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return([]byte("this is not json"), []byte{}, nil).Times(1)
	containers, err = runner.ListContainers(ctx, mockConn, false, nil)
	assert.Error(t, err)
	assert.Nil(t, containers)

	_, err = runner.ListContainers(ctx, mockConn, false, map[string]string{"": "value"})
	assert.Error(t, err)

	_, err = runner.ListContainers(ctx, nil, false, nil)
	assert.Error(t, err)
}

func TestDefaultRunner_GetContainerLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-log-container"

	logOutput := "Log line 1\nLog line 2"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker logs %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte(logOutput), []byte{}, nil).Times(1)
	logs, err := runner.GetContainerLogs(ctx, mockConn, containerID, ContainerLogOptions{})
	assert.NoError(t, err)
	assert.Equal(t, logOutput, logs)

	opts := ContainerLogOptions{
		Timestamps: true, Tail: "100", Since: "2023-01-01T00:00:00Z",
	}
	expectedCmdWithOpts := fmt.Sprintf("docker logs --timestamps --since %s --tail '100' %s", shellEscape(opts.Since), shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, expectedCmdWithOpts, gomock.Any()).
		Return([]byte(logOutput), []byte{}, nil).Times(1)
	logs, err = runner.GetContainerLogs(ctx, mockConn, containerID, opts)
	assert.NoError(t, err)
	assert.Equal(t, logOutput, logs)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker logs %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("docker error"), fmt.Errorf("exec error")).Times(1)
	logs, err = runner.GetContainerLogs(ctx, mockConn, containerID, ContainerLogOptions{})
	assert.Error(t, err)
	assert.Empty(t, logs)

	_, err = runner.GetContainerLogs(ctx, mockConn, "", ContainerLogOptions{})
	assert.Error(t, err)

	_, err = runner.GetContainerLogs(ctx, nil, containerID, ContainerLogOptions{})
	assert.Error(t, err)
}

func TestDefaultRunner_InspectContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-inspect-container"

	sampleInspectJSON := `[{"Id": "abcdef123456", "Name": "/trusting_archimedes"}]`
	var expectedDetailsArray []ContainerDetails
	errUn := json.Unmarshal([]byte(sampleInspectJSON), &expectedDetailsArray)
	assert.NoError(t, errUn)
	expectedDetails := expectedDetailsArray[0]

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte(sampleInspectJSON), []byte{}, nil).Times(1)
	details, err := runner.InspectContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)
	assert.NotNil(t, details)
	assert.Equal(t, expectedDetails.ID, details.ID)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("Error: No such object: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)
	assert.Nil(t, details)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("some other docker error"), fmt.Errorf("exec error")).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Nil(t, details)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte("this is not json"), []byte{}, nil).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Nil(t, details)
}

func TestDefaultRunner_PauseUnpauseContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-pause-container"

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.PauseContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("pause error"), fmt.Errorf("exec pause error")).Times(1)
	err = runner.PauseContainer(ctx, mockConn, containerID)
	assert.Error(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker unpause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.UnpauseContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker unpause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("unpause error"), fmt.Errorf("exec unpause error")).Times(1)
	err = runner.UnpauseContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
}

func TestDefaultRunner_ExecInContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-exec-container"
	cmdToExec := []string{"ls", "-l", "/tmp"}

	expectedCmdStr := fmt.Sprintf("docker exec %s %s %s %s", shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]), shellEscape(cmdToExec[2]))
	mockConn.EXPECT().Exec(ctx, expectedCmdStr, gomock.Any()).Return([]byte("stdout content"), []byte("stderr content"), nil).Times(1)
	output, err := runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, "", "", false)
	assert.NoError(t, err)
	assert.Equal(t, "stdout content"+"stderr content", output)

	user := "testuser"
	workDir := "/app"
	expectedCmdWithUserWorkdirTTY := fmt.Sprintf("docker exec -t --user %s --workdir %s %s %s %s %s", shellEscape(user), shellEscape(workDir), shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]), shellEscape(cmdToExec[2]))
	mockConn.EXPECT().Exec(ctx, expectedCmdWithUserWorkdirTTY, gomock.Any()).Return([]byte("tty output"), []byte{}, nil).Times(1)
	output, err = runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, user, workDir, true)
	assert.NoError(t, err)
	assert.Equal(t, "tty output", output)

	mockConn.EXPECT().Exec(ctx, expectedCmdStr, gomock.Any()).
		Return([]byte("stdout on fail"), []byte("stderr on fail"), &connector.CommandError{ExitCode: 127}).Times(1)
	output, err = runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, "", "", false)
	assert.Error(t, err)
	assert.Contains(t, output, "stdout on fail")
	assert.Contains(t, output, "stderr on fail")

	_, err = runner.ExecInContainer(ctx, mockConn, containerID, []string{}, "", "", false)
	assert.Error(t, err)
}

func TestDefaultRunner_GetContainerStats_NoStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containerID := "test-stats-container"
	// Sample JSON output from `docker stats --no-stream --format "{{json .}}"`
	sampleStatsJSON := `{"ID":"testid", "Name":"/testcontainer", "CPUPerc":"1.23%", "MemUsage":"10MiB / 1GiB", "MemPerc":"0.98%", "NetIO":"100B / 200B", "BlockIO":"1kB / 2kB", "PIDs":"5"}`
	cmd := fmt.Sprintf("docker stats --no-stream --format {{json .}} %s", shellEscape(containerID))

	mockConn.EXPECT().Exec(gomock.Any(), cmd, gomock.Any()).Return([]byte(sampleStatsJSON), []byte{}, nil).Times(1)

	statsChan, err := runner.GetContainerStats(ctx, mockConn, containerID, false)
	assert.NoError(t, err)
	assert.NotNil(t, statsChan)

	receivedStats := false
	select {
	case stats, ok := <-statsChan:
		if assert.True(t, ok, "Channel should be open and receive one stat") {
			assert.NoError(t, stats.Error, "Stats error should be nil")
			assert.Equal(t, 1.23, stats.CPUPercentage)
			assert.Equal(t, uint64(10*1024*1024), stats.MemoryUsageBytes)   // 10MiB
			assert.Equal(t, uint64(1024*1024*1024), stats.MemoryLimitBytes) // 1GiB
			assert.Equal(t, uint64(100), stats.NetworkRxBytes)
			assert.Equal(t, uint64(200), stats.NetworkTxBytes)
			assert.Equal(t, uint64(1000), stats.BlockReadBytes)  // 1kB
			assert.Equal(t, uint64(2000), stats.BlockWriteBytes) // 2kB
			assert.Equal(t, uint64(5), stats.PidsCurrent)
			receivedStats = true
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for stats on channel")
	}
	assert.True(t, receivedStats, "Did not receive stats from channel")

	// Check if channel is closed after the single stat is sent
	select {
	case _, ok := <-statsChan:
		assert.False(t, ok, "Channel should be closed after one stat for no-stream")
	case <-time.After(1 * time.Second): // Give a bit of time for the goroutine to close channel
		_, ok := <-statsChan
		assert.False(t, ok, "Channel should be closed after one stat for no-stream (checked after delay)")
	}
}

func TestDefaultRunner_CreateDockerNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	netName := "test-network"
	driver := "bridge"
	subnet := "172.20.0.0/16"
	gateway := "172.20.0.1"
	optsMap := map[string]string{"com.docker.network.bridge.name": "testbridge0"}

	expectedCmd := fmt.Sprintf("docker network create --driver %s --subnet %s --gateway %s --opt %s %s",
		shellEscape(driver), shellEscape(subnet), shellEscape(gateway), shellEscape("com.docker.network.bridge.name=testbridge0"), shellEscape(netName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CreateDockerNetwork(ctx, mockConn, netName, driver, subnet, gateway, optsMap)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).Return(nil, []byte("network error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CreateDockerNetwork(ctx, mockConn, netName, driver, subnet, gateway, optsMap)
	assert.Error(t, err)
}

func TestDefaultRunner_RemoveDockerNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	netName := "test-network-to-remove"

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).
		Return(nil, []byte("Error: No such network: "+netName), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).
		Return(nil, []byte("some other error"), fmt.Errorf("exec error")).Times(1)
	err = runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.Error(t, err)
}

func TestDefaultRunner_CreateDockerVolume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	volName := "test-volume"
	driver := "local"
	driverOpts := map[string]string{"type": "nfs", "o": "addr=192.168.1.1,rw"}
	labels := map[string]string{"env": "dev", "project": "kubexm"}

	// Using gomock.Any() for the command string because exact order of opts/labels is not guaranteed
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
			assert.Contains(t, cmd, "docker volume create")
			assert.Contains(t, cmd, shellEscape(volName))
			assert.Contains(t, cmd, fmt.Sprintf("--driver %s", shellEscape(driver)))
			assert.Contains(t, cmd, fmt.Sprintf("--opt %s", shellEscape("type=nfs")))
			assert.Contains(t, cmd, fmt.Sprintf("--opt %s", shellEscape("o=addr=192.168.1.1,rw")))
			assert.Contains(t, cmd, fmt.Sprintf("--label %s", shellEscape("env=dev")))
			assert.Contains(t, cmd, fmt.Sprintf("--label %s", shellEscape("project=kubexm")))
			return []byte(volName), []byte{}, nil
		}).Times(1)
	err := runner.CreateDockerVolume(ctx, mockConn, volName, driver, driverOpts, labels)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		Return(nil, []byte(fmt.Sprintf("Error response from daemon: a volume with the name %s already exists", volName)), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CreateDockerVolume(ctx, mockConn, volName, "", nil, nil)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		Return(nil, []byte("volume create error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CreateDockerVolume(ctx, mockConn, volName, "", nil, nil)
	assert.Error(t, err)
}

func TestDefaultRunner_DockerPrune(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	mockConn.EXPECT().Exec(ctx, "docker system prune -f --all", gomock.Any()).Return([]byte("Total reclaimed space: 1GB"), []byte{}, nil).Times(1)
	output, err := runner.DockerPrune(ctx, mockConn, "system", nil, true)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 1GB")

	mockConn.EXPECT().Exec(ctx, "docker image prune -f", gomock.Any()).Return([]byte("Total reclaimed space: 500MB"), []byte{}, nil).Times(1)
	output, err = runner.DockerPrune(ctx, mockConn, "image", nil, false)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 500MB")

	filters := map[string]string{"label": "dangling=true"}
	expectedCmd := "docker volume prune -f --filter 'label=dangling=true'"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("Total reclaimed space: 10MB"), []byte{}, nil).Times(1)
	output, err = runner.DockerPrune(ctx, mockConn, "volume", filters, false)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 10MB")

	_, err = runner.DockerPrune(ctx, mockConn, "invalidtype", nil, false)
	assert.Error(t, err)

	mockConn.EXPECT().Exec(ctx, "docker system prune -f", gomock.Any()).Return(nil, []byte("prune error"), fmt.Errorf("exec error")).Times(1)
	_, err = runner.DockerPrune(ctx, mockConn, "system", nil, false)
	assert.Error(t, err)
}

func TestDefaultRunner_GetDockerDaemonConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	rawJSON := `{"log-driver": "json-file", "registry-mirrors": ["https://mirror.docker.com"]}`
	mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte(rawJSON), nil).Times(1)
	config, err := runner.GetDockerDaemonConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "json-file", *config.LogDriver)
	assert.Contains(t, *config.RegistryMirrors, "https://mirror.docker.com")

	mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return(nil, fmt.Errorf("file not found")).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(false, nil).Times(1)
	config, err = runner.GetDockerDaemonConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Nil(t, config.LogDriver)

	mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte{}, nil).Times(1)
	config, err = runner.GetDockerDaemonConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Nil(t, config.LogDriver)

	mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte("{invalid-json"), nil).Times(1)
	config, err = runner.GetDockerDaemonConfig(ctx, mockConn)
	assert.Error(t, err)
	assert.Nil(t, config)

	unexpectedErr := fmt.Errorf("permission denied")
	mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return(nil, unexpectedErr).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).Times(1)
	config, err = runner.GetDockerDaemonConfig(ctx, mockConn)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, unexpectedErr))
	assert.Nil(t, config)
}

func TestDefaultRunner_ConfigureDockerDaemon(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	mockReadFileForEmpty := func() {
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte("{}"), nil).AnyTimes()
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).AnyTimes()
	}

	t.Run("ConfigureNewOptionsNoRestart", func(t *testing.T) {
		mockReadFileForEmpty()
		newLogDriver := "journald"
		newMirrors := []string{"https://mirror1.example.com"}
		opts1 := DockerDaemonOptions{LogDriver: &newLogDriver, RegistryMirrors: &newMirrors}
		expectedJSON1, _ := json.MarshalIndent(opts1, "", "  ")

		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, expectedJSON1, dockerDaemonConfigPath, "0644", true).Return(nil).Times(1)
		err := runner.ConfigureDockerDaemon(ctx, mockConn, opts1, false)
		assert.NoError(t, err)
	})

	t.Run("MergeWithExistingAndRestart", func(t *testing.T) {
		existingLogDriver := "json-file"
		existingJSON := fmt.Sprintf(`{"log-driver": "%s"}`, existingLogDriver)
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte(existingJSON), nil).Times(1)
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).AnyTimes()

		newExecOpts := []string{"native.cgroupdriver=systemd"}
		newDataRoot := "/mnt/docker-data"
		opts2 := DockerDaemonOptions{ExecOpts: &newExecOpts, DataRoot: &newDataRoot}
		expectedMergedOpts := DockerDaemonOptions{LogDriver: &existingLogDriver, ExecOpts: &newExecOpts, DataRoot: &newDataRoot}
		expectedJSON2, _ := json.MarshalIndent(expectedMergedOpts, "", "  ")

		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, expectedJSON2, dockerDaemonConfigPath, "0644", true).Return(nil).Times(1)
		mockFacts := &Facts{OS: &connector.OS{Family: "ubuntu"}, InitSystem: &ServiceInfo{Type: InitSystemSystemd}}
		mockConn.EXPECT().GatherFacts(ctx, mockConn).Return(mockFacts, nil).Times(1)
		mockConn.EXPECT().RestartService(ctx, mockConn, mockFacts, "docker").Return(nil).Times(1)
		err := runner.ConfigureDockerDaemon(ctx, mockConn, opts2, true)
		assert.NoError(t, err)
	})

	t.Run("WriteFileFails", func(t *testing.T) {
		mockReadFileForEmpty()
		opts3 := DockerDaemonOptions{LogDriver: strPtr("fluentd")}
		expectedJSON3, _ := json.MarshalIndent(opts3, "", "  ")
		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, expectedJSON3, dockerDaemonConfigPath, "0644", true).Return(fmt.Errorf("disk full")).Times(1)
		err := runner.ConfigureDockerDaemon(ctx, mockConn, opts3, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write Docker daemon config")
	})

	t.Run("RestartServiceFails", func(t *testing.T) {
		mockReadFileForEmpty()
		opts4 := DockerDaemonOptions{Debug: util.BoolPtr(true)}
		expectedJSON4, _ := json.MarshalIndent(opts4, "", "  ")
		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, expectedJSON4, dockerDaemonConfigPath, "0644", true).Return(nil).Times(1)
		mockFacts := &Facts{OS: &connector.OS{Family: "ubuntu"}, InitSystem: &ServiceInfo{Type: InitSystemSystemd}}
		mockConn.EXPECT().GatherFacts(ctx, mockConn).Return(mockFacts, nil).Times(1)
		mockConn.EXPECT().RestartService(ctx, mockConn, mockFacts, "docker").Return(fmt.Errorf("systemctl failed")).Times(1)
		err := runner.ConfigureDockerDaemon(ctx, mockConn, opts4, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to restart Docker service")
	})
}

func TestDefaultRunner_EnsureDefaultDockerConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	mockFacts := &Facts{OS: &connector.OS{Family: "debian"}, InitSystem: &ServiceInfo{Type: InitSystemSystemd}}

	t.Run("ConfigFileDoesNotExist", func(t *testing.T) {
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(false, nil).Times(1)
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return(nil, fmt.Errorf("not found")).AnyTimes()
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(false, nil).AnyTimes() // For Get inside Configure
		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), dockerDaemonConfigPath, "0644", true).
			DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
				var opts DockerDaemonOptions
				json.Unmarshal(content, &opts)
				assert.Equal(t, "systemd", (*opts.ExecOpts)[0])
				assert.Equal(t, "json-file", *opts.LogDriver)
				return nil
			}).Times(1)
		mockConn.EXPECT().GatherFacts(ctx, mockConn).Return(mockFacts, nil).Times(1)
		mockConn.EXPECT().RestartService(ctx, mockConn, mockFacts, "docker").Return(nil).Times(1)
		err := runner.EnsureDefaultDockerConfig(ctx, mockConn, mockFacts, true)
		assert.NoError(t, err)
	})

	t.Run("ConfigFileIsEmpty", func(t *testing.T) {
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).Times(1)
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte(""), nil).Times(1)
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte(""), nil).AnyTimes()
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).AnyTimes()
		mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(dockerDaemonConfigPath), "0755", true).Return(nil).Times(1)
		mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), dockerDaemonConfigPath, "0644", true).
			DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
				var opts DockerDaemonOptions
				json.Unmarshal(content, &opts)
				assert.Equal(t, "systemd", (*opts.ExecOpts)[0])
				return nil
			}).Times(1)
		err := runner.EnsureDefaultDockerConfig(ctx, mockConn, mockFacts, false)
		assert.NoError(t, err)
	})

	t.Run("ConfigFileIsNotEmpty", func(t *testing.T) {
		existingJSON := `{"log-driver": "custom"}`
		mockConn.EXPECT().Exists(ctx, mockConn, dockerDaemonConfigPath).Return(true, nil).Times(1)
		mockConn.EXPECT().ReadFile(ctx, mockConn, dockerDaemonConfigPath).Return([]byte(existingJSON), nil).Times(1)
		err := runner.EnsureDefaultDockerConfig(ctx, mockConn, mockFacts, true)
		assert.NoError(t, err)
	})
}
