package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time" // For ManualMockRunner.WaitForPort (eventually) & newTestStepEtcdContext

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache" // For Cache interface
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step" // For StepContext interface
	"github.com/stretchr/testify/assert"
	// "text/template" // For ManualMockRunner.Render (eventually)
	// "github.com/mensylisir/kubexm/pkg/mock" // Removed
	// "github.com/mensylisir/kubexm/pkg/runtime" // Removed
	// testmock "github.com/stretchr/testify/mock" // No longer needed with ManualMockRunner
)

// ManualMockRunner is a manual mock for runner.Runner.
type ManualMockRunner struct {
	// ExpectedCalls map[string]interface{} // For more complex manual mocking if needed
	// ActualCalls []string
}


// MARKER_BEFORE_EXISTS
func (m *ManualMockRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	// This logic needs to be controlled by test setup if specific behavior is needed.
	if strings.Contains(path, "nonexistent") { return false, nil }

	// For Precheck_AlreadyInstalled & Run, etcd/etcdctl should exist at /usr/local/bin
	if (path == "/usr/local/bin/etcd" || path == "/usr/local/bin/etcdctl") &&
	   (strings.Contains(conn.GetHost().Name, "-installed") || strings.Contains(conn.GetHost().Name, "-run")) {
		return true, nil
	}

	// For Run_Success, etcd/etcdctl should exist in the source extracted dir
	if (path == "/tmp/etcd-extracted/etcd" || path == "/tmp/etcd-extracted/etcdctl") &&
	   strings.Contains(conn.GetHost().Name, "-run") {
		return true, nil
	}

	// For Precheck_NotInstalled, etcd/etcdctl should NOT exist at /usr/local/bin
	if (path == "/usr/local/bin/etcd" || path == "/usr/local/bin/etcdctl") &&
	   strings.Contains(conn.GetHost().Name, "-not-installed") {
		return false, nil
	}

	return false, nil // Default
}
// MARKER_AFTER_EXISTS_BEFORE_MKDIRP
func (m *ManualMockRunner) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
// MARKER_AFTER_MKDIRP_BEFORE_RUNWITHOPTIONS
func (m *ManualMockRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if strings.Contains(cmd, "/usr/local/bin/etcd --version") { return []byte("etcd Version: 3.5.9"), nil, nil }
	if strings.Contains(cmd, "/usr/local/bin/etcdctl version") { return []byte("etcdctl version: 3.5.9"), nil, nil }
	if strings.Contains(cmd, "cp -fp /tmp/etcd-extracted/etcd") {return []byte("copied etcd"), nil, nil}
	if strings.Contains(cmd, "cp -fp /tmp/etcd-extracted/etcdctl") {return []byte("copied etcdctl"), nil, nil}
	return nil, nil, nil
}
// MARKER_AFTER_RUNWITHOPTIONS_BEFORE_CHMOD
func (m *ManualMockRunner) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
// MARKER_AFTER_CHMOD_BEFORE_REMOVE
func (m *ManualMockRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
// MARKER_AFTER_REMOVE


// var _ runner.Runner = (*ManualMockRunner)(nil) // Temporarily comment out to allow compilation with partial implementation

// localMockStepContextForEtcd implements step.StepContext for etcd step tests.
type localMockStepContextForEtcd struct {
	goCtx         context.Context
	logger        *logger.Logger
	currentHost   connector.Host
	mockConnector connector.Connector
	runner        runner.Runner
	clusterConfig *v1alpha1.Cluster
	facts         *runner.Facts
	internalStepCache   cache.Cache
	internalTaskCache   cache.Cache
	internalModuleCache cache.Cache
	globalWorkDir           string
	verbose                 bool
	ignoreErr               bool
	globalConnectionTimeout time.Duration
	clusterArtifactsDir     string
}

// GetCurrentHost returns the current host for the step context.
func (c *localMockStepContextForEtcd) GetCurrentHost() connector.Host { return c.currentHost }
// GetLogger returns the logger for the step context.
func (c *localMockStepContextForEtcd) GetLogger() *logger.Logger { return c.logger }
// GetRunner returns the runner for the step context.
func (c *localMockStepContextForEtcd) GetRunner() runner.Runner { return c.runner }
// GetConnectorForHost returns a connector for the given host.
func (c *localMockStepContextForEtcd) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	// For this mock, always return the mockConnector if host matches currentHost, else error.
	if host.Name == c.currentHost.Name {
		return c.mockConnector, nil
	}
	return nil, fmt.Errorf("mock context only has connector for current host %s, requested %s", c.currentHost.Name, host.Name)
}
// GetCurrentHostConnector returns the connector for the current host.
func (c *localMockStepContextForEtcd) GetCurrentHostConnector() (connector.Connector, error) {
	return c.mockConnector, nil
}
// GetClusterConfig returns the cluster configuration.
func (c *localMockStepContextForEtcd) GetClusterConfig() *v1alpha1.Cluster { return c.clusterConfig }
// GetFacts returns the facts for the current host.
func (c *localMockStepContextForEtcd) GetFacts() *runner.Facts { return c.facts }
// StepCache returns the step-level cache.
func (c *localMockStepContextForEtcd) StepCache() cache.Cache { return c.internalStepCache }
// TaskCache returns the task-level cache.
func (c *localMockStepContextForEtcd) TaskCache() cache.Cache { return c.internalTaskCache }
// ModuleCache returns the module-level cache.
func (c *localMockStepContextForEtcd) ModuleCache() cache.Cache { return c.internalModuleCache }
// GetGlobalWorkDir returns the global working directory.
func (c *localMockStepContextForEtcd) GetGlobalWorkDir() string { return c.globalWorkDir }
// GetClusterWorkDir returns the cluster-specific working directory.
func (c *localMockStepContextForEtcd) GetClusterWorkDir() string { return filepath.Join(c.globalWorkDir, c.clusterConfig.Name) }
// GetClusterArtifactsDir returns the directory for cluster artifacts.
func (c *localMockStepContextForEtcd) GetClusterArtifactsDir() string { return c.clusterArtifactsDir }
// IsVerbose returns true if verbose logging is enabled.
func (c *localMockStepContextForEtcd) IsVerbose() bool { return c.verbose }
// ShouldIgnoreErrors returns true if errors should be ignored.
func (c *localMockStepContextForEtcd) ShouldIgnoreErrors() bool { return c.ignoreErr }
// GetContext returns the underlying Go context.
func (c *localMockStepContextForEtcd) GetContext() context.Context { return c.goCtx }
// GetGlobalConnectionTimeout returns the global connection timeout.
func (c *localMockStepContextForEtcd) GetGlobalConnectionTimeout() time.Duration { return c.globalConnectionTimeout }


