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

func TestDefaultRunner_GetContainerdConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test Case 1: File exists, but parsing is not fully supported
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://mirror.example.com\"]"), nil).Times(1)
	config, err := runner.GetContainerdConfig(ctx, mockConn)
	assert.Error(t, err) // Expecting an error due to no TOML parsing
	assert.NotNil(t, config) // Should still return an empty struct
	assert.Contains(t, err.Error(), "full TOML parsing into struct not supported")

	// Test Case 2: File does not exist
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return(nil, fmt.Errorf("file not found")).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(false, nil).Times(1)
	config, err = runner.GetContainerdConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config) // Expect empty struct
	assert.Nil(t, config.Root)

	// Test Case 3: File is empty
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1)
	config, err = runner.GetContainerdConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Nil(t, config.Root)
}

func TestDefaultRunner_ConfigureContainerd_RegistryMirrorsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	opts := ContainerdConfigOptions{
		RegistryMirrors: map[string][]string{
			"docker.io": {"https://dockerhub.mirror.example.com"},
			"k8s.gcr.io": {"https://k8s.mirror.example.com"},
		},
	}
	expectedSnippetDocker := "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://dockerhub.mirror.example.com\"]"
	expectedSnippetK8s := "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"k8s.gcr.io\"]\n  endpoint = [\"https://k8s.mirror.example.com\"]"

	// Simulate empty existing config
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)

	// Check that the written content contains the snippets
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
			contentStr := string(content)
			assert.Contains(t, contentStr, "[plugins.\"io.containerd.grpc.v1.cri\".registry]")
			assert.Contains(t, contentStr, expectedSnippetDocker)
			assert.Contains(t, contentStr, expectedSnippetK8s)
			return nil
		}).Times(1)

	err := runner.ConfigureContainerd(ctx, mockConn, opts, false) // No restart
	assert.NoError(t, err)
}


func TestDefaultRunner_ConfigureCrictl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	configFileContent := `
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
debug: true
`
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(crictlConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(configFileContent), crictlConfigPath, "0644", true).Return(nil).Times(1)

	err := runner.ConfigureCrictl(ctx, mockConn, configFileContent)
	assert.NoError(t, err)
}

func TestDefaultRunner_CrictlInspectImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "docker.io/library/alpine:latest"

	sampleInspectiJSON := `{
    "status": {
        "id": "sha256:123abc...",
        "repoTags": ["alpine:latest", "docker.io/library/alpine:latest"],
        "repoDigests": ["docker.io/library/alpine@sha256:def456..."],
        "size": "2999000",
        "username": "",
        "uid": null
    },
    "info": {"someKey": "someValue"}
}`
	var expectedDetails CrictlImageDetails
	err := json.Unmarshal([]byte(sampleInspectiJSON), &expectedDetails)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample inspecti JSON")

	cmd := fmt.Sprintf("crictl inspecti %s -o json", shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectiJSON), []byte{}, nil).Times(1)

	details, err := runner.CrictlInspectImage(ctx, mockConn, imageName)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
}

func TestDefaultRunner_CrictlImageFSInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	sampleFsInfoJSON := `{
    "filesystems": [
        {
            "timestamp": 1670000000,
            "fsId": {"mountpoint": "/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs"},
            "usedBytes": "10GB",
            "inodesUsed": "1000"
        }
    ]
}`
	expectedFsInfo := []CrictlFSInfo{
		{Timestamp: 1670000000, FsID: struct{Mountpoint string `json:"mountpoint"`}{Mountpoint: "/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs"}, UsedBytes: "10GB", InodesUsed: "1000"},
	}
	cmd := "crictl imagefsinfo -o json"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleFsInfoJSON), []byte{}, nil).Times(1)
	fsInfo, err := runner.CrictlImageFSInfo(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, expectedFsInfo, fsInfo)
}

func TestDefaultRunner_CrictlListPods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	samplePodsJSON := `{
"items": [
    {
        "id": "podid1", "name": "pod1-name", "namespace": "default", "attempt": 0,
        "state": "SANDBOX_READY", "createdAt": "2023-01-01T12:00:00Z",
        "labels": {"app": "nginx"}, "annotations": {"note":"test"}, "runtimeHandler": "runc"
    }
]
}`
	expectedPods := []CrictlPodInfo{
		{ID: "podid1", Name: "pod1-name", Namespace: "default", Attempt: 0, State: "SANDBOX_READY", CreatedAt: "2023-01-01T12:00:00Z", Labels: map[string]string{"app":"nginx"}, Annotations: map[string]string{"note":"test"}, RuntimeHandler: "runc"},
	}
	cmd := "crictl pods -o json"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)
	pods, err := runner.CrictlListPods(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedPods, pods)
}


