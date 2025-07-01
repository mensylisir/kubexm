package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
)

func TestDefaultRunner_RemoveContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	containerID := "test-container"

	// Test case 1: Successful removal
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err := runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.NoError(t, err)

	// Test case 2: Successful removal with force
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, false)
	assert.NoError(t, err)

	// Test case 3: Successful removal with force and remove volumes
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f -v %s", shellEscape(containerID)), gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, true)
	assert.NoError(t, err)

	// Test case 4: Container not found, with force (should not error)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm -f %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("Error: No such container: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, true, false)
	assert.NoError(t, err)

	// Test case 5: Container not found, without force (should error)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("Error: No such container: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such container")

	// Test case 6: Docker command execution fails
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker rm %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("some docker error"), fmt.Errorf("exec error")).Times(1)
	err = runner.RemoveContainer(ctx, mockConn, containerID, false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")

	// Test case 7: Empty container ID
	err = runner.RemoveContainer(ctx, mockConn, "", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")

	// Test case 8: Nil connector
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

	// Sample JSON output from `docker ps --format "{{json .}}"`
	sampleContainer1JSON := `{"ID":"id1","Image":"image1","Command":"cmd1","CreatedAt":"2023-01-01 10:00:00 +0000 UTC","Names":"name1","Labels":"k1=v1,k2=v2","Mounts":"/mnt1,/mnt2","Networks":"net1","Ports":"80/tcp","Status":"Up 2 hours"}`
	sampleContainer2JSON := `{"ID":"id2","Image":"image2","Command":"cmd2","CreatedAt":"2023-01-02 12:00:00 +0000 UTC","Names":"name2,altName2","Labels":"k3=v3","Mounts":"","Networks":"net2","Ports":"0.0.0.0:8080->80/tcp","Status":"Exited (0) 1 day ago"}`

	// Expected parsed structures
	expectedContainer1 := ContainerInfo{
		ID:      "id1",
		Image:   "image1",
		Command: "cmd1",
		Created: time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC).Unix(),
		Names:   []string{"name1"},
		Labels:  map[string]string{"k1": "v1", "k2": "v2"},
		Mounts:  []ContainerMount{{Source: "/mnt1"}, {Source: "/mnt2"}},
		// Networks: []string{"net1"}, // Simplified, Networks field not directly in ContainerInfo
		Ports:   []ContainerPortMapping{{ContainerPort: "80", Protocol: "tcp"}},
		State:   "running",
		Status:  "Up 2 hours",
	}
	parsedTime2 := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).Unix()
	expectedContainer2 := ContainerInfo{
		ID:      "id2",
		Image:   "image2",
		Command: "cmd2",
		Created: parsedTime2,
		Names:   []string{"name2", "altName2"},
		Labels:  map[string]string{"k3": "v3"},
		Mounts:  nil,
		Ports:   []ContainerPortMapping{{HostIP: "0.0.0.0", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		State:   "exited",
		Status:  "Exited (0) 1 day ago",
	}


	// Test case 1: Successful list, all=false, no filters
	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return([]byte(sampleContainer1JSON+"\n"+sampleContainer2JSON), []byte{}, nil).Times(1)

	containers, err := runner.ListContainers(ctx, mockConn, false, nil)
	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	// Deep comparison can be tricky due to unexported fields or complex sub-structs if not careful with test setup
	// For critical fields:
	assert.Equal(t, expectedContainer1.ID, containers[0].ID)
	assert.Equal(t, expectedContainer1.Image, containers[0].Image)
	assert.Equal(t, expectedContainer1.State, containers[0].State)
	assert.Equal(t, expectedContainer2.ID, containers[1].ID)
	assert.Equal(t, expectedContainer2.Image, containers[1].Image)
	assert.Equal(t, expectedContainer2.State, containers[1].State)
	assert.Equal(t, expectedContainer2.Ports, containers[1].Ports)


	// Test case 2: Successful list, all=true, with filters
	filters := map[string]string{"status": "running"}
	expectedCmdWithFilters := "docker ps --all --filter 'status=running' --format {{json .}}"
	mockConn.EXPECT().Exec(ctx, expectedCmdWithFilters, gomock.Any()).
		Return([]byte(sampleContainer1JSON), []byte{}, nil).Times(1)

	containers, err = runner.ListContainers(ctx, mockConn, true, filters)
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, expectedContainer1.ID, containers[0].ID)

	// Test case 3: Docker command execution fails
	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return(nil, []byte("docker error"), fmt.Errorf("exec error")).Times(1)
	containers, err = runner.ListContainers(ctx, mockConn, false, nil)
	assert.Error(t, err)
	assert.Nil(t, containers)
	assert.Contains(t, err.Error(), "exec error")

	// Test case 4: Invalid JSON output
	mockConn.EXPECT().Exec(ctx, "docker ps --format {{json .}}", gomock.Any()).
		Return([]byte("this is not json"), []byte{}, nil).Times(1)
	containers, err = runner.ListContainers(ctx, mockConn, false, nil)
	assert.Error(t, err)
	assert.Nil(t, containers)
	assert.Contains(t, err.Error(), "failed to parse container JSON line")

	// Test case 5: Empty filter key/value
	_, err = runner.ListContainers(ctx, mockConn, false, map[string]string{"": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "filter key and value cannot be empty")

	// Test case 6: Nil connector
	_, err = runner.ListContainers(ctx, nil, false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

// Note: utils.ParseSizeToBytes would need to be accessible or reimplemented for GetContainerStats test
// For now, GetContainerStats test will be basic due to this dependency and streaming complexity.

func TestDefaultRunner_GetContainerLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-log-container"

	// Test case 1: Successful log retrieval
	logOutput := "Log line 1\nLog line 2"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker logs %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte(logOutput), []byte{}, nil).Times(1)
	logs, err := runner.GetContainerLogs(ctx, mockConn, containerID, ContainerLogOptions{})
	assert.NoError(t, err)
	assert.Equal(t, logOutput, logs)

	// Test case 2: With options
	opts := ContainerLogOptions{
		Timestamps: true,
		Tail:       "100",
		Since:      "2023-01-01T00:00:00Z",
	}
	expectedCmdWithOpts := fmt.Sprintf("docker logs --timestamps --since %s --tail '100' %s", shellEscape(opts.Since), shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, expectedCmdWithOpts, gomock.Any()).
		Return([]byte(logOutput), []byte{}, nil).Times(1)
	logs, err = runner.GetContainerLogs(ctx, mockConn, containerID, opts)
	assert.NoError(t, err)
	assert.Equal(t, logOutput, logs)

	// Test case 3: Docker command execution fails
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker logs %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("docker error"), fmt.Errorf("exec error")).Times(1)
	logs, err = runner.GetContainerLogs(ctx, mockConn, containerID, ContainerLogOptions{})
	assert.Error(t, err)
	assert.Empty(t, logs)
	assert.Contains(t, err.Error(), "exec error")

	// Test case 4: Empty container ID
	_, err = runner.GetContainerLogs(ctx, mockConn, "", ContainerLogOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")

	// Test case 5: Nil connector
	_, err = runner.GetContainerLogs(ctx, nil, containerID, ContainerLogOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}


func TestDefaultRunner_InspectContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-inspect-container"

	sampleInspectJSON := `[
    {
        "Id": "abcdef123456",
        "Created": "2023-10-27T10:20:30.123456789Z",
        "Path": "/bin/sh",
        "Args": ["-c", "while true; do echo hello; sleep 1; done"],
        "State": {
            "Status": "running",
            "Running": true,
            "Paused": false,
            "Restarting": false,
            "OOMKilled": false,
            "Dead": false,
            "Pid": 12345,
            "ExitCode": 0,
            "Error": "",
            "StartedAt": "2023-10-27T10:20:31.123456789Z",
            "FinishedAt": "0001-01-01T00:00:00Z"
        },
        "Image": "sha256:fedcba987654",
        "Name": "/trusting_archimedes",
		"HostConfig": {
			"NetworkMode": "default"
		}
    }
]`
	var expectedDetailsContainerArray []ContainerDetails
	err := json.Unmarshal([]byte(sampleInspectJSON), &expectedDetailsContainerArray)
	assert.NoError(t, err, "Failed to unmarshal sample inspect JSON for test setup")
	expectedDetails := expectedDetailsContainerArray[0]


	// Test case 1: Successful inspect
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte(sampleInspectJSON), []byte{}, nil).Times(1)
	details, err := runner.InspectContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)
	assert.NotNil(t, details)
	assert.Equal(t, expectedDetails.ID, details.ID)
	assert.Equal(t, expectedDetails.State.Status, details.State.Status)
	assert.Equal(t, expectedDetails.Name, details.Name)


	// Test case 2: Container not found (docker inspect returns exit code 1 and error in stderr)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("Error: No such object: "+containerID), &connector.CommandError{ExitCode: 1}).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.NoError(t, err) // Should return nil, nil for not found
	assert.Nil(t, details)

	// Test case 3: Docker command execution fails with a different error
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return(nil, []byte("some other docker error"), fmt.Errorf("exec error")).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "exec error")

	// Test case 4: Invalid JSON output
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker inspect %s", shellEscape(containerID)), gomock.Any()).
		Return([]byte("this is not json"), []byte{}, nil).Times(1)
	details, err = runner.InspectContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "failed to parse container inspect JSON")

	// Test case 5: Empty container ID
	_, err = runner.InspectContainer(ctx, mockConn, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "containerNameOrID cannot be empty")

	// Test case 6: Nil connector
	_, err = runner.InspectContainer(ctx, nil, containerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

func TestDefaultRunner_PauseUnpauseContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-pause-container"

	// Pause Test case 1: Successful pause
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.PauseContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)

	// Pause Test case 2: Docker command execution fails for pause
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker pause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("pause error"), fmt.Errorf("exec pause error")).Times(1)
	err = runner.PauseContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec pause error")

	// Unpause Test case 1: Successful unpause
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker unpause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.UnpauseContainer(ctx, mockConn, containerID)
	assert.NoError(t, err)

	// Unpause Test case 2: Docker command execution fails for unpause
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker unpause %s", shellEscape(containerID)), gomock.Any()).Return(nil, []byte("unpause error"), fmt.Errorf("exec unpause error")).Times(1)
	err = runner.UnpauseContainer(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec unpause error")
}

