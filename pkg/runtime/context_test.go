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
	expectedHostDir1 := filepath.Join(globalWD, hostName1)
	assert.Equal(t, expectedHostDir1, rtCtx.GetHostDir(hostName1))
}