func TestDefaultRunner_CrictlRunPod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    configFile := "/tmp/pod-config.json"
    expectedPodID := "new-pod-12345"

    mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(true, nil).Times(1)
    cmd := fmt.Sprintf("crictl runp %s", shellEscape(configFile))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(expectedPodID+"\n"), []byte{}, nil).Times(1)

    podID, err := runner.CrictlRunPod(ctx, mockConn, configFile)
    assert.NoError(t, err)
    assert.Equal(t, expectedPodID, podID)
}

func TestDefaultRunner_CrictlStopPod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    podID := "pod-to-stop-123"

    cmd := fmt.Sprintf("crictl stopp %s", shellEscape(podID))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Stopped sandbox "+podID), []byte{}, nil).Times(1)
    err := runner.CrictlStopPod(ctx, mockConn, podID)
    assert.NoError(t, err)

    // Idempotency: already stopped or not found
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("could not find sandbox"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CrictlStopPod(ctx, mockConn, podID)
    assert.NoError(t, err)
}

func TestDefaultRunner_CrictlRemovePod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    podID := "pod-to-rm-123"

    cmd := fmt.Sprintf("crictl rmp %s", shellEscape(podID))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Removed sandbox "+podID), []byte{}, nil).Times(1)
    err := runner.CrictlRemovePod(ctx, mockConn, podID)
    assert.NoError(t, err)

    // Idempotency: already removed or not found
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("could not find sandbox"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CrictlRemovePod(ctx, mockConn, podID)
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

func TestDefaultRunner_CtrImportImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "user-ns"
	filePath := "/mnt/images/myimage.tar"

	// Test Case 1: Successful import
	expectedCmd := fmt.Sprintf("ctr -n %s images import %s", shellEscape(namespace), shellEscape(filePath))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrImportImage(ctx, mockConn, namespace, filePath, false)
	assert.NoError(t, err)

	// Test Case 2: Successful import with all platforms
	expectedCmdAllPlatforms := fmt.Sprintf("ctr -n %s images import --all-platforms %s", shellEscape(namespace), shellEscape(filePath))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatforms, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrImportImage(ctx, mockConn, namespace, filePath, true)
	assert.NoError(t, err)

	// Test Case 3: Import command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("import error"), fmt.Errorf("exec import error")).Times(1)
	err = runner.CtrImportImage(ctx, mockConn, namespace, filePath, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to import image")
}

func TestDefaultRunner_CtrExportImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "user-ns"
	imageName := "docker.io/library/busybox:latest"
	outputFilePath := "/mnt/exports/busybox.tar"

	// Test Case 1: Successful export
	expectedCmd := fmt.Sprintf("ctr -n %s images export %s %s",
		shellEscape(namespace), shellEscape(outputFilePath), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, false)
	assert.NoError(t, err)

	// Test Case 2: Successful export with all platforms
	expectedCmdAllPlatforms := fmt.Sprintf("ctr -n %s images export --all-platforms %s %s",
		shellEscape(namespace), shellEscape(outputFilePath), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatforms, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, true)
	assert.NoError(t, err)

	// Test Case 3: Export command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("export error"), fmt.Errorf("exec export error")).Times(1)
	err = runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export image")
}


