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

func TestDefaultRunner_KubectlDescribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	resourceType := "pod"
	resourceName := "my-test-pod"
	namespace := "testing"
	expectedOutput := "Name: my-test-pod\nNamespace: testing\nStatus: Running..."

	// Test Case 1: Successful describe
	opts := KubectlDescribeOptions{Namespace: namespace}
	expectedCmd := fmt.Sprintf("kubectl describe %s %s --namespace %s",
		shellEscape(resourceType), shellEscape(resourceName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	output, err := runner.KubectlDescribe(ctx, mockConn, resourceType, resourceName, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, output)

	// Test Case 2: Describe with selector
	optsSelector := KubectlDescribeOptions{Namespace: namespace, Selector: "app=test"}
	expectedCmdSelector := fmt.Sprintf("kubectl describe %s --namespace %s -l %s",
		shellEscape(resourceType), shellEscape(namespace), shellEscape(optsSelector.Selector))
	mockConn.EXPECT().Exec(ctx, expectedCmdSelector, gomock.Any()).Return([]byte("Describing matching pods..."), []byte{}, nil).Times(1)
	output, err = runner.KubectlDescribe(ctx, mockConn, resourceType, "", optsSelector) // No specific name
	assert.NoError(t, err)
	assert.Equal(t, "Describing matching pods...", output)

	// Test Case 3: Command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return([]byte("Error: pod not found"), []byte("Error from server (NotFound)"), fmt.Errorf("exec error")).Times(1)
	output, err = runner.KubectlDescribe(ctx, mockConn, resourceType, resourceName, opts)
	assert.Error(t, err)
	assert.Contains(t, output, "Error: pod not foundError from server (NotFound)") // Combined output
	assert.Contains(t, err.Error(), "kubectl describe pod my-test-pod failed")
}

func TestDefaultRunner_KubectlLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	podName := "logger-pod"
	namespace := "logging"

	// Test Case 1: Basic logs
	opts := KubectlLogOptions{Namespace: namespace, Container: "app-container"}
	expectedCmd := fmt.Sprintf("kubectl logs %s --namespace %s -c %s",
		shellEscape(podName), shellEscape(namespace), shellEscape(opts.Container))
	logContent := "Log line 1\nLog line 2"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(logContent), []byte{}, nil).Times(1)
	output, err := runner.KubectlLogs(ctx, mockConn, podName, opts)
	assert.NoError(t, err)
	assert.Equal(t, logContent, output)

	// Test Case 2: Logs with --previous and --tail
	tailLines := int64(50)
	optsPrev := KubectlLogOptions{Namespace: namespace, Previous: true, TailLines: &tailLines}
	expectedCmdPrev := fmt.Sprintf("kubectl logs %s --namespace %s -p --tail=50", shellEscape(podName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmdPrev, gomock.Any()).Return([]byte("Previous logs..."), []byte{}, nil).Times(1)
	output, err = runner.KubectlLogs(ctx, mockConn, podName, optsPrev)
	assert.NoError(t, err)
	assert.Equal(t, "Previous logs...", output)
}

func TestDefaultRunner_KubectlExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	podName := "exec-target-pod"
	namespace := "exec-ns"
	containerName := "target-container"
	command := []string{"/bin/sh", "-c", "echo hello world"}

	// Test Case 1: Successful exec
	opts := KubectlExecOptions{Namespace: namespace, Container: containerName, TTY: true, Stdin: true}
	expectedCmd := fmt.Sprintf("kubectl exec --namespace %s -c %s -i -t %s -- %s %s %s",
		shellEscape(namespace), shellEscape(containerName), shellEscape(podName),
		shellEscape(command[0]), shellEscape(command[1]), shellEscape(command[2]))

	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("hello world\n"), []byte{}, nil).Times(1)
	output, err := runner.KubectlExec(ctx, mockConn, podName, containerName, command, opts)
	assert.NoError(t, err)
	assert.Equal(t, "hello world\n", output)

	// Test Case 2: Exec fails (command in container returns error)
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()). // Use Any cmd due to complexity for this case
		Return([]byte(""), []byte("OCI runtime exec failed: exec failed: ...: exit status 1"), &connector.CommandError{ExitCode: 1}).Times(1)
	output, err = runner.KubectlExec(ctx, mockConn, podName, containerName, command, opts)
	assert.Error(t, err)
	assert.Contains(t, output, "OCI runtime exec failed")
	assert.Contains(t, err.Error(), "kubectl exec in pod exec-target-pod failed")
}

