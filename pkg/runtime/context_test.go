package runtime

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
)

func TestContext_ArtifactPathHelpers(t *testing.T) {
	clusterName := "my-test-cluster"
	globalWD := "/tmp/kubexm_test_root"

	rtCtx := &Context{
		GlobalWorkDir: globalWD,
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
		},
		// ClusterArtifactsDir would be set by builder normally
	}
	// Manually set ClusterArtifactsDir as builder would
	rtCtx.ClusterArtifactsDir = filepath.Join(globalWD, clusterName)

	require.Equal(t, rtCtx.ClusterArtifactsDir, rtCtx.GetClusterArtifactsDir())

	// Test GetCertsDir
	expectedCertsDir := filepath.Join(globalWD, clusterName, common.DefaultCertsDir)
	assert.Equal(t, expectedCertsDir, rtCtx.GetCertsDir())

	// Test GetEtcdCertsDir
	expectedEtcdCertsDir := filepath.Join(expectedCertsDir, common.DefaultEtcdDir)
	assert.Equal(t, expectedEtcdCertsDir, rtCtx.GetEtcdCertsDir())

	// Test GetComponentArtifactsDir
	compName := "testcomp"
	expectedCompArtifactsDir := filepath.Join(globalWD, clusterName, compName)
	assert.Equal(t, expectedCompArtifactsDir, rtCtx.GetComponentArtifactsDir(compName))

	// Test GetEtcdArtifactsDir
	expectedEtcdArtifactsDir := filepath.Join(globalWD, clusterName, common.DefaultEtcdDir)
	assert.Equal(t, expectedEtcdArtifactsDir, rtCtx.GetEtcdArtifactsDir())

	// Test GetContainerRuntimeArtifactsDir
	expectedCRArtifactsDir := filepath.Join(globalWD, clusterName, common.DefaultContainerRuntimeDir)
	assert.Equal(t, expectedCRArtifactsDir, rtCtx.GetContainerRuntimeArtifactsDir())

	// Test GetKubernetesArtifactsDir
	expectedK8sArtifactsDir := filepath.Join(globalWD, clusterName, common.DefaultKubernetesDir)
	assert.Equal(t, expectedK8sArtifactsDir, rtCtx.GetKubernetesArtifactsDir())

	// Test GetFileDownloadPath
	compV1 := "etcd"
	verV1 := "v3.5.0"
	archV1 := "amd64"
	fileV1 := "etcd.tar.gz"
	expectedPathV1 := filepath.Join(globalWD, clusterName, compV1, verV1, archV1, fileV1)
	assert.Equal(t, expectedPathV1, rtCtx.GetFileDownloadPath(compV1, verV1, archV1, fileV1))

	compV2 := "containerd"
	verV2 := "1.7.1"
	// archV2 empty
	fileV2 := "containerd.zip"
	expectedPathV2 := filepath.Join(globalWD, clusterName, compV2, verV2, fileV2) // Arch part should be omitted
	assert.Equal(t, expectedPathV2, rtCtx.GetFileDownloadPath(compV2, verV2, "", fileV2))

	compV3 := "kubectl"
	// verV3 empty
	archV3 := "arm64"
	fileV3 := "kubectl"
	expectedPathV3 := filepath.Join(globalWD, clusterName, compV3, archV3, fileV3) // Version part should be omitted
	assert.Equal(t, expectedPathV3, rtCtx.GetFileDownloadPath(compV3, "", archV3, fileV3))

	compV4 := "cni-plugins"
	// verV4 empty, archV4 empty
	fileV4 := "cni-plugins-linux-amd64-v1.1.1.tgz" // Filename might contain version/arch info itself
	expectedPathV4 := filepath.Join(globalWD, clusterName, compV4, fileV4)
	assert.Equal(t, expectedPathV4, rtCtx.GetFileDownloadPath(compV4, "", "", fileV4))

	// Test GetHostDir
	hostName1 := "node-control-plane"
	expectedHostDir1 := filepath.Join(globalWD, clusterName, hostName1) // Corrected to include clusterName
	assert.Equal(t, expectedHostDir1, rtCtx.GetHostDir(hostName1))
}

