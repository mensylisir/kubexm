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

func TestDefaultRunner_HelmInstall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	releaseName := "my-nginx"
	chartPath := "stable/nginx-ingress"
	namespace := "test-ns"

	// Test Case 1: Basic successful install
	opts1 := HelmInstallOptions{Namespace: namespace, CreateNamespace: true}
	expectedCmd1 := fmt.Sprintf("helm install %s %s --namespace %s --create-namespace",
		shellEscape(releaseName), shellEscape(chartPath), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("NOTES: ..."), []byte{}, nil).Times(1)
	err := runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts1)
	assert.NoError(t, err)

	// Test Case 2: Install with values, version, and wait
	opts2 := HelmInstallOptions{
		Namespace: namespace,
		Version:   "1.2.3",
		ValuesFiles: []string{"/tmp/values.yaml"},
		SetValues:   []string{"controller.replicaCount=2", "image.tag=latest"},
		Wait:      true,
		Timeout:   300 * time.Second,
	}
	expectedCmd2Parts := []string{
		"helm", "install", shellEscape(releaseName), shellEscape(chartPath),
		"--namespace", shellEscape(namespace),
		"--version", shellEscape(opts2.Version),
		"--values", shellEscape(opts2.ValuesFiles[0]),
		"--set", shellEscape(opts2.SetValues[0]),
		"--set", shellEscape(opts2.SetValues[1]),
		"--wait",
		"--timeout", opts2.Timeout.String(),
	}
	// Using DoAndReturn to check if all parts are in the command, as order of --set might vary.
	mockConn.EXPECT().Exec(ctx, gomock.AssignableToTypeOf("string"), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, execOpts *connector.ExecOptions) ([]byte, []byte, error) {
			for _, part := range expectedCmd2Parts {
				assert.Contains(t, cmd, part)
			}
			assert.Equal(t, opts2.Timeout+(1*time.Minute), execOpts.Timeout) // Check adjusted exec timeout
			return []byte("NOTES: ..."), []byte{}, nil
		}).Times(1)
	err = runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts2)
	assert.NoError(t, err)

	// Test Case 3: Helm command execution fails
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()). // Re-use opts1 for simplicity
		Return([]byte("Error output"), []byte("helm generic error"), fmt.Errorf("exec error")).Times(1)
	err = runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "helm install for release")
	assert.Contains(t, err.Error(), "helm generic error") // Stderr should be in the error
	assert.Contains(t, err.Error(), "Error output")     // Stdout also
}

func TestDefaultRunner_HelmUninstall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	releaseName := "my-nginx-uninstall"
	namespace := "test-ns-uninstall"

	// Test Case 1: Basic successful uninstall
	opts1 := HelmUninstallOptions{Namespace: namespace}
	expectedCmd1 := fmt.Sprintf("helm uninstall %s --namespace %s", shellEscape(releaseName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("release \"my-nginx-uninstall\" uninstalled"), []byte{}, nil).Times(1)
	err := runner.HelmUninstall(ctx, mockConn, releaseName, opts1)
	assert.NoError(t, err)

	// Test Case 2: Uninstall with keep history and timeout
	opts2 := HelmUninstallOptions{
		Namespace:   namespace,
		KeepHistory: true,
		Timeout:     120 * time.Second,
	}
	expectedCmd2 := fmt.Sprintf("helm uninstall %s --namespace %s --keep-history --timeout %s",
		shellEscape(releaseName), shellEscape(namespace), opts2.Timeout.String())
	mockConn.EXPECT().Exec(ctx, expectedCmd2, gomock.Any()).Return([]byte("release uninstalled"), []byte{}, nil).Times(1)
	err = runner.HelmUninstall(ctx, mockConn, releaseName, opts2)
	assert.NoError(t, err)

	// Test Case 3: Release not found (idempotency)
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()). // Re-use opts1
		Return(nil, []byte("Error: uninstall: Release not loaded: my-nginx-uninstall: release: not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.HelmUninstall(ctx, mockConn, releaseName, opts1)
	assert.NoError(t, err) // Should be idempotent

	// Test Case 4: Helm command execution fails (other error)
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).
		Return(nil, []byte("helm generic uninstall error"), fmt.Errorf("exec uninstall error")).Times(1)
	err = runner.HelmUninstall(ctx, mockConn, releaseName, opts1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "helm uninstall for release")
	assert.Contains(t, err.Error(), "helm generic uninstall error")
}

