package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
)

func TestDefaultRunner_KubectlApply(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	filePath := "/tmp/deployment.yaml"
	namespace := "my-namespace"
	content := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx-deployment"

	// Test Case 1: Apply with filename
	opts1 := KubectlApplyOptions{Filenames: []string{filePath}, Namespace: namespace, Sudo: false}
	expectedCmd1 := fmt.Sprintf("kubectl apply -f %s --namespace %s", shellEscape(filePath), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("deployment.apps/nginx-deployment created"), []byte{}, nil).Times(1)
	err := runner.KubectlApply(ctx, mockConn, opts1)
	assert.NoError(t, err)

	// Test Case 2: Apply with stdin
	optsStdin := KubectlApplyOptions{Filenames: []string{"-"}, FileContent: content, Namespace: namespace}
	expectedCmdStdin := fmt.Sprintf("kubectl apply -f - --namespace %s", shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmdStdin, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, execOpts *connector.ExecOptions) ([]byte, []byte, error) {
			assert.Equal(t, []byte(content), execOpts.Stdin)
			return []byte("deployment.apps/nginx-deployment created from stdin"), []byte{}, nil
		}).Times(1)
	err = runner.KubectlApply(ctx, mockConn, optsStdin)
	assert.NoError(t, err)

	// Test Case 3: Apply with force, prune, and selector
	optsForcePrune := KubectlApplyOptions{
		Filenames: []string{filePath}, Force: true, Prune: true, Selector: "app=nginx", DryRun: "client",
	}
	expectedCmdForcePrune := fmt.Sprintf("kubectl apply -f %s --force --prune -l %s --dry-run=client",
		shellEscape(filePath), shellEscape(optsForcePrune.Selector))
	mockConn.EXPECT().Exec(ctx, expectedCmdForcePrune, gomock.Any()).Return([]byte("deployment.apps/nginx-deployment configured (dry run)"), []byte{}, nil).Times(1)
	err = runner.KubectlApply(ctx, mockConn, optsForcePrune)
	assert.NoError(t, err)

	// Test Case 4: Command execution error
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()). // Re-use cmd1
		Return(nil, []byte("kubectl error"), fmt.Errorf("exec apply error")).Times(1)
	err = runner.KubectlApply(ctx, mockConn, opts1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl apply failed")
	assert.Contains(t, err.Error(), "kubectl error")

	// Test Case 5: No filename or content
	err = runner.KubectlApply(ctx, mockConn, KubectlApplyOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Either Filenames or FileContent must be provided")

	// Test Case 6: Filename is stdin but no content
	err = runner.KubectlApply(ctx, mockConn, KubectlApplyOptions{Filenames: []string{"-"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FileContent must be provided when filename is '-'")
}


func TestDefaultRunner_KubectlGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	resourceType := "pods"
	resourceName := "my-pod"
	namespace := "default"
	expectedOutput := `{"apiVersion":"v1","kind":"Pod",...}` // Simplified JSON

	// Test Case 1: Get specific resource
	opts1 := KubectlGetOptions{Namespace: namespace, OutputFormat: "json"}
	expectedCmd1 := fmt.Sprintf("kubectl get %s %s --namespace %s -o json",
		shellEscape(resourceType), shellEscape(resourceName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	output, err := runner.KubectlGet(ctx, mockConn, resourceType, resourceName, opts1)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, output)

	// Test Case 2: Get all resources of a type in a namespace with selector
	optsAll := KubectlGetOptions{Namespace: namespace, Selector: "app=myApp", OutputFormat: "yaml"}
	expectedCmdAll := fmt.Sprintf("kubectl get %s --namespace %s -l %s -o yaml",
		shellEscape(resourceType), shellEscape(namespace), shellEscape(optsAll.Selector))
	mockConn.EXPECT().Exec(ctx, expectedCmdAll, gomock.Any()).Return([]byte("items:\n- apiVersion: v1..."), []byte{}, nil).Times(1)
	output, err = runner.KubectlGet(ctx, mockConn, resourceType, "", optsAll) // No specific name
	assert.NoError(t, err)
	assert.Contains(t, output, "items:")

	// Test Case 3: Ignore not found
	optsIgnore := KubectlGetOptions{Namespace: namespace, IgnoreNotFound: true}
	expectedCmdIgnore := fmt.Sprintf("kubectl get %s %s --namespace %s --ignore-not-found",
		shellEscape(resourceType), shellEscape("non-existent-pod"), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmdIgnore, gomock.Any()).
		Return(nil, []byte("Error from server (NotFound): pods \"non-existent-pod\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	output, err = runner.KubectlGet(ctx, mockConn, resourceType, "non-existent-pod", optsIgnore)
	assert.NoError(t, err)
	assert.Empty(t, output)

	// Test Case 4: Command execution error (not NotFound)
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()). // Re-use cmd1
		Return(nil, []byte("kubectl get error"), fmt.Errorf("exec get error")).Times(1)
	output, err = runner.KubectlGet(ctx, mockConn, resourceType, resourceName, opts1)
	assert.Error(t, err)
	assert.Empty(t, output) // Stdout is empty because it's part of the error
	assert.Contains(t, err.Error(), "kubectl get pods my-pod failed")
}

func TestDefaultRunner_KubectlDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	resourceType := "deployment"
	resourceName := "nginx-deploy"
	namespace := "kube-system"

	// Test Case 1: Delete by type and name
	opts1 := KubectlDeleteOptions{Namespace: namespace}
	expectedCmd1 := fmt.Sprintf("kubectl delete %s %s --namespace %s",
		shellEscape(resourceType), shellEscape(resourceName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("deployment.apps/nginx-deploy deleted"), []byte{}, nil).Times(1)
	err := runner.KubectlDelete(ctx, mockConn, resourceType, resourceName, opts1)
	assert.NoError(t, err)

	// Test Case 2: Delete with selector and force
	optsSelector := KubectlDeleteOptions{Namespace: namespace, Selector: "app=old-app", Force: true}
	expectedCmdSelector := fmt.Sprintf("kubectl delete %s --namespace %s -l %s --force",
		shellEscape(resourceType), shellEscape(namespace), shellEscape(optsSelector.Selector))
	mockConn.EXPECT().Exec(ctx, expectedCmdSelector, gomock.Any()).Return([]byte("deployment.apps/old-app-1 deleted\ndeployment.apps/old-app-2 deleted"), []byte{}, nil).Times(1)
	err = runner.KubectlDelete(ctx, mockConn, resourceType, "", optsSelector) // No specific name
	assert.NoError(t, err)

	// Test Case 3: Delete with --ignore-not-found
	optsIgnore := KubectlDeleteOptions{Namespace: namespace, IgnoreNotFound: true}
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()). // Re-use cmd1
		Return(nil, []byte("Error from server (NotFound): deployments.apps \"nginx-deploy\" not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.KubectlDelete(ctx, mockConn, resourceType, resourceName, optsIgnore)
	assert.NoError(t, err)

	// Test Case 4: Command execution error (not NotFound)
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).
		Return(nil, []byte("kubectl delete error"), fmt.Errorf("exec delete error")).Times(1)
	err = runner.KubectlDelete(ctx, mockConn, resourceType, resourceName, opts1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl delete failed")

	// Test Case 5: No target specified
	err = runner.KubectlDelete(ctx, mockConn, "", "", KubectlDeleteOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resources to delete must be specified")
}

func TestDefaultRunner_KubectlVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	sampleVersionJSON := `{
    "clientVersion": {
        "major": "1",
        "minor": "25",
        "gitVersion": "v1.25.3",
        "gitCommit": "434bfd82814af038ad94d628e45e622670853940",
        "gitTreeState": "clean",
        "buildDate": "2022-10-12T10:57:26Z",
        "goVersion": "go1.19.2",
        "compiler": "gc",
        "platform": "linux/amd64"
    },
    "serverVersion": {
        "major": "1",
        "minor": "25",
        "gitVersion": "v1.25.4+k3s1",
        "gitCommit": "02f0c52d0f70f892b4itas",
        "gitTreeState": "clean",
        "buildDate": "2022-11-20T02:22:09Z",
        "goVersion": "go1.19.3",
        "compiler": "gc",
        "platform": "linux/amd64"
    }
}`
	var expectedVersionInfo KubectlVersionInfo
	err := json.Unmarshal([]byte(sampleVersionJSON), &expectedVersionInfo)
	assert.NoError(t, err, "Test setup: failed to unmarshal sample version JSON")

	// Test Case 1: Successful version retrieval (client and server)
	mockConn.EXPECT().Exec(ctx, "kubectl version -o json", gomock.Any()).Return([]byte(sampleVersionJSON), []byte{}, nil).Times(1)
	versionInfo, err := runner.KubectlVersion(ctx, mockConn)
	assert.NoError(t, err)
	assert.Equal(t, &expectedVersionInfo, versionInfo)

	// Test Case 2: Server unreachable (kubectl still returns client version and errors)
	sampleClientOnlyJSON := `{
    "clientVersion": { "major": "1", "minor": "25", "gitVersion": "v1.25.3" }
}` // Server part would be missing or null
	serverErrorMsg := "Unable to connect to the server: dial tcp 127.0.0.1:6443: connect: connection refused"
	mockConn.EXPECT().Exec(ctx, "kubectl version -o json", gomock.Any()).
		Return([]byte(sampleClientOnlyJSON), []byte(serverErrorMsg), &connector.CommandError{ExitCode:1}).Times(1)

	versionInfo, err = runner.KubectlVersion(ctx, mockConn)
	assert.Error(t, err) // Error is expected due to server issue
	assert.NotNil(t, versionInfo) // Client version should still be parsed
	assert.Equal(t, "v1.25.3", versionInfo.ClientVersion.GitVersion)
	assert.Nil(t, versionInfo.ServerVersion) // Server version should be nil
	assert.Contains(t, err.Error(), "server might be unreachable")


	// Test Case 3: Invalid JSON output
	mockConn.EXPECT().Exec(ctx, "kubectl version -o json", gomock.Any()).Return([]byte("not json"), []byte{}, nil).Times(1)
	versionInfo, err = runner.KubectlVersion(ctx, mockConn)
	assert.Error(t, err)
	assert.Nil(t, versionInfo)
	assert.Contains(t, err.Error(), "failed to parse kubectl version JSON")

	// Test Case 4: Command fails entirely (no JSON output)
	mockConn.EXPECT().Exec(ctx, "kubectl version -o json", gomock.Any()).
		Return(nil, []byte("total failure"), fmt.Errorf("exec version error")).Times(1)
	versionInfo, err = runner.KubectlVersion(ctx, mockConn)
	assert.Error(t, err)
	assert.Nil(t, versionInfo)
	assert.Contains(t, err.Error(), "kubectl version failed")
}

// TODO: Add tests for KubectlDescribe, KubectlLogs, KubectlExec, etc.
// and specific Getters like KubectlGetNodes, KubectlGetPods once implemented.
// The pattern will be similar: mock kubectl command, check args, mock output, assert results/errors.
// For Getters that parse into specific structs (e.g. KubectlNodeInfo), tests will need sample JSON
// for those structs and assert fields.
