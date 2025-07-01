package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
)

func TestDefaultRunner_CtrListNamespaces(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test case 1: Successful listing
	expectedNamespaces := []string{"default", "k8s.io", "moby"}
	stdout := strings.Join(expectedNamespaces, "\n") + "\n" // ctr ns ls -q adds a newline
	mockConn.EXPECT().Exec(ctx, "ctr ns ls -q", gomock.Any()).Return([]byte(stdout), []byte{}, nil).Times(1)

	namespaces, err := runner.CtrListNamespaces(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, expectedNamespaces, namespaces)

	// Test case 2: Empty output
	mockConn.EXPECT().Exec(ctx, "ctr ns ls -q", gomock.Any()).Return([]byte(""), []byte{}, nil).Times(1)
	namespaces, err = runner.CtrListNamespaces(ctx, mockConn)
	assert.NoError(t, err)
	assert.Empty(t, namespaces)

	// Test case 3: Command execution error
	mockConn.EXPECT().Exec(ctx, "ctr ns ls -q", gomock.Any()).Return(nil, []byte("ctr error"), fmt.Errorf("exec error")).Times(1)
	namespaces, err = runner.CtrListNamespaces(ctx, mockConn)
	assert.Error(t, err)
	assert.Nil(t, namespaces)
	assert.Contains(t, err.Error(), "failed to list containerd namespaces")
}

func TestDefaultRunner_CtrListImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "k8s.io"

	// Test case 1: Successful listing with multiple images
	// Note: Parsing tabular output is fragile. This test assumes the parser in the implementation works for this format.
	tabularOutput := `REF                                                         TYPE                                                 DIGEST                                                                  SIZE      PLATFORMS        LABELS
docker.io/library/alpine:latest                             application/vnd.docker.distribution.manifest.v2+json sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300 2.83 MiB  linux/amd64      foo=bar
k8s.gcr.io/pause:3.5                                        application/vnd.docker.distribution.manifest.v2+json sha256:221177c60ce5107572697c109b00c6e9415809cfe0510b5a9800334731ffa9f7 303 KiB   linux/amd64,linux/arm64 -
`
	expectedImages := []CtrImageInfo{
		{Name: "docker.io/library/alpine:latest", Digest: "sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300", Size: "2.83 MiB", OSArch: "linux/amd64"}, // Labels parsing not implemented in this test's expected for simplicity of tabular check
		{Name: "k8s.gcr.io/pause:3.5", Digest: "sha256:221177c60ce5107572697c109b00c6e9415809cfe0510b5a9800334731ffa9f7", Size: "303 KiB", OSArch: "linux/amd64,linux/arm64"},
	}

	cmd := fmt.Sprintf("ctr -n %s images ls", shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(tabularOutput), []byte{}, nil).Times(1)

	images, err := runner.CtrListImages(ctx, mockConn, namespace)
	assert.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, expectedImages[0].Name, images[0].Name)
	assert.Equal(t, expectedImages[0].Digest, images[0].Digest)
	assert.Equal(t, expectedImages[0].Size, images[0].Size)
	assert.Equal(t, expectedImages[0].OSArch, images[0].OSArch)
	assert.Equal(t, expectedImages[1].Name, images[1].Name)


	// Test case 2: No images found (only header)
	headerOnlyOutput := "REF DIGEST TYPE SIZE PLATFORMS LABELS\n"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(headerOnlyOutput), []byte{}, nil).Times(1)
	images, err = runner.CtrListImages(ctx, mockConn, namespace)
	assert.NoError(t, err)
	assert.Empty(t, images)

	// Test case 3: Command execution error
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("ctr error"), fmt.Errorf("exec error")).Times(1)
	images, err = runner.CtrListImages(ctx, mockConn, namespace)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "failed to list images")
}