func TestDefaultRunner_HelmList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	sampleListOutput := `[
    {"name":"release1","namespace":"ns1","revision":"1","updated":"2023-01-01 10:00:00.000 +0000 UTC","status":"deployed","chart":"chart1-0.1.0","app_version":"1.0.0"},
    {"name":"release2","namespace":"ns2","revision":"3","updated":"2023-01-02 12:00:00.000 +0000 UTC","status":"failed","chart":"chart2-1.2.0","app_version":"2.1.0"}
]`
	expectedReleases := []HelmReleaseInfo{
		{Name: "release1", Namespace: "ns1", Revision: "1", Updated: "2023-01-01 10:00:00.000 +0000 UTC", Status: "deployed", Chart: "chart1-0.1.0", AppVersion: "1.0.0"},
		{Name: "release2", Namespace: "ns2", Revision: "3", Updated: "2023-01-02 12:00:00.000 +0000 UTC", Status: "failed", Chart: "chart2-1.2.0", AppVersion: "2.1.0"},
	}

	// Test Case 1: Successful list with default options
	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte(sampleListOutput), []byte{}, nil).Times(1)
	releases, err := runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedReleases, releases)

	// Test Case 2: List with all-namespaces and filter
	opts := HelmListOptions{AllNamespaces: true, Filter: "release"}
	expectedCmd := "helm list --all-namespaces --filter 'release' -o json"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleListOutput), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedReleases, releases)

	// Test Case 3: Empty list result (valid JSON "[]" or "" or "null")
	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte("[]"), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, releases)

	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte(""), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, releases)

	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte("null"), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, releases)


	// Test Case 4: Invalid JSON output
	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte("not json"), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.Error(t, err)
	assert.Nil(t, releases)
	assert.Contains(t, err.Error(), "failed to parse helm list JSON output")

	// Test Case 5: Helm command execution error
	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).
		Return(nil, []byte("helm list error"), fmt.Errorf("exec list error")).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{})
	assert.Error(t, err)
	assert.Nil(t, releases)
	assert.Contains(t, err.Error(), "helm list failed")
}

func TestDefaultRunner_HelmStatus(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    releaseName := "my-release"

    sampleStatusOutput := `{
        "name": "my-release",
        "namespace": "default",
        "revision": "2",
        "updated": "2023-10-28 11:00:00.000 +0000 UTC",
        "status": "deployed",
        "chart": "mychart-0.2.0",
        "app_version": "1.1.0",
        "notes": "Some notes here",
        "config": {"key": "value"}
    }`
    var expectedStatus HelmReleaseInfo
    err := json.Unmarshal([]byte(sampleStatusOutput), &expectedStatus)
    assert.NoError(t, err, "Test setup: failed to unmarshal sample status")


    // Test Case 1: Successful status
    opts := HelmStatusOptions{Namespace: "default"}
    expectedCmd := fmt.Sprintf("helm status %s --namespace %s -o json", shellEscape(releaseName), shellEscape(opts.Namespace))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleStatusOutput), []byte{}, nil).Times(1)

    status, err := runner.HelmStatus(ctx, mockConn, releaseName, opts)
    assert.NoError(t, err)
    assert.Equal(t, &expectedStatus, status)

    // Test Case 2: Release not found
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
        Return(nil, []byte("Error: release: not found"), &connector.CommandError{ExitCode: 1}).Times(1)
    status, err = runner.HelmStatus(ctx, mockConn, releaseName, opts)
    assert.NoError(t, err) // Expects nil, nil for not found
    assert.Nil(t, status)

    // Test Case 3: Helm command execution error
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
        Return(nil, []byte("helm status error"), fmt.Errorf("exec status error")).Times(1)
    status, err = runner.HelmStatus(ctx, mockConn, releaseName, opts)
    assert.Error(t, err)
    assert.Nil(t, status)
    assert.Contains(t, err.Error(), "helm status for release")
}