// --- Helper to create a basic context for testing cache hierarchy ---
func newTestContextWithCaches() *Context {
	// For these tests, we don't need a full RuntimeBuilder setup,
	// just a Context with its caches initialized as they would be by the builder.
	pipelineCache := cache.NewPipelineCache()
	moduleCache := cache.NewModuleCache()
	taskCache := cache.NewTaskCache()
	stepCache := cache.NewStepCache()

	// Simulate how RuntimeBuilder would initialize them if they were standalone
	// (though in the new derivation logic, only pipelineCache is set initially,
	// and others are created and parented during derivation).
	// For testing the derivation, we start with a context that has PipelineCache.

	l, _ := logger.NewLogger(logger.DefaultOptions())


	return &Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cache-test-cluster"}},
		PipelineCache: pipelineCache,
		// ModuleCache, TaskCache, StepCache will be set by derivation methods
	}
}

func TestContext_CacheHierarchyAndScoping(t *testing.T) {
	baseCtx := newTestContextWithCaches()

	// 1. Test PipelineCache (top level)
	baseCtx.GetPipelineCache().Set("pipeKey", "pipeValue")
	baseCtx.GetPipelineCache().Set("overrideKey", "pipeOriginal")

	val, ok := baseCtx.GetPipelineCache().Get("pipeKey")
	assert.True(t, ok && val == "pipeValue", "PipelineCache Get failed")

	// 2. Derive ModuleContext and test ModuleCache
	moduleCtx := baseCtx.ForModule().(ModuleContext) // Assert to specific interface for clarity

	// Check inherited read from PipelineCache
	val, ok = moduleCtx.GetModuleCache().Get("pipeKey")
	assert.True(t, ok && val == "pipeValue", "ModuleCache should inherit pipeKey from PipelineCache")

	// Set value in ModuleCache
	moduleCtx.GetModuleCache().Set("moduleKey", "moduleValue")
	moduleCtx.GetModuleCache().Set("overrideKey", "moduleOverride") // Override

	val, ok = moduleCtx.GetModuleCache().Get("moduleKey")
	assert.True(t, ok && val == "moduleValue", "ModuleCache Get for moduleKey failed")
	val, ok = moduleCtx.GetModuleCache().Get("overrideKey")
	assert.True(t, ok && val == "moduleOverride", "ModuleCache Get for overrideKey (module local) failed")

	// Ensure PipelineCache is not affected by ModuleCache Set
	val, ok = baseCtx.GetPipelineCache().Get("moduleKey")
	assert.False(t, ok, "PipelineCache should not see moduleKey")
	val, ok = baseCtx.GetPipelineCache().Get("overrideKey")
	assert.True(t, ok && val == "pipeOriginal", "PipelineCache overrideKey should remain pipeOriginal")

	// 3. Derive TaskContext and test TaskCache
	// moduleCtx here is the one derived from baseCtx. We need its underlying *Context type for ForTask.
	taskCtx := moduleCtx.(*Context).ForTask().(TaskContext)

	// Check inherited reads
	val, ok = taskCtx.GetTaskCache().Get("pipeKey")
	assert.True(t, ok && val == "pipeValue", "TaskCache should inherit pipeKey")
	val, ok = taskCtx.GetTaskCache().Get("moduleKey")
	assert.True(t, ok && val == "moduleValue", "TaskCache should inherit moduleKey")
	val, ok = taskCtx.GetTaskCache().Get("overrideKey") // Should get from ModuleCache
	assert.True(t, ok && val == "moduleOverride", "TaskCache should get overrideKey from ModuleCache")

	// Set value in TaskCache
	taskCtx.GetTaskCache().Set("taskKey", "taskValue")
	taskCtx.GetTaskCache().Set("overrideKey", "taskOverride") // Override again

	val, ok = taskCtx.GetTaskCache().Get("taskKey")
	assert.True(t, ok && val == "taskValue", "TaskCache Get for taskKey failed")
	val, ok = taskCtx.GetTaskCache().Get("overrideKey")
	assert.True(t, ok && val == "taskOverride", "TaskCache Get for overrideKey (task local) failed")

	// Ensure ModuleCache is not affected
	val, ok = moduleCtx.GetModuleCache().Get("taskKey")
	assert.False(t, ok, "ModuleCache should not see taskKey")
	val, ok = moduleCtx.GetModuleCache().Get("overrideKey")
	assert.True(t, ok && val == "moduleOverride", "ModuleCache overrideKey should remain moduleOverride")


	// 4. Derive StepContext and test StepCache
	mockHost := connector.NewHostFromSpec(v1alpha1.HostSpec{Name: "test-host"})
	// taskCtx here is the one derived from moduleCtx. We need its underlying *Context for ForStep.
	stepCtx := taskCtx.(*Context).ForStep(mockHost).(StepContext)

	// Check inherited reads
	val, ok = stepCtx.GetStepCache().Get("pipeKey")
	assert.True(t, ok && val == "pipeValue", "StepCache should inherit pipeKey")
	val, ok = stepCtx.GetStepCache().Get("moduleKey")
	assert.True(t, ok && val == "moduleValue", "StepCache should inherit moduleKey")
	val, ok = stepCtx.GetStepCache().Get("taskKey")
	assert.True(t, ok && val == "taskValue", "StepCache should inherit taskKey")
	val, ok = stepCtx.GetStepCache().Get("overrideKey") // Should get from TaskCache
	assert.True(t, ok && val == "taskOverride", "StepCache should get overrideKey from TaskCache")

	// Set value in StepCache
	stepCtx.GetStepCache().Set("stepKey", "stepValue")
	val, ok = stepCtx.GetStepCache().Get("stepKey")
	assert.True(t, ok && val == "stepValue", "StepCache Get for stepKey failed")

	// Ensure TaskCache is not affected
	val, ok = taskCtx.GetTaskCache().Get("stepKey")
	assert.False(t, ok, "TaskCache should not see stepKey")

	// Test GetHost in StepContext
	assert.Equal(t, mockHost.GetName(), stepCtx.GetHost().GetName(), "StepContext GetHost did not return correct host")
}

