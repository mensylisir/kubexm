package runner

import (
	"context"
	"encoding/json" // Added for Crictl tests
	"fmt"
	"path/filepath" // Added for config tests
	"regexp"        // Added for CtrContainerInfo test
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func TestDefaultRunner_CtrListNamespaces(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	expectedNamespaces := []string{"default", "k8s.io", "moby"}
	stdout := strings.Join(expectedNamespaces, "\n") + "\n"
	mockConn.EXPECT().Exec(ctx, "ctr ns ls -q", gomock.Any()).Return([]byte(stdout), []byte{}, nil).Times(1)

	namespaces, err := runner.CtrListNamespaces(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, expectedNamespaces, namespaces)

	mockConn.EXPECT().Exec(ctx, "ctr ns ls -q", gomock.Any()).Return([]byte(""), []byte{}, nil).Times(1)
	namespaces, err = runner.CtrListNamespaces(ctx, mockConn)
	assert.NoError(t, err)
	assert.Empty(t, namespaces)

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

	tabularOutput := `REF                                                         TYPE                                                 DIGEST                                                                  SIZE      PLATFORMS        LABELS
docker.io/library/alpine:latest                             application/vnd.docker.distribution.manifest.v2+json sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300 2.83 MiB  linux/amd64      foo=bar
k8s.gcr.io/pause:3.5                                        application/vnd.docker.distribution.manifest.v2+json sha256:221177c60ce5107572697c109b00c6e9415809cfe0510b5a9800334731ffa9f7 303 KiB   linux/amd64,linux/arm64 -
`
	expectedImages := []CtrImageInfo{
		{Name: "docker.io/library/alpine:latest", Digest: "sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300", Size: "2.83 MiB", OSArch: "linux/amd64"},
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

	headerOnlyOutput := "REF DIGEST TYPE SIZE PLATFORMS LABELS\n"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(headerOnlyOutput), []byte{}, nil).Times(1)
	images, err = runner.CtrListImages(ctx, mockConn, namespace)
	assert.NoError(t, err)
	assert.Empty(t, images)

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

	expectedCmd := fmt.Sprintf("ctr -n %s images pull %s", shellEscape(namespace), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("...output..."), []byte{}, nil).Times(1)
	err := runner.CtrPullImage(ctx, mockConn, namespace, imageName, false, "")
	assert.NoError(t, err)

	user := "testuser:testpass"
	expectedCmdAllPlatUser := fmt.Sprintf("ctr -n %s images pull --all-platforms --user %s %s",
		shellEscape(namespace), shellEscape(user), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatUser, gomock.Any()).Return([]byte("...output..."), []byte{}, nil).Times(1)
	err = runner.CtrPullImage(ctx, mockConn, namespace, imageName, true, user)
	assert.NoError(t, err)

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
	cmd := fmt.Sprintf("ctr -n %s images rm %s", shellEscape(namespace), shellEscape(imageName))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrRemoveImage(ctx, mockConn, namespace, imageName)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte(`ctr: image "docker.io/library/alpine:latest": not found`), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CtrRemoveImage(ctx, mockConn, namespace, imageName)
	assert.NoError(t, err)

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
	cmd := fmt.Sprintf("ctr -n %s images tag %s %s", shellEscape(namespace), shellEscape(sourceImage), shellEscape(targetImage))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrTagImage(ctx, mockConn, namespace, sourceImage, targetImage)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("tagging error"), fmt.Errorf("exec tag error")).Times(1)
	err = runner.CtrTagImage(ctx, mockConn, namespace, sourceImage, targetImage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to tag image")
}

func TestDefaultRunner_CtrRunContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "test-ns"
	opts := ContainerdContainerCreateOptions{
		ImageName:      "docker.io/library/nginx:latest",
		ContainerID:    "test-nginx-ctr",
		NetHost:        true,
		TTY:            true,
		Env:            []string{"FOO=bar", "BAZ=qux"},
		Command:        []string{"nginx", "-g", "daemon off;"},
		RemoveExisting: true,
	}
	rmCmd := fmt.Sprintf("ctr -n %s containers rm %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).Return(nil, []byte{}, nil).AnyTimes()
	stopCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	stopCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(opts.ContainerID))
	mockConn.EXPECT().Exec(ctx, stopCmdTerm, gomock.Any()).Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).AnyTimes()
	mockConn.EXPECT().Exec(ctx, stopCmdKill, gomock.Any()).Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).AnyTimes()
	expectedCmdParts := []string{
		"ctr", "-n", shellEscape(namespace), "run", "--net-host", "--tty",
		"--env", shellEscape("FOO=bar"), "--env", shellEscape("BAZ=qux"), "--rm",
		shellEscape(opts.ImageName), shellEscape(opts.ContainerID), shellEscape("nginx"), shellEscape("-g"), shellEscape("daemon off;"),
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

func TestDefaultRunner_CtrListContainers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "test-ns"
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

	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
	err := runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("some error on sigterm"), fmt.Errorf("sigterm failed")).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("sigterm general error"), fmt.Errorf("sigterm general error")).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("sigkill general error"), fmt.Errorf("sigkill general error")).Times(1)
	err = runner.CtrStopContainer(ctx, mockConn, namespace, containerID, 0*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to SIGKILL task")

	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
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

	mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrRemoveContainer(ctx, mockConn, namespace, containerID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).
		Return(nil, []byte("ctr: container \"test-container-rm\": not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CtrRemoveContainer(ctx, mockConn, namespace, containerID)
	assert.NoError(t, err)

	killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(containerID))
	killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, rmCmd, gomock.Any()).
		Return(nil, []byte("ctr: container has active task, please stop task before removing container or use --force"), &connector.CommandError{ExitCode: 1}).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdTerm, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
	mockConn.EXPECT().Exec(ctx, killCmdKill, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
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
	expectedCmdPattern := fmt.Sprintf("ctr -n %s task exec --tty --user %s --cwd %s --exec-id kubexm-exec-\\d+ %s %s %s",
		shellEscape(namespace), shellEscape(opts.User), shellEscape(opts.Cwd),
		shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]))

	mockConn.EXPECT().Exec(ctx, gomock.That(func(cmd string) bool {
		match, _ := regexp.MatchString(expectedCmdPattern, cmd)
		return match
	}), gomock.Any()).Return([]byte("hello\n"), []byte{}, nil).Times(1)
	output, err := runner.CtrExecInContainer(ctx, mockConn, namespace, containerID, opts, cmdToExec)
	assert.NoError(t, err)
	assert.Equal(t, "hello\n", output)

	mockConn.EXPECT().Exec(ctx, gomock.That(func(cmd string) bool {
		match, _ := regexp.MatchString(expectedCmdPattern, cmd)
		return match
	}), gomock.Any()).
		Return([]byte(""), []byte("Error: command failed with exit code 1"), &connector.CommandError{ExitCode: 1}).Times(1)
	output, err = runner.CtrExecInContainer(ctx, mockConn, namespace, containerID, opts, cmdToExec)
	assert.Error(t, err)
	assert.Contains(t, output, "Error: command failed with exit code 1")
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

	expectedCmd := fmt.Sprintf("ctr -n %s images import %s", shellEscape(namespace), shellEscape(filePath))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrImportImage(ctx, mockConn, namespace, filePath, false)
	assert.NoError(t, err)

	expectedCmdAllPlatforms := fmt.Sprintf("ctr -n %s images import --all-platforms %s", shellEscape(namespace), shellEscape(filePath))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatforms, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrImportImage(ctx, mockConn, namespace, filePath, true)
	assert.NoError(t, err)

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

	expectedCmd := fmt.Sprintf("ctr -n %s images export %s %s",
		shellEscape(namespace), shellEscape(outputFilePath), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, false)
	assert.NoError(t, err)

	expectedCmdAllPlatforms := fmt.Sprintf("ctr -n %s images export --all-platforms %s %s",
		shellEscape(namespace), shellEscape(outputFilePath), shellEscape(imageName))
	mockConn.EXPECT().Exec(ctx, expectedCmdAllPlatforms, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, true)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("export error"), fmt.Errorf("exec export error")).Times(1)
	err = runner.CtrExportImage(ctx, mockConn, namespace, imageName, outputFilePath, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export image")
}