func TestDefaultRunner_HelmRepoAdd(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    repoName := "bitnami"
    repoURL := "https://charts.bitnami.com/bitnami"

    // Test Case 1: Successful repo add
    opts := HelmRepoAddOptions{}
    expectedCmd := fmt.Sprintf("helm repo add %s %s", shellEscape(repoName), shellEscape(repoURL))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(fmt.Sprintf("\"%s\" has been added to your repositories", repoName)), []byte{}, nil).Times(1)
    err := runner.HelmRepoAdd(ctx, mockConn, repoName, repoURL, opts)
    assert.NoError(t, err)

    // Test Case 2: Repo already exists, no force update (should error by default helm behavior)
    optsNoForce := HelmRepoAddOptions{ForceUpdate: false}
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
        Return(nil, []byte(fmt.Sprintf("Error: repository name (%s) already exists, use --force-update to overwrite", repoName)), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.HelmRepoAdd(ctx, mockConn, repoName, repoURL, optsNoForce)
    assert.Error(t, err) // Helm errors if repo exists and no --force-update
    assert.Contains(t, err.Error(), "already exists and --force-update not used")

    // Test Case 3: Repo already exists, with force update
    optsForce := HelmRepoAddOptions{ForceUpdate: true}
    expectedCmdForce := fmt.Sprintf("helm repo add %s %s --force-update", shellEscape(repoName), shellEscape(repoURL))
    mockConn.EXPECT().Exec(ctx, expectedCmdForce, gomock.Any()).Return([]byte(fmt.Sprintf("\"%s\" has been updated in your repositories", repoName)), []byte{}, nil).Times(1)
    err = runner.HelmRepoAdd(ctx, mockConn, repoName, repoURL, optsForce)
    assert.NoError(t, err)
}

func TestDefaultRunner_HelmRepoUpdate(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    // Test Case 1: Update all repos
    mockConn.EXPECT().Exec(ctx, "helm repo update", gomock.Any()).Return([]byte("Update Complete."), []byte{}, nil).Times(1)
    err := runner.HelmRepoUpdate(ctx, mockConn, nil) // nil or empty slice for all
    assert.NoError(t, err)

    // Test Case 2: Update specific repos
    repoNames := []string{"stable", "bitnami"}
    expectedCmd := fmt.Sprintf("helm repo update %s %s", shellEscape(repoNames[0]), shellEscape(repoNames[1]))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("Update Complete."), []byte{}, nil).Times(1)
    err = runner.HelmRepoUpdate(ctx, mockConn, repoNames)
    assert.NoError(t, err)

    // Test Case 3: Command fails
    mockConn.EXPECT().Exec(ctx, "helm repo update", gomock.Any()).
        Return(nil, []byte("repo update error"), fmt.Errorf("exec repo update error")).Times(1)
    err = runner.HelmRepoUpdate(ctx, mockConn, nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "helm repo update failed")
}

