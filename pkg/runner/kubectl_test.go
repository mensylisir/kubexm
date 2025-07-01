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
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	filePath, namespace, content := "/t/d.yaml", "ns", "apiVersion: v1\nkind: Pod"

	opts1 := KubectlApplyOptions{Filenames: []string{filePath}, Namespace: namespace}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl apply -f %s --namespace %s", shellEscape(filePath), shellEscape(namespace)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlApply(ctx, mockConn, opts1))

	optsStdin := KubectlApplyOptions{Filenames: []string{"-"}, FileContent: content, Namespace: namespace}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl apply -f - --namespace %s", shellEscape(namespace)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlApply(ctx, mockConn, optsStdin))
}

func TestDefaultRunner_KubectlGet(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, ns, expectedOut := "pods", "mypod", "def", `{"kind":"Pod"}`
	opts1 := KubectlGetOptions{Namespace: ns, OutputFormat: "json"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl get %s %s --namespace %s -o json", shellEscape(resType), shellEscape(resName), shellEscape(ns)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlGet(ctx, mockConn, resType, resName, opts1); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlDelete(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, ns := "deploy", "nginx", "kube-sys"
	opts1 := KubectlDeleteOptions{Namespace: ns}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl delete %s %s --namespace %s", shellEscape(resType), shellEscape(resName), shellEscape(ns)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlDelete(ctx, mockConn, resType, resName, opts1))
}

func TestDefaultRunner_KubectlVersion(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	sampleJSON := `{"clientVersion": {"gitVersion":"v1.20"}, "serverVersion": {"gitVersion":"v1.21"}}`
	var expected KubectlVersionInfo; json.Unmarshal([]byte(sampleJSON), &expected)
	mockConn.EXPECT().Exec(ctx, "kubectl version -o json", gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
	ver, err := runner.KubectlVersion(ctx, mockConn); assert.NoError(t, err); assert.Equal(t, &expected, ver)
}

func TestDefaultRunner_KubectlDescribe(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, ns, expectedOut := "pod", "testpod", "testns", "Name: testpod..."
	opts := KubectlDescribeOptions{Namespace: ns}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl describe %s %s --namespace %s", shellEscape(resType), shellEscape(resName), shellEscape(ns)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlDescribe(ctx, mockConn, resType, resName, opts); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlLogs(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	podName, ns, expectedLogs := "logpod", "logns", "log line"
	opts := KubectlLogOptions{Namespace: ns, Container: "app"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl logs %s --namespace %s -c app", shellEscape(podName), shellEscape(ns)), gomock.Any()).Return([]byte(expectedLogs), nil, nil).Times(1)
	logs, err := runner.KubectlLogs(ctx, mockConn, podName, opts); assert.NoError(t, err); assert.Equal(t, expectedLogs, logs)
}

func TestDefaultRunner_KubectlExec(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	podName, container, cmdToRun, expectedOut := "execpod", "c1", []string{"ls", "-l"}, "total 0"
	opts := KubectlExecOptions{Container: container}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl exec -c %s %s -- %s %s", shellEscape(container), shellEscape(podName), shellEscape(cmdToRun[0]), shellEscape(cmdToRun[1])), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlExec(ctx, mockConn, podName, container, cmdToRun, opts); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlGetNodes(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    sampleJSON := `{"items": [{"metadata": {"name": "node1"}}]}`, var expected []KubectlNodeInfo; json.Unmarshal([]byte(sampleJSON), &struct{Items []KubectlNodeInfo `json:"items"`}{Items: &expected})
    mockConn.EXPECT().Exec(ctx, "kubectl get nodes -o json", gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    nodes, err := runner.KubectlGetNodes(ctx, mockConn, KubectlGetOptions{}); assert.NoError(t, err); assert.Equal(t, expected, nodes)
}

func TestDefaultRunner_KubectlGetPods(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns := "appns"
    sampleJSON := `{"items": [{"metadata": {"name": "poda"}}]}`, var expected []KubectlPodInfo; json.Unmarshal([]byte(sampleJSON), &struct{Items []KubectlPodInfo `json:"items"`}{Items: &expected})
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl get pods --namespace %s -o json", shellEscape(ns)), gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    pods, err := runner.KubectlGetPods(ctx, mockConn, ns, KubectlGetOptions{}); assert.NoError(t, err); assert.Equal(t, expected, pods)
}

func TestDefaultRunner_KubectlGetServices(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns := "prod"
	sampleJSON := `{"items": [{"metadata": {"name": "svca"}}]}`, var expected []KubectlServiceInfo; json.Unmarshal([]byte(sampleJSON), &struct{Items []KubectlServiceInfo `json:"items"`}{Items: &expected})
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl get services --namespace %s -o json", shellEscape(ns)), gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
	svcs, err := runner.KubectlGetServices(ctx, mockConn, ns, KubectlGetOptions{}); assert.NoError(t, err); assert.Equal(t, expected, svcs)
}

func TestDefaultRunner_KubectlGetDeployments(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns := "dev"
	sampleJSON := `{"items": [{"metadata": {"name": "depx"}}]}`, var expected []KubectlDeploymentInfo; json.Unmarshal([]byte(sampleJSON), &struct{Items []KubectlDeploymentInfo `json:"items"`}{Items: &expected})
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl get deployments --namespace %s -o json", shellEscape(ns)), gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
	deps, err := runner.KubectlGetDeployments(ctx, mockConn, ns, KubectlGetOptions{}); assert.NoError(t, err); assert.Equal(t, expected, deps)
}

func TestDefaultRunner_KubectlRolloutStatus(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, ns, expectedOut := "deploy", "myapp", "apps", "rolled out"
	opts := KubectlRolloutOptions{Namespace: ns}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl rollout status %s/%s --namespace %s", shellEscape(resType), shellEscape(resName), shellEscape(ns)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlRolloutStatus(ctx, mockConn, resType, resName, opts); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlScale(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, ns, replicas, expectedOut := "rs", "myrs", "scl", 5, "scaled"
	opts := KubectlScaleOptions{Namespace: ns}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl scale %s %s --replicas=%d --namespace %s", shellEscape(resType), shellEscape(resName), replicas, shellEscape(ns)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlScale(ctx, mockConn, resType, resName, replicas, opts); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlConfigView(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    sampleJSON := `{"kind":"Config","current-context":"ctx1"}`; var expected KubectlConfigInfo; json.Unmarshal([]byte(sampleJSON), &expected)
    mockConn.EXPECT().Exec(ctx, "kubectl config view -o json", gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    cfg, err := runner.KubectlConfigView(ctx, mockConn, KubectlConfigViewOptions{}); assert.NoError(t, err); assert.Equal(t, &expected, cfg)
}

func TestDefaultRunner_KubectlConfigGetContexts(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    sampleJSON := `{"contexts":[{"name":"ctx1"}],"current-context":"ctx1"}` // Simplified
    mockConn.EXPECT().Exec(ctx, "kubectl config view -o json", gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    ctxs, err := runner.KubectlConfigGetContexts(ctx, mockConn); assert.NoError(t, err); assert.Len(t, ctxs, 1)
}

func TestDefaultRunner_KubectlConfigUseContext(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ctxName := "newctx"
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl config use-context %s", shellEscape(ctxName)), gomock.Any()).Return(nil,nil,nil).Times(1)
    assert.NoError(t, runner.KubectlConfigUseContext(ctx, mockConn, ctxName))
}

func TestDefaultRunner_KubectlTopNode(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    sampleJSON := `{"items": [{"metadata":{"name":"node1"},"cpu":{"usageNanoCores":"100m"},"memory":{"usageBytes":"200Mi"}}]}`
    mockConn.EXPECT().Exec(ctx, "kubectl top nodes -o json", gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    nodes, err := runner.KubectlTopNodes(ctx, mockConn, KubectlTopOptions{}); assert.NoError(t, err); assert.Len(t, nodes, 1)
	assert.Equal(t, "node1", nodes[0].Metadata.Name)
	// Add more specific checks for parsed CPU/memory if utils.ParseCPU/Memory are robustly tested elsewhere
	// For now, just checking the top-level structure.
}

func TestDefaultRunner_KubectlTopPods(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns := "topns"
    sampleJSON := `{"items": [{"metadata":{"name":"pod1","namespace":"topns"},"containers":[{"name":"c1","cpu":{"usageNanoCores":"50m"},"memory":{"usageBytes":"100Mi"}}]}]}`
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl top pods --namespace %s --containers -o json", shellEscape(ns)), gomock.Any()).Return([]byte(sampleJSON), nil, nil).Times(1)
    pods, err := runner.KubectlTopPods(ctx, mockConn, KubectlTopOptions{Namespace: ns, Containers: true}); assert.NoError(t, err); assert.Len(t, pods, 1)
	assert.Equal(t, "pod1", pods[0].Metadata.Name)
	assert.Len(t, pods[0].Containers, 1)
}

func TestDefaultRunner_KubectlExplain(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); resType := "pods.spec.containers"
    expectedOut := "RESOURCE: containers <Object>\nDESCRIPTION:\n List of containers belonging to the pod."
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl explain %s", shellEscape(resType)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
    out, err := runner.KubectlExplain(ctx, mockConn, resType, KubectlExplainOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlDrainNode(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); nodeName := "node-drain"
	opts := KubectlDrainOptions{IgnoreDaemonSets: true, Force: true}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl drain %s --ignore-daemonsets --force", shellEscape(nodeName)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlDrainNode(ctx, mockConn, nodeName, opts))
}

func TestDefaultRunner_KubectlCordonUncordonNode(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); nodeName := "node-cordon"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl cordon %s", shellEscape(nodeName)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCordonNode(ctx, mockConn, nodeName, KubectlCordonUncordonOptions{}))
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl uncordon %s", shellEscape(nodeName)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlUncordonNode(ctx, mockConn, nodeName, KubectlCordonUncordonOptions{}))
}

func TestDefaultRunner_KubectlTaintNode(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); nodeName := "node-taint"
	taints := []string{"key1=value1:NoSchedule", "key2=:NoExecute"}
	expectedCmd := fmt.Sprintf("kubectl taint nodes %s %s %s", shellEscape(nodeName), shellEscape(taints[0]), shellEscape(taints[1]))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlTaintNode(ctx, mockConn, nodeName, taints, KubectlTaintOptions{}))
}

func TestDefaultRunner_KubectlCreateSecretGeneric(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns, name := "sec-ns", "mysecret"
	literals := map[string]string{"user": "admin"}
	expectedCmd := fmt.Sprintf("kubectl create secret generic %s --namespace %s --from-literal=%s=%s", shellEscape(name), shellEscape(ns), shellEscape("user"), shellEscape("admin"))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateSecretGeneric(ctx, mockConn, ns, name, literals, nil, KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateSecretDockerRegistry(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns, name := "img-sec", "regcred"
	server, user, pass := "docker.io", "u", "p"
	expectedCmd := fmt.Sprintf("kubectl create secret docker-registry %s --docker-server=%s --docker-username=%s --docker-password=%s --namespace %s",
		shellEscape(name), shellEscape(server), shellEscape(user), shellEscape(pass), shellEscape(ns))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateSecretDockerRegistry(ctx, mockConn, ns, name, server, user, pass, "", KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateSecretTLS(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns, name := "tls-sec", "mytls"
	cert, key := "/p/t.crt", "/p/t.key"
	expectedCmd := fmt.Sprintf("kubectl create secret tls %s --cert=%s --key=%s --namespace %s", shellEscape(name), shellEscape(cert), shellEscape(key), shellEscape(ns))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateSecretTLS(ctx, mockConn, ns, name, cert, key, KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateConfigMap(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns, name := "cm-ns", "mycm"
	literals := map[string]string{"k": "v"}
	expectedCmd := fmt.Sprintf("kubectl create configmap %s --namespace %s --from-literal=%s=%s", shellEscape(name), shellEscape(ns), shellEscape("k"), shellEscape("v"))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateConfigMap(ctx, mockConn, ns, name, literals, nil, "", KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateServiceAccount(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); ns, name := "sa-ns", "mysa"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl create serviceaccount %s --namespace %s", shellEscape(name), shellEscape(ns)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateServiceAccount(ctx, mockConn, ns, name, KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateRoleClusterRole(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	ns, name, verbs, resources := "role-ns", "myrole", []string{"get","list"}, []string{"pods"}
	// Role
	expCmdRole := fmt.Sprintf("kubectl create role %s --verb=%s --resource=%s --namespace %s", shellEscape(name), shellEscape("get,list"), shellEscape("pods"), shellEscape(ns))
	mockConn.EXPECT().Exec(ctx, expCmdRole, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateRole(ctx, mockConn, ns, name, verbs, resources, nil, KubectlCreateOptions{}))
	// ClusterRole
	expCmdCRole := fmt.Sprintf("kubectl create clusterrole %s --verb=%s --resource=%s", shellEscape(name), shellEscape("get,list"), shellEscape("pods"))
	mockConn.EXPECT().Exec(ctx, expCmdCRole, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateClusterRole(ctx, mockConn, name, verbs, resources, nil, "", KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlCreateRoleClusterRoleBinding(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	ns, name, role, sa := "rb-ns", "myrb", "edit", "mysa"
	// RoleBinding
	expCmdRB := fmt.Sprintf("kubectl create rolebinding %s --role=%s --serviceaccount=%s:%s --namespace %s", shellEscape(name), shellEscape(role), shellEscape(ns), shellEscape(sa), shellEscape(ns))
	mockConn.EXPECT().Exec(ctx, expCmdRB, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateRoleBinding(ctx, mockConn, ns, name, role, sa, nil, nil, KubectlCreateOptions{}))
	// ClusterRoleBinding
	fullSA := fmt.Sprintf("%s:%s", ns, sa) // For CRB, SA needs namespace
	expCmdCRB := fmt.Sprintf("kubectl create clusterrolebinding %s --clusterrole=%s --serviceaccount=%s:%s", shellEscape(name), shellEscape(role), shellEscape(ns), shellEscape(sa))
	mockConn.EXPECT().Exec(ctx, expCmdCRB, gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlCreateClusterRoleBinding(ctx, mockConn, name, role, fullSA, nil, nil, KubectlCreateOptions{}))
}

func TestDefaultRunner_KubectlSetImage(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, container, newImg := "deploy", "myapp", "c1", "img:v2"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl set image %s %s %s=%s", shellEscape(resType), shellEscape(resName), shellEscape(container), shellEscape(newImg)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlSetImage(ctx, mockConn, resType, resName, container, newImg, KubectlSetOptions{}))
}

func TestDefaultRunner_KubectlSetEnv(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, container := "deploy", "myapp", "c1"
	envVars := map[string]string{"K":"V"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl set env %s %s --containers=%s K=V", shellEscape(resType), shellEscape(resName), shellEscape(container)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlSetEnv(ctx, mockConn, resType, resName, container, envVars, nil, "", "", KubectlSetOptions{}))
}

func TestDefaultRunner_KubectlSetResources(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, container := "deploy", "myapp", "c1"
	limits := map[string]string{"cpu":"100m"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl set resources %s %s --containers=%s --limits=%s", shellEscape(resType), shellEscape(resName), shellEscape(container), shellEscape("cpu=100m")), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlSetResources(ctx, mockConn, resType, resName, container, limits, nil, KubectlSetOptions{}))
}

func TestDefaultRunner_KubectlAutoscale(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, min, max, cpu := "deploy", "myapp", int32(1), int32(5), int32(80)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl autoscale %s %s --min=%d --max=%d --cpu-percent=%d", shellEscape(resType), shellEscape(resName), min, max, cpu), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlAutoscale(ctx, mockConn, resType, resName, min, max, cpu, KubectlAutoscaleOptions{}))
}

func TestDefaultRunner_KubectlCompletion(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); shell, expectedOut := "bash", "bash completion script"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl completion %s", shellEscape(shell)), gomock.Any()).Return([]byte(expectedOut), nil, nil).Times(1)
	out, err := runner.KubectlCompletion(ctx, mockConn, shell); assert.NoError(t, err); assert.Equal(t, expectedOut, out)
}

func TestDefaultRunner_KubectlWait(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, cond := "pod", "mywaitpod", "condition=Ready"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl wait %s/%s --for=%s", shellEscape(resType), shellEscape(resName), shellEscape(cond)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlWait(ctx, mockConn, resType, resName, cond, KubectlWaitOptions{}))
}

func TestDefaultRunner_KubectlLabel(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, labels := "node", "mynode", map[string]string{"k":"v"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl label %s %s k=v", shellEscape(resType), shellEscape(resName)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlLabel(ctx, mockConn, resType, resName, labels, false, KubectlLabelOptions{}))
}

func TestDefaultRunner_KubectlAnnotate(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, annos := "pod", "mypod", map[string]string{"desc":"test"}
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl annotate %s %s desc=test", shellEscape(resType), shellEscape(resName)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlAnnotate(ctx, mockConn, resType, resName, annos, false, KubectlAnnotateOptions{}))
}

func TestDefaultRunner_KubectlPatch(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	resType, resName, patchType, patchContent := "svc", "mysvc", "merge", `{"spec":{"ports":[{"port":8080}]}}`
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("kubectl patch %s %s --type %s -p %s", shellEscape(resType), shellEscape(resName), shellEscape(patchType), shellEscape(patchContent)), gomock.Any()).Return(nil,nil,nil).Times(1)
	assert.NoError(t, runner.KubectlPatch(ctx, mockConn, resType, resName, patchType, patchContent, KubectlPatchOptions{}))
}
