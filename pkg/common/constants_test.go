package common

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 22, DefaultSSHPort)
	assert.Equal(t, 30*time.Second, DefaultConnectionTimeout)
	assert.Equal(t, "amd64", DefaultArch)
	assert.Contains(t, SupportedArches, "amd64")
	assert.Contains(t, SupportedArches, "arm64")
	assert.Contains(t, ValidTaintEffects, "NoSchedule")
	assert.Equal(t, "kubexm", ClusterTypeKubeXM)
	assert.Equal(t, "kubeadm", ClusterTypeKubeadm)
}

func TestDomainValidationRegexConstant(t *testing.T) {
	expectedDomainRegex := `^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`
	assert.Equal(t, expectedDomainRegex, DomainValidationRegexString)
}

func TestCgroupAndProxyModeConstants(t *testing.T) {
	assert.Equal(t, "systemd", CgroupDriverSystemd)
	assert.Equal(t, "cgroupfs", CgroupDriverCgroupfs)
	assert.Equal(t, "iptables", KubeProxyModeIPTables)
	assert.Equal(t, "ipvs", KubeProxyModeIPVS)
}

func TestSpecialRoleNameConstants(t *testing.T) {
	assert.Equal(t, "all", AllHostsRole)
	assert.Equal(t, "control-node", ControlNodeRole)
	assert.Equal(t, "kubexm-control-node", ControlNodeHostName)
}

func TestKernelModulesConstants(t *testing.T) {
	assert.Equal(t, "br_netfilter", KernelModuleBrNetfilter)
	assert.Equal(t, "ip_vs", KernelModuleIpvs)
}

func TestPreflightDefaultsConstants(t *testing.T) {
	assert.Equal(t, 2, DefaultMinCPUCores)
	assert.Equal(t, uint64(2048), DefaultMinMemoryMB)
}

func TestFilePermissionConstants(t *testing.T) {
	assert.Equal(t, os.FileMode(0755), DefaultDirPermission)
	assert.Equal(t, os.FileMode(0644), DefaultFilePermission)
	assert.Equal(t, os.FileMode(0600), DefaultKubeconfigPermission)
	assert.Equal(t, os.FileMode(0600), DefaultPrivateKeyPermission)
}

func TestMiscDefaultValuesConstants(t *testing.T) {
	assert.Equal(t, "registry.k8s.io", DefaultImageRegistry)
	assert.Equal(t, "pause", PauseImageName)
	assert.Equal(t, ".kubexm", DefaultWorkDirName)
	assert.Equal(t, ".kubexm_tmp", DefaultTmpDirName)
	assert.Equal(t, "/var/lib/etcd", EtcdDefaultDataDir)
	assert.Equal(t, "ca.pem", CACertFileName)
	assert.Equal(t, DefaultK8sVersion, "v1.28.2")
	assert.Equal(t, DefaultEtcdVersion, "3.5.10-0")
	assert.Equal(t, DefaultKeepalivedAuthPass, "kxm_pass")
	assert.Equal(t, DefaultRemoteWorkDir, "/tmp/kubexms_work")
	assert.Equal(t, "/var/lib/docker", DockerDefaultDataRoot)
}

func TestSocketPathConstants(t *testing.T) {
	assert.Equal(t, "unix:///run/containerd/containerd.sock", ContainerdSocketPath)
	assert.Equal(t, "unix:///var/run/docker.sock", DockerSocketPath)
	assert.Equal(t, "/var/run/cri-dockerd.sock", CriDockerdSocketPath)
}

func TestDockerDefaultsConstants(t *testing.T) {
	assert.Equal(t, "100m", DockerLogOptMaxSizeDefault)
	assert.Equal(t, "5", DockerLogOptMaxFileDefault)
	assert.Equal(t, 3, DockerMaxConcurrentDownloadsDefault)
	assert.Equal(t, 5, DockerMaxConcurrentUploadsDefault)
	assert.Equal(t, "docker0", DefaultDockerBridgeName)
	assert.Equal(t, "json-file", DockerLogDriverJSONFile)
}

func TestSELinuxAndIPTablesModeConstants(t *testing.T) {
	assert.Equal(t, "permissive", DefaultSELinuxMode)
	assert.Equal(t, "legacy", DefaultIPTablesMode)
	assert.Contains(t, ValidSELinuxModes, "permissive")
	assert.Contains(t, ValidIPTablesModes, "legacy")
}

func TestEtcdFileAndBinPathConstants(t *testing.T) {
	assert.Equal(t, "/etc/kubernetes/pki/etcd", EtcdDefaultPKIDir)
	assert.Equal(t, "server.pem", EtcdServerCertFileName)
	assert.Equal(t, "server-key.pem", EtcdServerKeyFileName)
	assert.Equal(t, "peer.pem", EtcdPeerCertFileName)
	assert.Equal(t, "peer-key.pem", EtcdPeerKeyFileName)
	assert.Equal(t, "client.pem", EtcdClientCertFileName)
	assert.Equal(t, "client-key.pem", EtcdClientKeyFileName)
	assert.Equal(t, "/usr/local/bin", EtcdDefaultBinDir)
	assert.Equal(t, "v3.5.13", DefaultEtcdVersionForBinInstall)
}

func TestContainerdMiscConstants(t *testing.T) {
	assert.Equal(t, "cri", ContainerdPluginCRI)
	assert.Equal(t, "1.7.11", DefaultContainerdVersion)
}

func TestStringEnumLikeConstantsForTypes(t *testing.T) {
	// These test the string values that might be used before full adoption of typed enums.
	assert.Equal(t, "calico", CNICalicoStr)
	assert.Equal(t, "docker", RuntimeDockerStr)
}