func TestDefaultRunner_HelmVersion(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    sampleVersionJSON := `{"version":"v3.8.1","gitCommit":"5cb92b3e801bd32cc3319ed842d7a5c6e81e4bab","gitTreeState":"clean","goVersion":"go1.17.5"}`
    expectedVersionInfo := HelmVersionInfo{
        Version:    "v3.8.1",
        GitCommit:  "5cb92b3e801bd32cc3319ed842d7a5c6e81e4bab",
        GitTreeState: "clean",
        GoVersion:  "go1.17.5",
    }

    // Test Case 1: Successful version retrieval
    mockConn.EXPECT().Exec(ctx, "helm version -o json", gomock.Any()).Return([]byte(sampleVersionJSON), []byte{}, nil).Times(1)
    versionInfo, err := runner.HelmVersion(ctx, mockConn)
    assert.NoError(t, err)
    assert.Equal(t, &expectedVersionInfo, versionInfo)

    // Test Case 2: Invalid JSON
    mockConn.EXPECT().Exec(ctx, "helm version -o json", gomock.Any()).Return([]byte("not json"), []byte{}, nil).Times(1)
    versionInfo, err = runner.HelmVersion(ctx, mockConn)
    assert.Error(t, err)
    assert.Nil(t, versionInfo)
    assert.Contains(t, err.Error(), "failed to parse helm version JSON")

    // Test Case 3: Command fails
    mockConn.EXPECT().Exec(ctx, "helm version -o json", gomock.Any()).
        Return(nil, []byte("version error"), fmt.Errorf("exec version error")).Times(1)
    versionInfo, err = runner.HelmVersion(ctx, mockConn)
    assert.Error(t, err)
    assert.Nil(t, versionInfo)
    assert.Contains(t, err.Error(), "helm version failed")
}

func TestDefaultRunner_HelmSearchRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	keyword := "nginx"

	sampleSearchOutput := `[
    {"name":"bitnami/nginx","version":"13.2.22","app_version":"1.23.3","description":"NGINX Open Source is a web server that can be also used as a reverse proxy..."},
    {"name":"ingress-nginx/ingress-nginx","version":"4.7.0","app_version":"1.8.0","description":"Ingress controller for Kubernetes using NGINX as a reverse proxy and load balancer"}
]`
	expectedCharts := []HelmChartInfo{
		{Name: "bitnami/nginx", Version: "13.2.22", AppVersion: "1.23.3", Description: "NGINX Open Source is a web server that can be also used as a reverse proxy..."},
		{Name: "ingress-nginx/ingress-nginx", Version: "4.7.0", AppVersion: "1.8.0", Description: "Ingress controller for Kubernetes using NGINX as a reverse proxy and load balancer"},
	}

	// Test Case 1: Successful search
	opts := HelmSearchOptions{}
	expectedCmd := fmt.Sprintf("helm search repo %s -o json", shellEscape(keyword))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleSearchOutput), []byte{}, nil).Times(1)
	charts, err := runner.HelmSearchRepo(ctx, mockConn, keyword, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedCharts, charts)

	// Test Case 2: Search with --versions and --devel
	optsVersions := HelmSearchOptions{Versions: true, Devel: true}
	expectedCmdVersions := fmt.Sprintf("helm search repo %s --devel --versions -o json", shellEscape(keyword))
	mockConn.EXPECT().Exec(ctx, expectedCmdVersions, gomock.Any()).Return([]byte(sampleSearchOutput), []byte{}, nil).Times(1) // Same output for simplicity
	charts, err = runner.HelmSearchRepo(ctx, mockConn, keyword, optsVersions)
	assert.NoError(t, err)
	assert.Equal(t, expectedCharts, charts)

	// Test Case 3: No results (empty JSON array)
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("[]"), []byte{}, nil).Times(1)
	charts, err = runner.HelmSearchRepo(ctx, mockConn, keyword, opts)
	assert.NoError(t, err)
	assert.Empty(t, charts)

	// Test Case 4: Command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("search error"), fmt.Errorf("exec search error")).Times(1)
	charts, err = runner.HelmSearchRepo(ctx, mockConn, keyword, opts)
	assert.Error(t, err)
	assert.Nil(t, charts)
	assert.Contains(t, err.Error(), "helm search repo for keyword")
}