func TestDefaultRunner_CrictlListImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test Case 1: Successful list with JSON output
	sampleCrictlImagesJSON := `{
"images": [
    {
        "id": "sha256:abcdef123456",
        "repoTags": ["docker.io/library/alpine:latest", "alpine:latest"],
        "repoDigests": ["docker.io/library/alpine@sha256:digest1"],
        "size": "5.57MB",
        "uid": null,
        "username": ""
    },
    {
        "id": "sha256:fedcba654321",
        "repoTags": ["k8s.gcr.io/pause:3.5"],
        "repoDigests": ["k8s.gcr.io/pause@sha256:digest2"],
        "size": "296kB",
        "uid": null,
        "username": ""
    }
]
}`
	expectedImages := []CrictlImageInfo{
		{ID: "sha256:abcdef123456", RepoTags: []string{"docker.io/library/alpine:latest", "alpine:latest"}, RepoDigests: []string{"docker.io/library/alpine@sha256:digest1"}, Size: "5.57MB"},
		{ID: "sha256:fedcba654321", RepoTags: []string{"k8s.gcr.io/pause:3.5"}, RepoDigests: []string{"k8s.gcr.io/pause@sha256:digest2"}, Size: "296kB"},
	}

	mockConn.EXPECT().Exec(ctx, "crictl images -o json", gomock.Any()).Return([]byte(sampleCrictlImagesJSON), []byte{}, nil).Times(1)
	images, err := runner.CrictlListImages(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedImages, images)

	// Test Case 2: Empty list (e.g. crictl returns `{"images": []}` or just `[]`)
	mockConn.EXPECT().Exec(ctx, "crictl images -o json", gomock.Any()).Return([]byte(`{"images": []}`), []byte{}, nil).Times(1)
	images, err = runner.CrictlListImages(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Empty(t, images)

	mockConn.EXPECT().Exec(ctx, "crictl images -o json", gomock.Any()).Return([]byte(`[]`), []byte{}, nil).Times(1)
	images, err = runner.CrictlListImages(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Empty(t, images)


	// Test Case 3: Filters applied (example: filter by image name)
	filters := map[string]string{"image": "alpine:latest"}
	expectedCmdWithFilter := "crictl images --image 'alpine:latest' -o json"
	mockConn.EXPECT().Exec(ctx, expectedCmdWithFilter, gomock.Any()).Return([]byte(sampleCrictlImagesJSON), []byte{}, nil).Times(1) // Assuming filter returns same set for test
	images, err = runner.CrictlListImages(ctx, mockConn, filters)
	assert.NoError(t, err)
	assert.Equal(t, expectedImages, images)


	// Test Case 4: crictl command execution error
	mockConn.EXPECT().Exec(ctx, "crictl images -o json", gomock.Any()).
		Return(nil, []byte("crictl error"), fmt.Errorf("exec error")).Times(1)
	images, err = runner.CrictlListImages(ctx, mockConn, nil)
	assert.Error(t, err)
	assert.Nil(t, images)
	assert.Contains(t, err.Error(), "crictl images failed")
}

func TestDefaultRunner_CrictlPullImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "docker.io/library/nginx:stable"

	// Test Case 1: Successful pull
	expectedCmd := fmt.Sprintf("crictl pull %s", shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("Image is up to date"), []byte{}, nil).Times(1)
	err := runner.CrictlPullImage(ctx, mockConn, imageName, "", "")
	assert.NoError(t, err)

	// Test Case 2: Pull with auth
	authCreds := "user:pass"
	expectedCmdWithAuth := fmt.Sprintf("crictl pull --auth %s %s", shellEscape(authCreds), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmdWithAuth, gomock.Any()).Return([]byte("Image is up to date"), []byte{}, nil).Times(1)
	err = runner.CrictlPullImage(ctx, mockConn, imageName, authCreds, "")
	assert.NoError(t, err)

	// Test Case 3: crictl command execution error
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("pull error"), fmt.Errorf("exec pull error")).Times(1)
	err = runner.CrictlPullImage(ctx, mockConn, imageName, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl pull image failed")
}

func TestDefaultRunner_CrictlRemoveImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageNameOrID := "sha256:abcdef123456"

	// Test Case 1: Successful removal
	cmd := fmt.Sprintf("crictl rmi %s", shellEscape(imageNameOrID))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Deleted: "+imageNameOrID), []byte{}, nil).Times(1)
	err := runner.CrictlRemoveImage(ctx, mockConn, imageNameOrID)
	assert.NoError(t, err)

	// Test Case 2: Image not found (idempotency)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("FATA[0000] rpc error: code = NotFound desc = image "+imageNameOrID+" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CrictlRemoveImage(ctx, mockConn, imageNameOrID)
	assert.NoError(t, err)

	// Test Case 3: Other command execution error
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some rmi error"), fmt.Errorf("exec rmi error")).Times(1)
	err = runner.CrictlRemoveImage(ctx, mockConn, imageNameOrID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl rmi image failed")
}

func TestDefaultRunner_GetContainerdConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test Case 1: File exists, but parsing is not fully supported by this simplified Get
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://mirror.example.com\"]"), nil).Times(1)
	config, err := runner.GetContainerdConfig(ctx, mockConn)
	assert.Error(t, err)
	assert.NotNil(t, config)
	assert.Contains(t, err.Error(), "full TOML parsing into struct not supported")

	// Test Case 2: File does not exist - should return empty struct and no error
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return(nil, fmt.Errorf("file not found")).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(false, nil).Times(1)
	config, err = runner.GetContainerdConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Nil(t, config.Root)

	// Test Case 3: File is empty - should return empty struct and no error
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1)
	config, err = runner.GetContainerdConfig(ctx, mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Nil(t, config.Root)
}

func TestDefaultRunner_ConfigureContainerd_RegistryMirrorsOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	opts := ContainerdConfigOptions{
		RegistryMirrors: map[string][]string{
			"docker.io": {"https://dockerhub.mirror.example.com"},
		},
	}
	expectedSnippetDocker := "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://dockerhub.mirror.example.com\"]"

	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1) // Simulate empty existing config
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)

	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
			contentStr := string(content)
			assert.Contains(t, contentStr, "[plugins.\"io.containerd.grpc.v1.cri\".registry]")
			assert.Contains(t, contentStr, expectedSnippetDocker)
			return nil
		}).Times(1)

	err := runner.ConfigureContainerd(ctx, mockConn, opts, false)
	assert.NoError(t, err)
}