func TestDefaultRunner_CtrContainerInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	r := NewDefaultRunner()
	ctx := context.Background()
	namespace := "test-ns"
	containerID := "test-ctr-info"
	infoOutput := `
Image: docker.io/library/nginx:latest
Runtime: io.containerd.runc.v2 args=[]
Snapshotter: overlayfs
Labels:
  label1: value1
  "label.with.dots": "value with spaces"
Spec: (omitted)
`
	taskPsCmd := fmt.Sprintf("ctr -n %s task ps %s", shellEscape(namespace), shellEscape(containerID))
	infoCmd := fmt.Sprintf("ctr -n %s container info %s", shellEscape(namespace), shellEscape(containerID))

	mockConn.EXPECT().Exec(ctx, infoCmd, gomock.Any()).Return([]byte(infoOutput), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, taskPsCmd, gomock.Any()).Return([]byte("PID STATUS\n123 RUNNING"), []byte{}, nil).Times(1)
	info, err := r.CtrContainerInfo(ctx, mockConn, namespace, containerID)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, containerID, info.ID)
	assert.Equal(t, "docker.io/library/nginx:latest", info.Image)
	assert.Equal(t, "io.containerd.runc.v2", info.Runtime)
	assert.Equal(t, "RUNNING", info.Status)
	assert.Equal(t, "value1", info.Labels["label1"])
	assert.Equal(t, "value with spaces", info.Labels["label.with.dots"])

	mockConn.EXPECT().Exec(ctx, infoCmd, gomock.Any()).Return([]byte(infoOutput), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, taskPsCmd, gomock.Any()).
		Return(nil, []byte("no such process"), &connector.CommandError{ExitCode: 1}).Times(1)
	info, err = r.CtrContainerInfo(ctx, mockConn, namespace, containerID)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "STOPPED", info.Status)

	mockConn.EXPECT().Exec(ctx, infoCmd, gomock.Any()).
		Return(nil, []byte("ctr: container \"test-ctr-info\": not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	info, err = r.CtrContainerInfo(ctx, mockConn, namespace, containerID)
	assert.NoError(t, err)
	assert.Nil(t, info)
}

// --- Original tests for config and crictl that were part of the duplicated block ---
// --- These should be the single source of truth for these tests now ---

func TestDefaultRunner_GetContainerdConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	// Test Case 1: File exists, but parsing is not fully supported by this simplified Get
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://mirror.example.com\"]"), nil).Times(1)
	config, err := runner.GetContainerdConfig(ctx, mockConn)
	// This test acknowledges the current limitation of GetContainerdConfig (no real TOML parsing).
	// If TOML parsing is added, this test should be updated to expect successful parsing.
	assert.Error(t, err, "Expected an error due to simplified TOML parsing")
	assert.NotNil(t, config)
	if err != nil {
		assert.Contains(t, err.Error(), "full TOML parsing into struct not supported")
	}

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
			"docker.io":  {"https://dockerhub.mirror.example.com"},
			"k8s.gcr.io": {"https://k8s.mirror.example.com"},
		},
	}
	expectedSnippetDocker := "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]\n  endpoint = [\"https://dockerhub.mirror.example.com\"]"
	expectedSnippetK8s := "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"k8s.gcr.io\"]\n  endpoint = [\"https://k8s.mirror.example.com\"]"
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
			contentStr := string(content)
			assert.Contains(t, contentStr, "[plugins.\"io.containerd.grpc.v1.cri\".registry]")
			assert.Contains(t, contentStr, expectedSnippetDocker)
			assert.Contains(t, contentStr, expectedSnippetK8s)
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

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("FATA[0000] image \"docker.io/library/alpine:latest\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	details, err = runner.CrictlInspectImage(ctx, mockConn, imageName)
	assert.NoError(t, err)
	assert.Nil(t, details)
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
		{Timestamp: 1670000000, FsID: struct {
			Mountpoint string `json:"mountpoint"`
		}{Mountpoint: "/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs"}, UsedBytes: "10GB", InodesUsed: "1000"},
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
		{ID: "podid1", Name: "pod1-name", Namespace: "default", Attempt: 0, State: "SANDBOX_READY", CreatedAt: "2023-01-01T12:00:00Z", Labels: map[string]string{"app": "nginx"}, Annotations: map[string]string{"note": "test"}, RuntimeHandler: "runc"},
	}
	cmd := "crictl pods -o json"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)
	pods, err := runner.CrictlListPods(ctx, mockConn, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedPods, pods)

	filters := map[string]string{"name": "pod1-name"}
	cmdFiltered := "crictl pods --name 'pod1-name' -o json"
	mockConn.EXPECT().Exec(ctx, cmdFiltered, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)
	pods, err = runner.CrictlListPods(ctx, mockConn, filters)
	assert.NoError(t, err)
	assert.Equal(t, expectedPods, pods)
}