func TestDefaultRunner_HelmPull(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	chartPath := "stable/mysql"

	// Test Case 1: Successful pull
	opts := HelmPullOptions{Version: "1.6.7", Destination: "/tmp/charts"}
	expectedCmd := fmt.Sprintf("helm pull %s --destination %s --version %s",
		shellEscape(chartPath), shellEscape(opts.Destination), shellEscape(opts.Version))
	expectedOutput := "Successfully downloaded chart to /tmp/charts/mysql-1.6.7.tgz"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(expectedOutput+"\n"), []byte{}, nil).Times(1)

	outputPath, err := runner.HelmPull(ctx, mockConn, chartPath, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, outputPath)

	// Test Case 2: Pull with untar
	optsUntar := HelmPullOptions{Untar: true, UntarDir: "/tmp/untarred_mysql"}
	expectedCmdUntar := fmt.Sprintf("helm pull %s --untar --untardir %s", shellEscape(chartPath), shellEscape(optsUntar.UntarDir))
	expectedOutputUntar := "Successfully downloaded chart to /tmp/untarred_mysql/mysql"
	mockConn.EXPECT().Exec(ctx, expectedCmdUntar, gomock.Any()).Return([]byte(expectedOutputUntar), []byte{}, nil).Times(1)

	outputPath, err = runner.HelmPull(ctx, mockConn, chartPath, optsUntar)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutputUntar, outputPath)

	// Test Case 3: Command fails
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()). // Use Any for cmd as it varies with options
		Return(nil, []byte("pull error"), fmt.Errorf("exec pull error")).Times(1)
	outputPath, err = runner.HelmPull(ctx, mockConn, chartPath, HelmPullOptions{})
	assert.Error(t, err)
	assert.Empty(t, outputPath)
	assert.Contains(t, err.Error(), "helm pull for chart")
}

func TestDefaultRunner_HelmPackage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	chartSourcePath := "./mychartdir" // Path to the chart source directory

	// Test Case 1: Successful package
	opts := HelmPackageOptions{Destination: "/tmp/packages"}
	expectedCmd := fmt.Sprintf("helm package %s --destination %s", shellEscape(chartSourcePath), shellEscape(opts.Destination))
	helmOutput := "Successfully packaged chart and saved it to: /tmp/packages/mychartdir-0.1.0.tgz"
	expectedPackagePath := "/tmp/packages/mychartdir-0.1.0.tgz"
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(helmOutput+"\n"), []byte{}, nil).Times(1)

	packagePath, err := runner.HelmPackage(ctx, mockConn, chartSourcePath, opts)
	assert.NoError(t, err)
	assert.Equal(t, expectedPackagePath, packagePath)

	// Test Case 2: Package with version and appVersion override
	optsVersioned := HelmPackageOptions{Version: "0.2.0", AppVersion: "1.1.0"}
	expectedCmdVersioned := fmt.Sprintf("helm package %s --version %s --app-version %s",
		shellEscape(chartSourcePath), shellEscape(optsVersioned.Version), shellEscape(optsVersioned.AppVersion))
	helmOutputVersioned := "Successfully packaged chart and saved it to: mychartdir-0.2.0.tgz" // Assuming default destination "."
	expectedPackagePathVersioned := "mychartdir-0.2.0.tgz"
	mockConn.EXPECT().Exec(ctx, expectedCmdVersioned, gomock.Any()).Return([]byte(helmOutputVersioned), []byte{}, nil).Times(1)

	packagePath, err = runner.HelmPackage(ctx, mockConn, chartSourcePath, optsVersioned)
	assert.NoError(t, err)
	assert.Equal(t, expectedPackagePathVersioned, packagePath)

	// Test Case 3: Command fails
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()). // Use Any for cmd
		Return(nil, []byte("package error"), fmt.Errorf("exec package error")).Times(1)
	packagePath, err = runner.HelmPackage(ctx, mockConn, chartSourcePath, HelmPackageOptions{})
	assert.Error(t, err)
	assert.Empty(t, packagePath)
	assert.Contains(t, err.Error(), "helm package for chart at")
}