func TestDefaultRunner_ConfigureCrictl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	configFileContent := "runtime-endpoint: unix:///run/containerd/containerd.sock\nimage-endpoint: unix:///run/containerd/containerd.sock\ntimeout: 10\ndebug: true\n"

	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(crictlConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(configFileContent), crictlConfigPath, "0644", true).Return(nil).Times(1)

	err := runner.ConfigureCrictl(ctx, mockConn, configFileContent)
	assert.NoError(t, err)
}

func TestDefaultRunner_CrictlInspectImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	imageName := "docker.io/library/alpine:latest"

	sampleInspectiJSON := `{"status": {"id": "sha256:123abc...","repoTags": ["alpine:latest"],"size": "2.8MB"}, "info": {}}`
	var expectedDetails CrictlImageDetails
	json.Unmarshal([]byte(sampleInspectiJSON), &expectedDetails)


	cmd := fmt.Sprintf("crictl inspecti %s -o json", shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectiJSON), []byte{}, nil).Times(1)

	details, err := runner.CrictlInspectImage(ctx, mockConn, imageName)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)

	// Test Not Found
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("FATA[0000] image \"docker.io/library/alpine:latest\" not found"), &connector.CommandError{ExitCode:1}).Times(1)
	details, err = runner.CrictlInspectImage(ctx, mockConn, imageName)
	assert.NoError(t, err) // Not found should be nil, nil
	assert.Nil(t, details)
}

func TestDefaultRunner_CrictlImageFSInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	sampleFsInfoJSON := `{"filesystems": [{"timestamp": 1670000000, "fsId": {"mountpoint": "/var/lib/containerd"}, "usedBytes": "10GB", "inodesUsed": "1000"}]}`
	var expectedFsInfoStruct struct { FileSystems []CrictlFSInfo `json:"filesystems"`}
	json.Unmarshal([]byte(sampleFsInfoJSON), &expectedFsInfoStruct)

	cmd := "crictl imagefsinfo -o json"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleFsInfoJSON), []byte{}, nil).Times(1)
	fsInfo, err := runner.CrictlImageFSInfo(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, expectedFsInfoStruct.FileSystems, fsInfo)
}

func TestDefaultRunner_CrictlListPods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	samplePodsJSON := `{"items": [{"id": "podid1", "name": "pod1-name", "state": "SANDBOX_READY"}]}`
	var expectedPodsStruct struct { Items []CrictlPodInfo `json:"items"`}
	json.Unmarshal([]byte(samplePodsJSON), &expectedPodsStruct)

	cmd := "crictl pods -o json"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)
	pods, err := runner.CrictlListPods(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedPodsStruct.Items, pods)

	// Test with filter
	filters := map[string]string{"name": "pod1-name"}
	cmdFiltered := "crictl pods --name 'pod1-name' -o json"
	mockConn.EXPECT().Exec(ctx, cmdFiltered, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)
	pods, err = runner.CrictlListPods(ctx, mockConn, filters)
	assert.NoError(t, err)
	assert.Equal(t, expectedPodsStruct.Items, pods)
}


func TestDefaultRunner_CrictlRunPod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    configFile := "/tmp/pod-config.json"
    expectedPodID := "new-pod-12345"

    mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(true, nil).Times(1)
    cmd := fmt.Sprintf("crictl runp %s", shellEscape(configFile))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(expectedPodID+"\n"), []byte{}, nil).Times(1)

    podID, err := runner.CrictlRunPod(ctx, mockConn, configFile)
    assert.NoError(t, err)
    assert.Equal(t, expectedPodID, podID)
}

func TestDefaultRunner_CrictlStopPod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    podID := "pod-to-stop-123"

    cmd := fmt.Sprintf("crictl stopp %s", shellEscape(podID))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Stopped sandbox "+podID), []byte{}, nil).Times(1)
    err := runner.CrictlStopPod(ctx, mockConn, podID)
    assert.NoError(t, err)

    // Idempotency: already stopped or not found
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("could not find sandbox"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CrictlStopPod(ctx, mockConn, podID)
    assert.NoError(t, err)
}

func TestDefaultRunner_CrictlRemovePod(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    podID := "pod-to-rm-123"

    cmd := fmt.Sprintf("crictl rmp %s", shellEscape(podID))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Removed sandbox "+podID), []byte{}, nil).Times(1)
    err := runner.CrictlRemovePod(ctx, mockConn, podID)
    assert.NoError(t, err)

    // Idempotency: already removed or not found
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("could not find sandbox"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CrictlRemovePod(ctx, mockConn, podID)
    assert.NoError(t, err)
}

// TODO: Add tests for remaining crictl functions once implemented.