// --- New crictl tests start here ---

func TestDefaultRunner_CrictlPodSandboxStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-pod-status-id"
	cmd := fmt.Sprintf("crictl inspectp %s -o json", shellEscape(podID))
	sampleInspectPJSON := `{"status": {"id": "test-pod-status-id", "state": "SANDBOX_READY"}, "info": {}}`
	var expectedDetails CrictlPodDetails
	err := json.Unmarshal([]byte(sampleInspectPJSON), &expectedDetails)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample inspectp JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectPJSON), []byte{}, nil).Times(1)
	details, err := runner.CrictlPodSandboxStatus(ctx, mockConn, podID, true)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectPJSON), []byte{}, nil).Times(1)
	details, err = runner.CrictlPodSandboxStatus(ctx, mockConn, podID, false)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
}

func TestDefaultRunner_CrictlStartContainerInPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-container-start-id"
	cmd := fmt.Sprintf("crictl start %s", shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(containerID), []byte{}, nil).Times(1)
	err := runner.CrictlStartContainerInPod(ctx, mockConn, containerID)
	assert.NoError(t, err)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("start error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CrictlStartContainerInPod(ctx, mockConn, containerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl start test-container-start-id failed")
}

func TestDefaultRunner_CrictlStopContainerInPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-container-stop-id"
	timeout := int64(30)
	cmdWithTimeout := fmt.Sprintf("crictl stop --timeout %d %s", timeout, shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdWithTimeout, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CrictlStopContainerInPod(ctx, mockConn, containerID, timeout)
	assert.NoError(t, err)
	cmdWithoutTimeout := fmt.Sprintf("crictl stop --timeout 0 %s", shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdWithoutTimeout, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CrictlStopContainerInPod(ctx, mockConn, containerID, 0)
	assert.NoError(t, err)
	mockConn.EXPECT().Exec(ctx, cmdWithoutTimeout, gomock.Any()).
		Return(nil, []byte("Container \"test-container-stop-id\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CrictlStopContainerInPod(ctx, mockConn, containerID, 0)
	assert.NoError(t, err)
	mockConn.EXPECT().Exec(ctx, cmdWithoutTimeout, gomock.Any()).
		Return(nil, []byte("some stop error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CrictlStopContainerInPod(ctx, mockConn, containerID, 0)
	assert.Error(t, err)
}

func TestDefaultRunner_CrictlRemoveContainerInPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-container-rm-id"
	cmdNoForce := fmt.Sprintf("crictl rm %s", shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdNoForce, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CrictlRemoveContainerInPod(ctx, mockConn, containerID, false)
	assert.NoError(t, err)
	cmdWithForce := fmt.Sprintf("crictl rm -f %s", shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdWithForce, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.CrictlRemoveContainerInPod(ctx, mockConn, containerID, true)
	assert.NoError(t, err)
	mockConn.EXPECT().Exec(ctx, cmdNoForce, gomock.Any()).
		Return(nil, []byte("Container \"test-container-rm-id\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CrictlRemoveContainerInPod(ctx, mockConn, containerID, false)
	assert.NoError(t, err)
	mockConn.EXPECT().Exec(ctx, cmdNoForce, gomock.Any()).
		Return(nil, []byte("some rm error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CrictlRemoveContainerInPod(ctx, mockConn, containerID, false)
	assert.Error(t, err)
}

func TestDefaultRunner_CrictlInspectContainerInPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-inspect-container-id"
	cmd := fmt.Sprintf("crictl inspect %s -o json", shellEscape(containerID))
	sampleInspectJSON := `{"status": {"id": "test-inspect-container-id", "metadata": {"name": "my-container"}, "state": "CONTAINER_RUNNING"}, "pid": 1234, "info": {}}`
	var expectedDetails CrictlContainerDetails
	err := json.Unmarshal([]byte(sampleInspectJSON), &expectedDetails)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample inspect JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectJSON), []byte{}, nil).Times(1)
	details, err := runner.CrictlInspectContainerInPod(ctx, mockConn, containerID)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("Container \"test-inspect-container-id\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	details, err = runner.CrictlInspectContainerInPod(ctx, mockConn, containerID)
	assert.NoError(t, err)
	assert.Nil(t, details)
}

func TestDefaultRunner_CrictlContainerStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-container-status-id"
	cmd := fmt.Sprintf("crictl inspect %s -o json", shellEscape(containerID))
	sampleInspectJSON := `{"status": {"id": "test-container-status-id", "state": "CONTAINER_EXITED"}, "pid": 0, "info": {}}`
	var expectedDetails CrictlContainerDetails
	err := json.Unmarshal([]byte(sampleInspectJSON), &expectedDetails)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample inspect JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectJSON), []byte{}, nil).Times(1)
	details, err := runner.CrictlContainerStatus(ctx, mockConn, containerID, true)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
}

func TestDefaultRunner_CrictlLogsForContainer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-logs-container-id"
	expectedLogs := "Log line 1\nLog line 2"
	cmdBasic := fmt.Sprintf("crictl logs %s", shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdBasic, gomock.Any()).Return([]byte(expectedLogs), []byte{}, nil).Times(1)
	logs, err := runner.CrictlLogsForContainer(ctx, mockConn, containerID, CrictlLogOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)
	tailLines := int64(10)
	opts := CrictlLogOptions{TailLines: &tailLines, Timestamps: true}
	cmdWithOpts := fmt.Sprintf("crictl logs --timestamps --tail %d %s", tailLines, shellEscape(containerID))
	mockConn.EXPECT().Exec(ctx, cmdWithOpts, gomock.Any()).Return([]byte(expectedLogs), []byte{}, nil).Times(1)
	logs, err = runner.CrictlLogsForContainer(ctx, mockConn, containerID, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)
}

func TestDefaultRunner_CrictlExecInContainerSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	containerID := "test-execsync-id"
	cmdToExec := []string{"ls", "-l", "/tmp"}
	expectedStdout := "total 0\n-rw-r--r-- 1 root root 0 Jan 1 00:00 file.txt"
	expectedStderr := ""
	cmd := fmt.Sprintf("crictl exec -s %s %s %s %s", shellEscape(containerID), shellEscape(cmdToExec[0]), shellEscape(cmdToExec[1]), shellEscape(cmdToExec[2]))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(expectedStdout), []byte(expectedStderr), nil).Times(1)
	stdout, stderr, err := runner.CrictlExecInContainerSync(ctx, mockConn, containerID, 0, cmdToExec)
	assert.NoError(t, err)
	assert.Equal(t, expectedStdout, stdout)
	assert.Equal(t, expectedStderr, stderr)
	expectedErrStderr := "ls: cannot access '/tmp/nonexistent': No such file or directory"
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte(expectedErrStderr), &connector.CommandError{ExitCode: 2}).Times(1)
	stdout, stderr, err = runner.CrictlExecInContainerSync(ctx, mockConn, containerID, 0, cmdToExec)
	assert.Error(t, err)
	assert.Equal(t, "", stdout)
	assert.Equal(t, expectedErrStderr, stderr)
	assert.Contains(t, err.Error(), "crictl exec sync in test-execsync-id failed")
}

func TestDefaultRunner_CrictlPortForward(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-portforward-pod"
	ports := []string{"8080:80", "9090:90"}
	cmd := fmt.Sprintf("crictl port-forward %s %s %s", shellEscape(podID), shellEscape(ports[0]), shellEscape(ports[1]))
	cancellableCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	mockConn.EXPECT().Exec(cancellableCtx, cmd, gomock.Any()).Return(nil, nil, context.DeadlineExceeded).Times(1)
	_, err := runner.CrictlPortForward(cancellableCtx, mockConn, podID, ports)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CrictlPortForward started but command is long-running")
}

func TestDefaultRunner_CrictlVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	cmd := "crictl version -o json"
	sampleVersionJSON := `{"version": "v1.25.0", "runtimeName": "containerd", "runtimeVersion": "v1.6.8", "runtimeApiVersion": "v1"}`
	var expectedVersion CrictlVersionInfo
	err := json.Unmarshal([]byte(sampleVersionJSON), &expectedVersion)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample version JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleVersionJSON), []byte{}, nil).Times(1)
	version, err := runner.CrictlVersion(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, &expectedVersion, version)
}