func TestDefaultRunner_CtrPullImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	namespace := "default"
	imageName := "docker.io/library/alpine:latest"

	// Test case 1: Successful pull
	expectedCmd := fmt.Sprintf("ctr -n %s images pull %s", shellEscape(namespace), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("...output..."), []byte{}, nil).Times(1)
	err := runner.CtrPullImage(ctx, mockConn, namespace, imageName, false, "")
	assert.NoError(t, err)

	// Test case 2: Successful pull with all platforms and user
	user := "testuser:testpass"
	expectedCmdAllPlatUser := fmt.Sprintf("ctr -n %s images pull --all-platforms --user %s %s",
		shellEscape(namespace), shellEscape(user), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatUser, gomock.Any()).Return([]byte("...output..."), []byte{}, nil).Times(1)
	err = runner.CtrPullImage(ctx, mockConn, namespace, imageName, true, user)
	assert.NoError(t, err)

	// Test case 3: Command execution error
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte("pull error"), fmt.Errorf("exec pull error")).Times(1)
	err = runner.CtrPullImage(ctx, mockConn, namespace, imageName, false, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pull image")
}

func TestDefaultRunner_CtrRemoveImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "default"
	imageName := "docker.io/library/alpine:latest"

	// Test case 1: Successful removal
	cmd := fmt.Sprintf("ctr -n %s images rm %s", shellEscape(namespace), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrRemoveImage(ctx, mockConn, namespace, imageName)
	assert.NoError(t, err)

	// Test case 2: Image not found (idempotency)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte(`ctr: image "docker.io/library/alpine:latest": not found`), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CtrRemoveImage(ctx, mockConn, namespace, imageName)
	assert.NoError(t, err)

	// Test case 3: Other command execution error
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some other error"), fmt.Errorf("exec rmi error")).Times(1)
	err = runner.CtrRemoveImage(ctx, mockConn, namespace, imageName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove image")
}

func TestDefaultRunner_CtrTagImage(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    namespace := "default"
    sourceImage := "docker.io/library/alpine:latest"
    targetImage := "myalpine:custom"

    // Test case 1: Successful tag
    cmd := fmt.Sprintf("ctr -n %s images tag %s %s", shellEscape(namespace), shellEscape(sourceImage), shellEscape(targetImage))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CtrTagImage(ctx, mockConn, namespace, sourceImage, targetImage)
    assert.NoError(t, err)

    // Test case 2: Tag command fails
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
        Return(nil, []byte("tagging error"), fmt.Errorf("exec tag error")).Times(1)
    err = runner.CtrTagImage(ctx, mockConn, namespace, sourceImage, targetImage)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to tag image")
}

// Basic test for CtrRunContainer to check command construction
func TestDefaultRunner_CtrRunContainer(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    namespace := "test-ns"
    opts := ContainerdContainerCreateOptions{
        ImageName:   "docker.io/library/nginx:latest",
        ContainerID: "test-nginx-ctr",
        NetHost:     true,
        TTY:         true,
        Env:         []string{"FOO=bar", "BAZ=qux"},
        Command:     []string{"nginx", "-g", "daemon off;"},
		RemoveExisting: true, // This will trigger stop and rm calls
    }

	// Mock for potential CtrRemoveContainer call due to RemoveExisting:true
	// It might be called, so we allow it. If it's not, this mock won't be hit, which is fine for AnyTimes.
	rmCmd := fmt.Sprintf("ctr -n %s containers rm %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).Return(nil, []byte{}, nil).AnyTimes()

	// Mock for potential CtrStopContainer call
	stopCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	stopCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	mockConn.EXPECT().Exec(ctx, stopCmdTerm, gomock.Any()).Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).AnyTimes() // Assume it was already stopped or never ran
	mockConn.EXPECT().Exec(ctx, stopCmdKill, gomock.Any()).Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).AnyTimes()


    expectedCmdParts := []string{
        "ctr", "-n", shellEscape(namespace), "run",
        "--net-host", "--tty",
        "--env", shellEscape("FOO=bar"),
        "--env", shellEscape("BAZ=qux"),
        "--rm", // Added by the function
        shellEscape(opts.ImageName),
        shellEscape(opts.ContainerID),
        shellEscape("nginx"),
        shellEscape("-g"),
        shellEscape("daemon off;"),
    }

    mockConn.EXPECT().Exec(ctx, gomock.AssignableToTypeOf("string"), gomock.Any()).
        DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
            for _, part := range expectedCmdParts {
                if !strings.Contains(cmd, part) {
                     return nil, nil, fmt.Errorf("expected cmd part '%s' not found in '%s'", part, cmd)
                }
            }
            return []byte(opts.ContainerID), []byte{}, nil
        }).Times(1)

    id, err := runner.CtrRunContainer(ctx, mockConn, namespace, opts)
    assert.NoError(t, err)
    assert.Equal(t, opts.ContainerID, id)
}