func TestContext_LazyCacheInitialization(t *testing.T) {
	baseCtx := newTestContextWithCaches() // Only PipelineCache is really set here

	// 1. Access ModuleCache on baseCtx (should be lazily created and parented to PipelineCache)
	mc := baseCtx.GetModuleCache()
	require.NotNil(t, mc, "ModuleCache should be lazily initialized")

	// Verify parenting by trying to get a pipeline key
	baseCtx.GetPipelineCache().Set("lazyPipeKey", "lazyPipeValue")
	val, ok := mc.Get("lazyPipeKey")
	assert.True(t, ok && val == "lazyPipeValue", "Lazily initialized ModuleCache should inherit from PipelineCache")

	mc.Set("lazyModuleKey", "lazyModuleValue")

	// 2. Access TaskCache on the same baseCtx (which now has a ModuleCache)
	// This GetTaskCache call will use the baseCtx.ModuleCache as parent.
	tc := baseCtx.GetTaskCache()
	require.NotNil(t, tc, "TaskCache should be lazily initialized")

	val, ok = tc.Get("lazyModuleKey")
	assert.True(t, ok && val == "lazyModuleValue", "Lazily initialized TaskCache should inherit from ModuleCache")
	val, ok = tc.Get("lazyPipeKey")
	assert.True(t, ok && val == "lazyPipeValue", "Lazily initialized TaskCache should inherit from PipelineCache via ModuleCache")

	tc.Set("lazyTaskKey", "lazyTaskValue")

	// 3. Access StepCache on the same baseCtx
	sc := baseCtx.GetStepCache()
	require.NotNil(t, sc, "StepCache should be lazily initialized")

	val, ok = sc.Get("lazyTaskKey")
	assert.True(t, ok && val == "lazyTaskValue", "Lazily initialized StepCache should inherit from TaskCache")
}