func TestDefaultRunner_CrictlInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	cmd := "crictl info -o json"
	sampleInfoJSON := `{"config": {"containerd": {"snapshotter": "overlayfs"}}, "status": {"someRuntimeStatus": "ok"}}`
	var expectedInfo CrictlRuntimeInfo
	err := json.Unmarshal([]byte(sampleInfoJSON), &expectedInfo)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample info JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInfoJSON), []byte{}, nil).Times(1)
	info, err := runner.CrictlInfo(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, &expectedInfo, info)
}

func TestDefaultRunner_CrictlStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	resourceID := "res-id-123"
	expectedStatsJSON := `{"cpu": {"usageCoreNanoSeconds": "100"}, "memory": {"workingSetBytes": "2048"}}`
	cmdJSON := fmt.Sprintf("crictl stats -o json %s", shellEscape(resourceID))
	mockConn.EXPECT().Exec(ctx, cmdJSON, gomock.Any()).Return([]byte(expectedStatsJSON), []byte{}, nil).Times(1)
	stats, err := runner.CrictlStats(ctx, mockConn, resourceID, "json")
	assert.NoError(t, err)
	assert.JSONEq(t, expectedStatsJSON, stats)
	cmdAllDefault := "crictl stats"
	expectedDefaultOutput := "CONTAINER CPU MEMORY...\n..."
	mockConn.EXPECT().Exec(ctx, cmdAllDefault, gomock.Any()).Return([]byte(expectedDefaultOutput), []byte{}, nil).Times(1)
	stats, err = runner.CrictlStats(ctx, mockConn, "", "")
	assert.NoError(t, err)
	assert.Equal(t, expectedDefaultOutput, stats)
}