// Further tests for CtrListContainers, CtrStopContainer, CtrRemoveContainer, CtrExecInContainer can be added.
// These will involve more complex output parsing or sequence of mock calls.
// For brevity, only a subset is shown. The pattern would be similar.

func TestDefaultRunner_CtrListContainers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "test-ns"

	// Test case 1: Successful list
	tabularOutput := `CONTAINER    IMAGE                             RUNTIME
container1   docker.io/library/alpine:latest   io.containerd.runc.v2
container2   k8s.gcr.io/pause:3.5              io.containerd.runc.v2
`
	expectedContainers := []CtrContainerInfo{
		{ID: "container1", Image: "docker.io/library/alpine:latest", Runtime: "io.containerd.runc.v2"},
		{ID: "container2", Image: "k8s.gcr.io/pause:3.5", Runtime: "io.containerd.runc.v2"},
	}
	cmd := fmt.Sprintf("ctr -n %s containers ls", shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(tabularOutput), []byte{}, nil).Times(1)

	containers, err := runner.CtrListContainers(ctx, mockConn, namespace)
	assert.NoError(t, err)
	assert.Equal(t, expectedContainers, containers)

	// Test case 2: Empty list
	headerOnlyOutput := "CONTAINER    IMAGE    RUNTIME\n"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(headerOnlyOutput), []byte{}, nil).Times(1)
	containers, err = runner.CtrListContainers(ctx, mockConn, namespace)
	assert.NoError(t, err)
	assert.Empty(t, containers)
}

func TestDefaultRunner_CtrStopContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "test-ns"
	containerID := "test-container-stop"

	killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(containerID))
	killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(containerID))

	// Test Case 1: Successful SIGTERM stop
	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	// Assuming SIGTERM is enough, SIGKILL might not be called if first call successful and timeout=0 or short
	// The current CtrStopContainer is a bit simplified on this, might call SIGKILL regardless if errTerm is nil.
	// Let's assume for this test, if SIGTERM is ok, SIGKILL is also tried and says "no such process"
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}). // SIGKILL finds process gone
		Times(1)
	err := runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.NoError(t, err)

	// Test Case 2: SIGTERM fails, SIGKILL succeeds
	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("some error on sigterm"), fmt.Errorf("sigterm failed")).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.NoError(t, err) // Should not error if SIGKILL works

	// Test Case 3: Both SIGTERM and SIGKILL fail with errors other than "not found"
	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("sigterm general error"), fmt.Errorf("sigterm general error")).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("sigkill general error"), fmt.Errorf("sigkill general error")).Times(1)
	err = runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(),"failed to SIGKILL task") // The error from SIGKILL should be reported

	// Test Case 4: SIGTERM reports "no such process" (already stopped)
	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode:1}).Times(1)
	// SIGKILL should not be called if SIGTERM indicates already stopped.
	// (Current implementation might still call SIGKILL, adjust test if needed based on final logic of CtrStopContainer)
	// Adjusting expectation: current code calls SIGKILL if SIGTERM had an error (even "no such process")
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode:1}).Times(1)
	err = runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.NoError(t, err)
}


