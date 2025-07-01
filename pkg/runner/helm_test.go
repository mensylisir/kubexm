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
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	releaseName, chartPath, namespace := "my-nginx", "stable/nginx-ingress", "test-ns"

	opts1 := HelmInstallOptions{Namespace: namespace, CreateNamespace: true}
	expectedCmd1 := fmt.Sprintf("helm install %s %s --namespace %s --create-namespace", shellEscape(releaseName), shellEscape(chartPath), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("NOTES: ..."), []byte{}, nil).Times(1)
	err := runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts1); assert.NoError(t, err)

	opts2 := HelmInstallOptions{Namespace: namespace, Version: "1.2.3", ValuesFiles: []string{"/tmp/v.yaml"}, SetValues: []string{"k=v"}, Wait: true, Timeout: 300 * time.Second}
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).Return([]byte("NOTES: ..."), []byte{}, nil).Times(1)
	err = runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts2); assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("Err"), []byte("helm err"), fmt.Errorf("exec err")).Times(1)
	err = runner.HelmInstall(ctx, mockConn, releaseName, chartPath, opts1); assert.Error(t, err)
}

func TestDefaultRunner_HelmUninstall(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	releaseName, namespace := "my-nginx-u", "test-ns-u"

	opts1 := HelmUninstallOptions{Namespace: namespace}
	expectedCmd1 := fmt.Sprintf("helm uninstall %s --namespace %s", shellEscape(releaseName), shellEscape(namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return([]byte("release uninstalled"), []byte{}, nil).Times(1)
	err := runner.HelmUninstall(ctx, mockConn, releaseName, opts1); assert.NoError(t, err)

	mockConn.EXPECT().Exec(ctx, expectedCmd1, gomock.Any()).Return(nil, []byte("Error: release: not found"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.HelmUninstall(ctx, mockConn, releaseName, opts1); assert.NoError(t, err)
}

func TestDefaultRunner_HelmList(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	sampleOutput := `[{"name":"r1","namespace":"ns1","chart":"c1-0.1.0"},{"name":"r2","namespace":"ns2","chart":"c2-1.2.0"}]`
	var expected []HelmReleaseInfo; json.Unmarshal([]byte(sampleOutput), &expected)

	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte(sampleOutput), []byte{}, nil).Times(1)
	releases, err := runner.HelmList(ctx, mockConn, HelmListOptions{}); assert.NoError(t, err); assert.Equal(t, expected, releases)

	mockConn.EXPECT().Exec(ctx, "helm list -o json", gomock.Any()).Return([]byte("[]"), []byte{}, nil).Times(1)
	releases, err = runner.HelmList(ctx, mockConn, HelmListOptions{}); assert.NoError(t, err); assert.Empty(t, releases)
}

func TestDefaultRunner_HelmStatus(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); releaseName := "my-rel"
    sampleOutput := `{"name":"my-rel","namespace":"def","revision":"1","status":"deployed"}`
    var expected HelmReleaseInfo; json.Unmarshal([]byte(sampleOutput), &expected)
    expectedCmd := fmt.Sprintf("helm status %s -o json", shellEscape(releaseName))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte(sampleOutput), []byte{}, nil).Times(1)
    status, err := runner.HelmStatus(ctx, mockConn, releaseName, HelmStatusOptions{}); assert.NoError(t, err); assert.Equal(t, &expected, status)
}

func TestDefaultRunner_HelmRepoAdd(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); repoName, repoURL := "bn", "https://charts.bitnami.com/bitnami"
    expectedCmd := fmt.Sprintf("helm repo add %s %s", shellEscape(repoName), shellEscape(repoURL))
    mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("added"), []byte{}, nil).Times(1)
    err := runner.HelmRepoAdd(ctx, mockConn, repoName, repoURL, HelmRepoAddOptions{}); assert.NoError(t, err)
}

func TestDefaultRunner_HelmRepoUpdate(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    mockConn.EXPECT().Exec(ctx, "helm repo update", gomock.Any()).Return([]byte("updated"), []byte{}, nil).Times(1)
    err := runner.HelmRepoUpdate(ctx, mockConn, nil); assert.NoError(t, err)
}

func TestDefaultRunner_HelmVersion(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    sampleOutput := `{"version":"v3.8.1"}`; var expected HelmVersionInfo; json.Unmarshal([]byte(sampleOutput), &expected)
    mockConn.EXPECT().Exec(ctx, "helm version -o json", gomock.Any()).Return([]byte(sampleOutput), []byte{}, nil).Times(1)
    verInfo, err := runner.HelmVersion(ctx, mockConn); assert.NoError(t, err); assert.Equal(t, &expected, verInfo)
}

func TestDefaultRunner_HelmSearchRepo(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); keyword := "ng"
	sampleOutput := `[{"name":"b/n","version":"1"}]`; var expected []HelmChartInfo; json.Unmarshal([]byte(sampleOutput), &expected)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm search repo %s -o json", shellEscape(keyword)), gomock.Any()).Return([]byte(sampleOutput), []byte{}, nil).Times(1)
	charts, err := runner.HelmSearchRepo(ctx, mockConn, keyword, HelmSearchOptions{}); assert.NoError(t, err); assert.Equal(t, expected, charts)
}

func TestDefaultRunner_HelmPull(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); chartPath := "s/m"
	expectedOut := "pulled to /tmp/m.tgz"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm pull %s", shellEscape(chartPath)), gomock.Any()).Return([]byte(expectedOut+"\n"), []byte{}, nil).Times(1)
	outPath, err := runner.HelmPull(ctx, mockConn, chartPath, HelmPullOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOut, outPath)
}