func TestDefaultRunner_CrictlPodStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "pod-stats-id"
	expectedPodStatsJSON := `{"cpu": {"usageCoreNanoSeconds": "500"}, "memory": {"workingSetBytes": "10240"}}`
	cmd := fmt.Sprintf("crictl stats -o json %s", shellEscape(podID))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(expectedPodStatsJSON), []byte{}, nil).Times(1)
	stats, err := runner.CrictlPodStats(ctx, mockConn, "json", podID)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedPodStatsJSON, stats)
}

func TestDefaultRunner_EnsureDefaultContainerdConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	defaultFact := &Facts{
		OS:             &connector.OS{ID: "linux", VersionID: "test", Arch: "amd64", Kernel: "mock-kernel"},
		InitSystem:     &ServiceInfo{Type: InitSystemSystemd, RestartCmd: "systemctl restart %s"},
		PackageManager: &PackageInfo{Type: PackageManagerApt},
	}

	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(false, nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
			contentStr := string(content)
			assert.Contains(t, contentStr, "version = 2")
			assert.Contains(t, contentStr, "[plugins.\"io.containerd.grpc.v1.cri\"]")
			assert.Contains(t, contentStr, "SystemdCgroup = true")
			return nil
		}).Times(1)
	mockConn.EXPECT().Exec(ctx, "systemctl restart containerd", gomock.Any()).Return(nil, []byte{}, nil).Times(1)

	err := runner.EnsureDefaultContainerdConfig(ctx, mockConn, defaultFact)
	assert.NoError(t, err)

	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(true, nil).Times(1)
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(""), nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).Return(nil).Times(1)
	mockConn.EXPECT().Exec(ctx, "systemctl restart containerd", gomock.Any()).Return(nil, []byte{}, nil).Times(1)

	err = runner.EnsureDefaultContainerdConfig(ctx, mockConn, defaultFact)
	assert.NoError(t, err)

	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(true, nil).Times(1)
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte("version = 2"), nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).Return(nil).Times(1)
	mockConn.EXPECT().Exec(ctx, "systemctl restart containerd", gomock.Any()).Return(nil, []byte{}, nil).Times(1)

	err = runner.EnsureDefaultContainerdConfig(ctx, mockConn, defaultFact)
	assert.NoError(t, err)

	existingConfig := `version = 2
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "my.custom.pause/pause:3.8"
`
	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(true, nil).Times(1)
	mockConn.EXPECT().ReadFile(ctx, mockConn, containerdConfigPath).Return([]byte(existingConfig), nil).Times(1)
	err = runner.EnsureDefaultContainerdConfig(ctx, mockConn, defaultFact)
	assert.NoError(t, err)

	nonSystemdFact := &Facts{
		OS:             &connector.OS{ID: "linux"},
		InitSystem:     &ServiceInfo{Type: InitSystemSysV, RestartCmd: "service containerd restart"},
		PackageManager: &PackageInfo{Type: PackageManagerYum},
	}
	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(false, nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, _ string, _ string, _ bool) error {
			contentStr := string(content)
			assert.Contains(t, contentStr, "SystemdCgroup = false")
			return nil
		}).Times(1)
	mockConn.EXPECT().Exec(ctx, "service containerd restart", gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.EnsureDefaultContainerdConfig(ctx, mockConn, nonSystemdFact)
	assert.NoError(t, err)

	mockConn.EXPECT().Exists(ctx, mockConn, containerdConfigPath).Return(false, nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(containerdConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), containerdConfigPath, "0644", true).Return(nil).Times(1)
	mockConn.EXPECT().Exec(ctx, "systemctl restart containerd", gomock.Any()).Return(nil, []byte("failed"), fmt.Errorf("restart failed")).Times(1)
	mockConn.EXPECT().Exec(ctx, "systemctl restart containerd.service", gomock.Any()).Return(nil, []byte{}, nil).Times(1)

	err = runner.EnsureDefaultContainerdConfig(ctx, mockConn, defaultFact)
	assert.NoError(t, err)
}