func TestDefaultRunner_KubectlGetNodes(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    sampleNodesJSON := `{
        "apiVersion": "v1",
        "kind": "List",
        "items": [
            {
                "metadata": {"name": "node1", "uid": "uid1"},
                "spec": {"podCIDR": "10.244.0.0/24"},
                "status": {"nodeInfo": {"kubeletVersion": "v1.25.0"}}
            },
            {
                "metadata": {"name": "node2", "uid": "uid2"},
                "spec": {"podCIDR": "10.244.1.0/24"},
                "status": {"nodeInfo": {"kubeletVersion": "v1.25.1"}}
            }
        ]
    }`
    expectedCmd := "kubectl get nodes -o json" // Base command for KubectlGetNodes with default opts
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleNodesJSON), []byte{}, nil).Times(1)

    nodes, err := runner.KubectlGetNodes(ctx, mockConn, KubectlGetOptions{})
    assert.NoError(t, err)
    assert.Len(t, nodes, 2)
    assert.Equal(t, "node1", nodes[0].Metadata.Name)
    assert.Equal(t, "v1.25.1", nodes[1].Status.NodeInfo.KubeletVersion)
}

func TestDefaultRunner_KubectlGetPods(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    namespace := "app-ns"

    samplePodsJSON := `{
        "apiVersion": "v1",
        "kind": "List",
        "items": [
            {
                "metadata": {"name": "pod-a", "namespace": "app-ns"},
                "spec": {"nodeName": "node1"},
                "status": {"phase": "Running", "podIP": "10.1.1.2"}
            }
        ]
    }`
    expectedCmd := fmt.Sprintf("kubectl get pods --namespace %s -o json", shellEscape(namespace))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(samplePodsJSON), []byte{}, nil).Times(1)

    pods, err := runner.KubectlGetPods(ctx, mockConn, namespace, KubectlGetOptions{})
    assert.NoError(t, err)
    assert.Len(t, pods, 1)
    assert.Equal(t, "pod-a", pods[0].Metadata.Name)
    assert.Equal(t, "Running", pods[0].Status.Phase)
}


func TestDefaultRunner_KubectlGetServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "production"

	sampleServicesJSON := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"metadata": {"name": "service-a", "namespace": "production"},
				"spec": {"type": "ClusterIP", "clusterIP": "10.0.0.100", "ports": [{"port": 80, "targetPort": 8080}]},
				"status": {}
			}
		]
	}`
	opts := KubectlGetOptions{} // Using default get options, namespace will be set by func
	expectedCmd := fmt.Sprintf("kubectl get services --namespace %s -o json", shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleServicesJSON), []byte{}, nil).Times(1)

	services, err := runner.KubectlGetServices(ctx, mockConn, namespace, opts)
	assert.NoError(t, err)
	assert.Len(t, services, 1)
	assert.Equal(t, "service-a", services[0].Metadata.Name)
	assert.Equal(t, "ClusterIP", services[0].Spec.Type)
}

func TestDefaultRunner_KubectlGetDeployments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	namespace := "dev"

	sampleDeploymentsJSON := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"metadata": {"name": "deploy-x", "namespace": "dev", "generation": 1},
				"spec": {"replicas": 3},
				"status": {"readyReplicas": 3, "availableReplicas": 3}
			}
		]
	}`
	opts := KubectlGetOptions{}
	expectedCmd := fmt.Sprintf("kubectl get deployments --namespace %s -o json", shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleDeploymentsJSON), []byte{}, nil).Times(1)

	deployments, err := runner.KubectlGetDeployments(ctx, mockConn, namespace, opts)
	assert.NoError(t, err)
	assert.Len(t, deployments, 1)
	assert.Equal(t, "deploy-x", deployments[0].Metadata.Name)
	assert.Equal(t, int32(3), *deployments[0].Spec.Replicas)
}

func TestDefaultRunner_KubectlRolloutStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	resourceType := "deployment"
	resourceName := "my-app"
	namespace := "apps"
	successOutput := "deployment \"my-app\" successfully rolled out"

	// Test Case 1: Successful rollout status
	opts := KubectlRolloutOptions{Namespace: namespace, Watch: true, Timeout: 60 * time.Second}
	expectedCmd := fmt.Sprintf("kubectl rollout status %s/%s --namespace %s --watch --timeout %s",
		shellEscape(resourceType), shellEscape(resourceName), shellEscape(namespace), opts.Timeout.String())
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(successOutput), []byte{}, nil).Times(1)

	output, err := runner.KubectlRolloutStatus(ctx, mockConn, resourceType, resourceName, opts)
	assert.NoError(t, err)
	assert.Equal(t, successOutput, output)

	// Test Case 2: Rollout status command fails
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		Return(nil, []byte("Error: timed out waiting for condition"), fmt.Errorf("exec error")).Times(1)
	output, err = runner.KubectlRolloutStatus(ctx, mockConn, resourceType, resourceName, opts)
	assert.Error(t, err)
	assert.Contains(t, output, "Error: timed out waiting for condition")
	assert.Contains(t, err.Error(), "kubectl rollout status for deployment/my-app failed")
}