// newTestStepEtcdContext creates a step.StepContext for etcd step tests.
func newTestStepEtcdContext(t *testing.T, testRunner runner.Runner, currentHostName string, cacheVals map[string]interface{}) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultConfig())
	l.SetLogLevel(logger.DebugLevel)

	if currentHostName == "" {
		currentHostName = "test-etcd-host" // Default, will be overridden by specific tests
	}

	hostSpec := v1alpha1.HostSpec{Name: currentHostName, Address: "1.2.3.4", Type: "ssh", User:"test", Port:22, Roles: []string{"etcd", common.ControlNodeRole}}
	currentHost := connector.NewHostFromSpec(hostSpec)

	currentConnector := &step.MockStepConnector{Host: currentHost} // Ensure MockStepConnector knows its host

	defaultFacts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}, Hostname: currentHostName}

	clusterCfg := &v1alpha1.Cluster{
		ObjectMeta: v1alpha1.ObjectMeta{Name: "test-etcd-cluster"},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []v1alpha1.HostSpec{hostSpec},
			Global: &v1alpha1.GlobalSpec{WorkDir: "/tmp/_kubexm_etcd_step_test_work"},
		},
	}
	v1alpha1.SetDefaults_Cluster(clusterCfg)
	globalWorkDir := clusterCfg.Spec.Global.WorkDir

	ctx := &localMockStepContextForEtcd{
		goCtx:         context.Background(),
		logger:        l,
		currentHost:   currentHost,
		mockConnector: currentConnector,
		runner:        testRunner,
		clusterConfig: clusterCfg,
		facts:         defaultFacts,
		internalStepCache:   cache.NewMemoryCache(),
		internalTaskCache:   cache.NewMemoryCache(),
		internalModuleCache: cache.NewMemoryCache(),
		globalWorkDir:       globalWorkDir,
		clusterArtifactsDir: filepath.Join(globalWorkDir, clusterCfg.Name),
		verbose:             true,
		globalConnectionTimeout: 30 * time.Second,
	}

	if cacheVals != nil {
		for k, v := range cacheVals {
			ctx.TaskCache().Set(k, v)
		}
	}
	return ctx
}

func TestCopyEtcdBinariesToPathStep_Run_Success(t *testing.T) {
	mockRunner := new(ManualMockRunner)

	cacheData := make(map[string]interface{})
	cacheData[ExtractedEtcdDirCacheKey] = "/tmp/etcd-extracted"
	// Hostname for this test helps ManualMockRunner.Exists differentiate behavior
	stepCtx := newTestStepEtcdContext(t, mockRunner, "test-etcd-host-run", cacheData)

	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		ExtractedEtcdDirCacheKey,
		"/usr/local/bin",
		"3.5.9",
		true,
		true,
	)

	err := s.Run(stepCtx, stepCtx.GetCurrentHost())
	assert.NoError(t, err)
}

func TestCopyEtcdBinariesToPathStep_Precheck_AlreadyInstalled(t *testing.T) {
	mockRunner := new(ManualMockRunner)
	// Hostname for this test helps ManualMockRunner.Exists differentiate behavior
	stepCtx := newTestStepEtcdContext(t, mockRunner, "test-etcd-precheck-installed", nil)


	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		"",
		"/usr/local/bin",
		"3.5.9",
		true,
		false,
	)

	done, err := s.Precheck(stepCtx, stepCtx.GetCurrentHost())
	assert.NoError(t, err)
	assert.True(t, done, "Precheck should be done as etcd is considered installed by mock")
}

func TestCopyEtcdBinariesToPathStep_Precheck_NotInstalled(t *testing.T) {
	mockRunner := new(ManualMockRunner)
	// Hostname for this test helps ManualMockRunner.Exists differentiate behavior
	stepCtx := newTestStepEtcdContext(t, mockRunner, "test-etcd-precheck-not-installed", nil)


	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "3.5.9", true, false)

	done, err := s.Precheck(stepCtx, stepCtx.GetCurrentHost())
	assert.NoError(t, err)
	assert.False(t, done, "Precheck should not be done as etcd is considered not installed by mock")
}


func TestCopyEtcdBinariesToPathStep_Rollback(t *testing.T) {
	mockRunner := new(ManualMockRunner)
	stepCtx := newTestStepEtcdContext(t, mockRunner, "test-etcd-rollback", nil)


	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "", true, false)

	err := s.Rollback(stepCtx, stepCtx.GetCurrentHost())
	assert.NoError(t, err)
}