func TestDefaultRunner_EnsureDefaultCrictlConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	expectedDefaultContent := `runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10 # seconds
debug: false
pull-image-on-create: false
`
	mockConn.EXPECT().Exists(ctx, mockConn, crictlConfigPath).Return(false, nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(crictlConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(expectedDefaultContent), crictlConfigPath, "0644", true).Return(nil).Times(1)
	err := runner.EnsureDefaultCrictlConfig(ctx, mockConn)
	assert.NoError(t, err)

	mockConn.EXPECT().Exists(ctx, mockConn, crictlConfigPath).Return(true, nil).Times(1)
	mockConn.EXPECT().ReadFile(ctx, mockConn, crictlConfigPath).Return([]byte(""), nil).Times(1)
	mockConn.EXPECT().Mkdirp(ctx, mockConn, filepath.Dir(crictlConfigPath), "0755", true).Return(nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(expectedDefaultContent), crictlConfigPath, "0644", true).Return(nil).Times(1)
	err = runner.EnsureDefaultCrictlConfig(ctx, mockConn)
	assert.NoError(t, err)

	existingConfig := "runtime-endpoint: unix:///var/run/crio/crio.sock\n"
	mockConn.EXPECT().Exists(ctx, mockConn, crictlConfigPath).Return(true, nil).Times(1)
	mockConn.EXPECT().ReadFile(ctx, mockConn, crictlConfigPath).Return([]byte(existingConfig), nil).Times(1)
	err = runner.EnsureDefaultCrictlConfig(ctx, mockConn)
	assert.NoError(t, err)
}