func TestDefaultRunner_HelmPackage(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); chartSrcPath := "./mc"
	helmOut := "Successfully packaged chart and saved it to: /tmp/mc-0.1.0.tgz"; expectedPath := "/tmp/mc-0.1.0.tgz"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm package %s", shellEscape(chartSrcPath)), gomock.Any()).Return([]byte(helmOut+"\n"), []byte{}, nil).Times(1)
	pkgPath, err := runner.HelmPackage(ctx, mockConn, chartSrcPath, HelmPackageOptions{}); assert.NoError(t, err); assert.Equal(t, expectedPath, pkgPath)
}

func TestDefaultRunner_HelmUpgrade(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	release, chart := "myrel", "stable/mychart"
	opts := HelmUpgradeOptions{Install: true, Namespace: "upns", Version: "1.1.0"}
	expectedCmdParts := []string{"helm", "upgrade", shellEscape(release), shellEscape(chart), "--install", "--namespace", shellEscape(opts.Namespace), "--version", shellEscape(opts.Version)}
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
			for _, part := range expectedCmdParts { assert.Contains(t, cmd, part) }
			return []byte("upgraded"), []byte{}, nil
		}).Times(1)
	err := runner.HelmUpgrade(ctx, mockConn, release, chart, opts); assert.NoError(t, err)
}

func TestDefaultRunner_HelmRollback(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	release, rev := "myrel", 2
	opts := HelmRollbackOptions{Namespace: "rbns", DryRun: true}
	expectedCmd := fmt.Sprintf("helm rollback %s %d --namespace %s --dry-run", shellEscape(release), rev, shellEscape(opts.Namespace))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return([]byte("rolled back"), []byte{}, nil).Times(1)
	err := runner.HelmRollback(ctx, mockConn, release, rev, opts); assert.NoError(t, err)
}

func TestDefaultRunner_HelmHistory(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); release := "histrel"
	sampleOutput := `[{"revision":1,"status":"superseded"},{"revision":2,"status":"deployed"}]`
	var expected []HelmReleaseRevisionInfo; json.Unmarshal([]byte(sampleOutput), &expected)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm history %s -o json", shellEscape(release)), gomock.Any()).Return([]byte(sampleOutput), []byte{}, nil).Times(1)
	hist, err := runner.HelmHistory(ctx, mockConn, release, HelmHistoryOptions{}); assert.NoError(t, err); assert.Equal(t, expected, hist)
}

func TestDefaultRunner_HelmGetValues(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); release := "getvals"
	expectedOutput := "key: value\nreplicaCount: 2"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm get values %s -o yaml", shellEscape(release)), gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	vals, err := runner.HelmGetValues(ctx, mockConn, release, HelmGetOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOutput, vals)
}

func TestDefaultRunner_HelmGetManifest(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); release := "getman"
	expectedOutput := "apiVersion: v1\nkind: Pod..."
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm get manifest %s", shellEscape(release)), gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	man, err := runner.HelmGetManifest(ctx, mockConn, release, HelmGetOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOutput, man)
}

func TestDefaultRunner_HelmGetHooks(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); release := "gethooks"
	expectedOutput := "--- # Source: mychart/templates/post-install-hook.yaml\napiVersion: batch/v1..."
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm get hooks %s", shellEscape(release)), gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	hooks, err := runner.HelmGetHooks(ctx, mockConn, release, HelmGetOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOutput, hooks)
}

func TestDefaultRunner_HelmTemplate(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); release, chart := "tplrel", "./mychart"
	expectedOutput := "apiVersion: v1\nkind: Service..."
	opts := HelmTemplateOptions{ValuesFiles: []string{"v.yaml"}, SetValues: []string{"name=override"}}
	expectedCmdParts := []string{"helm", "template", shellEscape(release), shellEscape(chart), "--values", shellEscape("v.yaml"), "--set", shellEscape("name=override")}
	mockConn.EXPECT().Exec(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd string, _ *connector.ExecOptions) ([]byte, []byte, error) {
			for _, part := range expectedCmdParts { assert.Contains(t, cmd, part) }
			return []byte(expectedOutput), []byte{}, nil
		}).Times(1)
	tpl, err := runner.HelmTemplate(ctx, mockConn, release, chart, opts); assert.NoError(t, err); assert.Equal(t, expectedOutput, tpl)
}

func TestDefaultRunner_HelmDependencyUpdate(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); chartPath := "./dependent-chart"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm dependency update %s", shellEscape(chartPath)), gomock.Any()).Return([]byte("updated deps"), []byte{}, nil).Times(1)
	err := runner.HelmDependencyUpdate(ctx, mockConn, chartPath, HelmDependencyOptions{}); assert.NoError(t, err)
}

func TestDefaultRunner_HelmLint(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); chartPath := "./lintable-chart"
	expectedOutput := "[INFO] Chart is good"
	// Helm lint can return non-zero on warnings/errors. We check output.
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm lint %s", shellEscape(chartPath)), gomock.Any()).Return([]byte(expectedOutput), []byte{}, nil).Times(1)
	lintOut, err := runner.HelmLint(ctx, mockConn, chartPath, HelmLintOptions{}); assert.NoError(t, err); assert.Equal(t, expectedOutput, lintOut)

	// Test lint with errors (non-zero exit)
	lintErrOutput := "[ERROR] Something is wrong"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("helm lint %s", shellEscape(chartPath)), gomock.Any()).
		Return([]byte(""), []byte(lintErrOutput), &connector.CommandError{ExitCode: 1}).Times(1)
	lintOut, err = runner.HelmLint(ctx, mockConn, chartPath, HelmLintOptions{});
	assert.Error(t, err) // Expect an error because helm lint exited non-zero
	assert.Contains(t, lintOut, lintErrOutput) // Output should still be returned
}