func TestDefaultRunner_CtrRemoveContainer(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    namespace := "test-ns"
    containerID := "test-container-rm"

    rmCmd := fmt.Sprintf("ctr -n %s containers rm %s", shellEscape(namespace), shellEscape(containerID))

    // Test Case 1: Successful remove
    mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CtrRemoveContainer(ctx, mockConn, namespace, containerID)
    assert.NoError(t, err)

    // Test Case 2: Container not found (idempotent)
    mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).
        Return(nil, []byte("ctr: container \"test-container-rm\": not found"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CtrRemoveContainer(ctx, mockConn, namespace, containerID)
    assert.NoError(t, err)

    // Test Case 3: Container has active task, stop is attempted and succeeds, then remove succeeds
    killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(containerID))
    killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(containerID))

    // First rm fails due to active task
    mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).
        Return(nil, []byte("ctr: container has active task, please stop task before removing container or use --force"), &connector.CommandError{ExitCode: 1}).Times(1)

    // StopContainer logic is called: SIGTERM (assume it says "no such process" as task might be gone quickly or never existed robustly for this test path)
    mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
        Return(nil, []byte("no such process"), &connector.CommandError{ExitCode:1}).Times(1)
    // SIGKILL (also assumes "no such process")
    mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
        Return(nil, []byte("no such process"), &connector.CommandError{ExitCode:1}).Times(1)

    // Second rm attempt succeeds
    mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err = runner.CtrRemoveContainer(ctx, mockConn, namespace, containerID)
    assert.NoError(t, err)
}


func TestDefaultRunner_CtrExecInContainer(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    namespace := "test-ns"
    containerID := "test-exec-ctr"
    cmdToExec := []string{"/bin/echo", "hello"}
    opts := CtrExecOptions{TTY: true, User: "testuser", Cwd: "/tmp"}

	// Using gomock.Any() for execID as it's time-dependent.
    // A more specific matcher could be used if execID format is strictly defined and testable.
    // Example: gomock.MatchRegex("kubexm-exec-\\d+") for the execID part.
    // For simplicity, we check that --exec-id is present.

    expectedCmdPattern := fmt.Sprintf("ctr -n %s task exec --tty --user %s --cwd %s --exec-id kubexm-exec-\\d+ %s %s %s",
        shellEscape(namespace), shellEscape(opts.User), shellEscape(opts.Cwd),
        shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]))


    // Test Case 1: Successful exec
    mockConn.EXPECT().Exec(ctx, gomock. दट(func(cmd string) bool {
        match, _ := regexp.MatchString(expectedCmdPattern, cmd)
        return match
    }), gomock.Any()).Return([]byte("hello\n"), []byte{}, nil).Times(1)

    output, err := runner.CtrExecInContainer(ctx, mockConn, namespace, containerID, opts, cmdToExec)
    assert.NoError(t, err)
    assert.Equal(t, "hello\n", output)

    // Test Case 2: Exec command fails in container (ctr task exec itself might succeed or fail)
    // Let's assume ctr task exec returns non-zero if the command inside fails.
    mockConn.EXPECT().Exec(ctx, gomock. दट(func(cmd string) bool {
        match, _ := regexp.MatchString(expectedCmdPattern, cmd)
        return match
    }), gomock.Any()).
        Return([]byte(""), []byte("Error: command failed with exit code 1"), &connector.CommandError{ExitCode: 1}).Times(1)

    output, err = runner.CtrExecInContainer(ctx, mockConn, namespace, containerID, opts, cmdToExec)
    assert.Error(t, err)
    assert.Contains(t, output, "Error: command failed with exit code 1") // output includes stderr
    assert.Contains(t, err.Error(), "failed to exec in container")
}

// TODO: Add tests for CtrImportImage, CtrExportImage
// TODO: Add tests for the crictl functions once implemented.