func TestDefaultRunner_CrictlRunPodSandbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	configFile := "/tmp/pod-config.json"
	runtimeHandler := "runc"
	expectedPodID := "test-pod-id-123"

	mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(true, nil).Times(1)
	cmdWithRuntime := fmt.Sprintf("crictl runp --runtime %s %s", shellEscape(runtimeHandler), shellEscape(configFile))
	mockConn.EXPECT().Exec(ctx, cmdWithRuntime, gomock.Any()).Return([]byte(expectedPodID+"\n"), []byte{}, nil).Times(1)
	podID, err := runner.CrictlRunPodSandbox(ctx, mockConn, configFile, runtimeHandler)
	assert.NoError(t, err)
	assert.Equal(t, expectedPodID, podID)

	mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(true, nil).Times(1)
	cmdWithoutRuntime := fmt.Sprintf("crictl runp %s", shellEscape(configFile))
	mockConn.EXPECT().Exec(ctx, cmdWithoutRuntime, gomock.Any()).Return([]byte(expectedPodID+"\n"), []byte{}, nil).Times(1)
	podID, err = runner.CrictlRunPodSandbox(ctx, mockConn, configFile, "")
	assert.NoError(t, err)
	assert.Equal(t, expectedPodID, podID)

	mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(false, nil).Times(1)
	_, err = runner.CrictlRunPodSandbox(ctx, mockConn, configFile, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	mockConn.EXPECT().Exists(ctx, mockConn, configFile).Return(true, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, cmdWithoutRuntime, gomock.Any()).Return(nil, []byte("error running pod"), fmt.Errorf("exec error")).Times(1)
	_, err = runner.CrictlRunPodSandbox(ctx, mockConn, configFile, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl runp failed")
}

func TestDefaultRunner_CrictlStopPodSandbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-pod-stop-id"
	cmd := fmt.Sprintf("crictl stopp %s", shellEscape(podID))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Stopped sandbox "+podID), []byte{}, nil).Times(1)
	err := runner.CrictlStopPodSandbox(ctx, mockConn, podID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("WARN[0000] StopPodSandbox Request for ID \"test-pod-stop-id\" failed: rpc error: code = NotFound desc = PodSandbox with ID \"test-pod-stop-id\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CrictlStopPodSandbox(ctx, mockConn, podID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some other error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CrictlStopPodSandbox(ctx, mockConn, podID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl stopp test-pod-stop-id failed")
}

func TestDefaultRunner_CrictlRemovePodSandbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-pod-rm-id"
	cmd := fmt.Sprintf("crictl rmp %s", shellEscape(podID))

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("Removed sandbox "+podID), []byte{}, nil).Times(1)
	err := runner.CrictlRemovePodSandbox(ctx, mockConn, podID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("WARN[0000] RemovePodSandbox Request for ID \"test-pod-rm-id\" failed: rpc error: code = NotFound desc = PodSandbox with ID \"test-pod-rm-id\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.CrictlRemovePodSandbox(ctx, mockConn, podID)
	assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some other rmp error"), fmt.Errorf("exec error")).Times(1)
	err = runner.CrictlRemovePodSandbox(ctx, mockConn, podID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl rmp test-pod-rm-id failed")
}

func TestDefaultRunner_CrictlInspectPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-inspect-pod"
	cmd := fmt.Sprintf("crictl inspectp %s -o json", shellEscape(podID))
	sampleInspectPJSON := `{"status": {"id": "test-inspect-pod", "metadata": {"name": "my-pod"}, "state": "SANDBOX_READY"}, "info": {}}`
	var expectedDetails CrictlPodDetails
	err := json.Unmarshal([]byte(sampleInspectPJSON), &expectedDetails)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample inspectp JSON")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleInspectPJSON), []byte{}, nil).Times(1)
	details, err := runner.CrictlInspectPod(ctx, mockConn, podID)
	assert.NoError(t, err)
	assert.Equal(t, &expectedDetails, details)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("pod sandbox \"test-inspect-pod\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	details, err = runner.CrictlInspectPod(ctx, mockConn, podID)
	assert.NoError(t, err)
	assert.Nil(t, details)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte("this is not json"), []byte{}, nil).Times(1)
	details, err = runner.CrictlInspectPod(ctx, mockConn, podID)
	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "failed to parse crictl inspectp JSON")
}