func TestDefaultRunner_ExecInContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-exec-container"
	cmdToExec := []string{"ls", "-l", "/tmp"}

	// Test case 1: Successful exec
	expectedCmdStr := fmt.Sprintf("docker exec %s %s %s %s", shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]), shellEscape(cmdToExec[2]))
	mockConn.EXPECT().Exec(ctx, expectedCmdStr, gomock.Any()).Return([]byte("stdout content"), []byte("stderr content"), nil).Times(1)
	output, err := runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, "", "", false)
	assert.NoError(t, err)
	assert.Equal(t, "stdout content"+"stderr content", output)

	// Test case 2: Exec with user and workDir and TTY
	user := "testuser"
	workDir := "/app"
	expectedCmdWithUserWorkdirTTY := fmt.Sprintf("docker exec -t --user %s --workdir %s %s %s %s %s", shellEscape(user), shellEscape(workDir), shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]), shellEscape(cmdToExec[2]))
	mockConn.EXPECT().Exec(ctx, expectedCmdWithUserWorkdirTTY, gomock.Any()).Return([]byte("tty output"), []byte{}, nil).Times(1)
	output, err = runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, user, workDir, true)
	assert.NoError(t, err)
	assert.Equal(t, "tty output", output)


	// Test case 3: Command in container fails (docker exec itself succeeds but command returns non-zero)
	// This is represented by Exec returning an error (e.g. *connector.CommandError)
	mockConn.EXPECT().Exec(ctx, expectedCmdStr, gomock.Any()).
		Return([]byte("stdout on fail"), []byte("stderr on fail"), &connector.CommandError{ExitCode: 127}).Times(1)
	output, err = runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, "", "", false)
	assert.Error(t, err)
	assert.Contains(t, output, "stdout on fail")
	assert.Contains(t, output, "stderr on fail")
	assert.Contains(t, err.Error(), "failed to exec in container")


	// Test case 4: Docker command execution itself fails (e.g., container not running)
	mockConn.EXPECT().Exec(ctx, expectedCmdStr, gomock.Any()).
		Return(nil, []byte("Error: container not running"), fmt.Errorf("exec error")).Times(1)
	output, err = runner.ExecInContainer(ctx, mockConn, containerID, cmdToExec, "", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")
	assert.Contains(t, output, "Error: container not running") // output might still contain stderr from the command

	// Test case 5: Empty command
	_, err = runner.ExecInContainer(ctx, mockConn, containerID, []string{}, "", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command to execute cannot be empty")
}

// Basic test for GetContainerStats (non-streaming part)
// A full streaming test is complex with current Exec mock and would require a more sophisticated mock or actual Docker.
func TestDefaultRunner_GetContainerStats_NoStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Timeout for the test itself
	defer cancel()

	containerID := "test-stats-container"
	// Sample JSON output from `docker stats --no-stream --format "{{json .}}"`
	// Note: The actual ContainerStats struct in interface.go is simpler than this full JSON.
	// The parsing logic in GetContainerStats will map these fields.
	sampleStatsJSON := `{
		"ID":"testid", "Name":"/testcontainer",
		"CPUPerc":"1.23%", "MemUsage":"10MiB / 1GiB", "MemPerc":"0.98%",
		"NetIO":"100B / 200B", "BlockIO":"1kB / 2kB", "PIDs":"5"
	}`

	cmd := fmt.Sprintf("docker stats --no-stream --format {{json .}} %s", shellEscape(containerID))
	// For the non-streaming case, GetContainerStats makes one Exec call.
	// The parsing of the JSON into ContainerStats happens within the function.
	// We provide a mock JSON that GetContainerStats is expected to parse.
	mockConn.EXPECT().Exec(gomock.Any(), cmd, gomock.Any()).
		Return([]byte(sampleStatsJSON), []byte{}, nil).Times(1)

	statsChan, err := runner.GetContainerStats(ctx, mockConn, containerID, false)
	assert.NoError(t, err)
	assert.NotNil(t, statsChan)

	receivedStats := false
	select {
	case stats, ok := <-statsChan:
		if assert.True(t, ok, "Channel should be open and receive one stat") {
			assert.NoError(t, stats.Error, "Stats error should be nil")
			// Verify some basic parsing if possible.
			// The actual parsing logic is in GetContainerStats, using utils.ParseSizeToBytes.
			// We can't directly verify utils.ParseSizeToBytes here without making it an interface
			// or setting up more complex test data.
			// For now, we rely on the fact that no error was propagated from parsing.
			// If CPUPercentage was parsed, it would be 1.23.
			// If MemoryUsageBytes was parsed from "10MiB", it would be 10 * 1024 * 1024.
			// This test ensures the channel communication and basic error handling.
			// Deeper parsing verification would require more focused unit tests on the parsing logic itself
			// or more detailed assertions here if the internal fields of ContainerStats were directly settable/comparable
			// from the raw JSON fields shown in sampleStatsJSON.
			// Example (if CPUPercentage was float64 and directly parsed):
			// assert.InDelta(t, 1.23, stats.CPUPercentage, 0.001)
			receivedStats = true
		}
	case <-time.After(3 * time.Second): // Timeout for receiving from channel
		t.Fatal("Timeout waiting for stats on channel")
	}
	assert.True(t, receivedStats, "Did not receive stats from channel")

	// Check if channel is closed
	_, ok := <-statsChan
	assert.False(t, ok, "Channel should be closed after one stat for no-stream")
}
// TODO: Add more tests for other functions: CreateDockerNetwork, RemoveDockerNetwork, etc.
// For brevity, only a subset of tests are included here. The pattern would be similar.

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
	opts := map[string]string{"com.docker.network.bridge.name": "testbridge0"}

	// Test case 1: Successful creation
	expectedCmd := fmt.Sprintf("docker network create --driver %s --subnet %s --gateway %s --opt %s %s",
		shellEscape(driver), shellEscape(subnet), shellEscape(gateway), shellEscape("com.docker.network.bridge.name=testbridge0"), shellEscape(netName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CreateDockerNetwork(ctx, mockConn, netName, driver, subnet, gateway, opts)
	assert.NoError(t, err)

	// Test case 2: Command failure
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).Return(nil, []byte("network error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CreateDockerNetwork(ctx, mockConn, netName, driver, subnet, gateway, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")
}

func TestDefaultRunner_RemoveDockerNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	netName := "test-network-to-remove"

	// Test case 1: Successful removal
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.NoError(t, err)

	// Test case 2: Network not found (idempotency)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).
		Return(nil, []byte("Error: No such network: "+netName), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.NoError(t, err)

	// Test case 3: Other command failure
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("docker network rm %s", shellEscape(netName)), gomock.Any()).
		Return(nil, []byte("some other error"), fmt.Errorf("exec error")).Times(1)
	err = runner.RemoveDockerNetwork(ctx, mockConn, netName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")
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

	// Test case 1: Successful creation with all options
	// Order of labels and driverOpts can vary, so use gomock.Any() for cmd or more complex matcher if needed.
	// For simplicity, we assume a fixed order for this test or use Contains.
	expectedCmdParts := []string{
		"docker", "volume", "create",
		"--driver", shellEscape(driver),
		"--opt", shellEscape("type=nfs"),
		"--opt", shellEscape("o=addr=192.168.1.1,rw"),
		"--label", shellEscape("env=dev"),
		"--label", shellEscape("project=kubexm"),
		shellEscape(volName),
	}
	// A more robust way for options and labels would be to check for substrings or use a custom gomock.Matcher
	// This test relies on the order produced by the implementation.

	mockConn.EXPECT().Exec(ctx, gomock.AssignableToTypeOf("string"), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
			for _, part := range expectedCmdParts {
				assert.Contains(t, cmd, part)
			}
			assert.True(t, strings.HasSuffix(cmd, shellEscape(volName))) // Ensure name is last among args
			return []byte(volName), []byte{}, nil
		}).Times(1)
	err := runner.CreateDockerVolume(ctx, mockConn, volName, driver, driverOpts, labels)
	assert.NoError(t, err)

	// Test case 2: Volume already exists (idempotency)
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		Return(nil, []byte(fmt.Sprintf("Error response from daemon: a volume with the name %s already exists", volName)), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CreateDockerVolume(ctx, mockConn, volName, "", nil, nil) // Simpler call for this case
	assert.NoError(t, err)

	// Test case 3: Command failure
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		Return(nil, []byte("volume create error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CreateDockerVolume(ctx, mockConn, volName, "", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")
}

func TestDefaultRunner_DockerPrune(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test case 1: System prune, all
	mockConn.EXPECT().Exec(ctx, "docker system prune -f --all", gomock.Any()).Return([]byte("Total reclaimed space: 1GB"), []byte{}, nil).Times(1)
	output, err := runner.DockerPrune(ctx, mockConn, "system", nil, true)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 1GB")

	// Test case 2: Image prune, not all
	mockConn.EXPECT().Exec(ctx, "docker image prune -f", gomock.Any()).Return([]byte("Total reclaimed space: 500MB"), []byte{}, nil).Times(1)
	output, err = runner.DockerPrune(ctx, mockConn, "image", nil, false)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 500MB")

	// Test case 3: Volume prune with filter (example filter, actual filter syntax varies)
	filters := map[string]string{"label": "dangling=true"}
	// The current implementation of DockerPrune for filters is basic (key=value).
	// Docker CLI for `volume prune` uses `--filter label=key` or `--filter label=key=value`.
	// This test will use the current implementation's filter format.
	// A more robust test would need to align with actual Docker CLI filter capabilities.
	expectedCmd := "docker volume prune -f --filter 'label=dangling=true'"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("Total reclaimed space: 10MB"), []byte{}, nil).Times(1)
	output, err = runner.DockerPrune(ctx, mockConn, "volume", filters, false)
	assert.NoError(t, err)
	assert.Contains(t, output, "Total reclaimed space: 10MB")

	// Test case 4: Invalid pruneType
	_, err = runner.DockerPrune(ctx, mockConn, "invalidtype", nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pruneType: invalidtype")

	// Test case 5: Command failure
	mockConn.EXPECT().Exec(ctx, "docker system prune -f", gomock.Any()).Return(nil, []byte("prune error"), fmt.Errorf("exec error")).Times(1)
	_, err = runner.DockerPrune(ctx, mockConn, "system", nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec error")
}

// Mock for utils.ParseSizeToBytes if not directly testable or to avoid external dependency in this test file
// For now, this is not implemented as it's a util function. Tests for GetContainerStats are simplified.
// If utils.ParseSizeToBytes is part of the same package or easily mockable, tests can be more detailed.
// Consider moving utils.ParseSizeToBytes to a shared testing utility or making it part of an interface if it needs mocking.
// As it stands, the GetContainerStats test relies on its correct behavior implicitly.