func TestDefaultRunner_KubectlScale(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	resourceType := "replicaset"
	resourceName := "my-rs"
	namespace := "scaling"
	replicas := 5
	successOutput := "replicaset.apps/my-rs scaled"

	// Test Case 1: Successful scale
	opts := KubectlScaleOptions{Namespace: namespace}
	expectedCmd := fmt.Sprintf("kubectl scale %s %s --replicas=%d --namespace %s",
		shellEscape(resourceType), shellEscape(resourceName), replicas, shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(successOutput), []byte{}, nil).Times(1)

	output, err := runner.KubectlScale(ctx, mockConn, resourceType, resourceName, replicas, opts)
	assert.NoError(t, err)
	assert.Equal(t, successOutput, output)

	// Test Case 2: Scale command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("Error: scaling failed"), fmt.Errorf("exec error")).Times(1)
	output, err = runner.KubectlScale(ctx, mockConn, resourceType, resourceName, replicas, opts)
	assert.Error(t, err)
	assert.Contains(t, output, "Error: scaling failed")
	assert.Contains(t, err.Error(), "kubectl scale for replicaset/my-rs failed")
}

func TestDefaultRunner_KubectlConfigView(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    sampleConfigJSON := `{
        "apiVersion": "v1",
        "clusters": [{"name": "cluster1", "cluster": {"server": "https://localhost:6443"}}],
        "contexts": [{"name": "context1", "context": {"cluster": "cluster1", "user": "user1"}}],
        "current-context": "context1",
        "kind": "Config",
        "users": [{"name": "user1", "user": {}}]
    }`
    var expectedConfig KubectlConfigInfo
    json.Unmarshal([]byte(sampleConfigJSON), &expectedConfig)

    cmd := "kubectl config view -o json"
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(sampleConfigJSON), []byte{}, nil).Times(1)

    config, err := runner.KubectlConfigView(ctx, mockConn, KubectlConfigViewOptions{})
    assert.NoError(t, err)
    assert.Equal(t, &expectedConfig, config)
}

func TestDefaultRunner_KubectlConfigGetContexts(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    sampleConfigJSON := `{
        "apiVersion": "v1",
        "clusters": [{"name": "c1", "cluster": {"server":"s1"}}, {"name": "c2", "cluster": {"server":"s2"}}],
        "contexts": [
            {"name": "ctx1", "context": {"cluster": "c1", "user": "u1", "namespace": "ns1"}},
            {"name": "ctx2", "context": {"cluster": "c2", "user": "u2"}}
        ],
        "current-context": "ctx1",
        "kind": "Config",
        "users": [{"name": "u1", "user": {}}, {"name": "u2", "user": {}}]
    }`
	expectedContexts := []KubectlContextInfo{
		{Name: "ctx1", Cluster: "c1", AuthInfo: "u1", Namespace: "ns1", Current: true},
		{Name: "ctx2", Cluster: "c2", AuthInfo: "u2", Namespace: "", Current: false},
	}

    // KubectlConfigGetContexts internally calls KubectlConfigView
    mockConn.EXPECT().Exec(ctx, "kubectl config view -o json", gomock.Any()).Return([]byte(sampleConfigJSON), []byte{}, nil).Times(1)

    contexts, err := runner.KubectlConfigGetContexts(ctx, mockConn)
    assert.NoError(t, err)
    assert.Equal(t, expectedContexts, contexts)
}

func TestDefaultRunner_KubectlConfigUseContext(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    contextName := "new-context"

    cmd := fmt.Sprintf("kubectl config use-context %s", shellEscape(contextName))
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return([]byte(fmt.Sprintf("Switched to context \"%s\".", contextName)), []byte{}, nil).Times(1)
    err := runner.KubectlConfigUseContext(ctx, mockConn, contextName)
    assert.NoError(t, err)

    // Test failure
    mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("error: no context exists with the name"), fmt.Errorf("exec error")).Times(1)
    err = runner.KubectlConfigUseContext(ctx, mockConn, contextName)
    assert.Error(t, err)
}

// TODO: Add tests for KubectlPortForward, KubectlTopNode, KubectlTopPod once implemented.