func TestDefaultRunner_CrictlCreateContainerInPod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podID := "test-pod-for-container"
	containerConfigFile := "/tmp/container-config.json"
	podSandboxConfigFile := "/tmp/pod-sandbox-config.json"
	expectedContainerID := "new-container-id-456"
	cmd := fmt.Sprintf("crictl create %s %s %s", shellEscape(podID), shellEscape(containerConfigFile), shellEscape(podSandboxConfigFile))
	mockConn.EXPECT().Exists(ctx, mockConn, containerConfigFile).Return(true, nil).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, podSandboxConfigFile).Return(true, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(expectedContainerID+"\n"), []byte{}, nil).Times(1)
	containerID, err := runner.CrictlCreateContainerInPod(ctx, mockConn, podID, containerConfigFile, podSandboxConfigFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedContainerID, containerID)
	mockConn.EXPECT().Exists(ctx, mockConn, containerConfigFile).Return(false, nil).Times(1)
	_, err = runner.CrictlCreateContainerInPod(ctx, mockConn, podID, containerConfigFile, podSandboxConfigFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	mockConn.EXPECT().Exists(ctx, mockConn, containerConfigFile).Return(true, nil).Times(1)
	mockConn.EXPECT().Exists(ctx, mockConn, podSandboxConfigFile).Return(true, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("error creating container"), fmt.Errorf("exec error")).Times(1)
	_, err = runner.CrictlCreateContainerInPod(ctx, mockConn, podID, containerConfigFile, podSandboxConfigFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crictl create container in pod")
}
