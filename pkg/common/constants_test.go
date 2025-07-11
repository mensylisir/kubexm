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
	assert.Equal(t, "registry.k8s.io", DefaultK8sImageRegistry) // Updated from DefaultImageRegistry
	assert.Equal(t, "pause", PauseImageName)
	assert.Equal(t, ".kubexm", DefaultWorkDirName) // This is KubexmRootDirName in paths.go or DefaultWorkDirName in workdirs.go
	assert.Equal(t, ".kubexm_tmp", DefaultTmpDirName)
	assert.Equal(t, "/var/lib/etcd", EtcdDefaultDataDirTarget) // Updated from EtcdDefaultDataDir
	assert.Equal(t, "ca.crt", CACertFileName)                 // Updated from ca.pem to ca.crt
	assert.Equal(t, DefaultK8sVersion, "v1.28.2")
	assert.Equal(t, DefaultEtcdVersion, "3.5.10-0")
	assert.Equal(t, DefaultKeepalivedAuthPass, "kxm_pass")
	assert.Equal(t, DefaultRemoteWorkDir, "/tmp/kubexms_work")
	assert.Equal(t, "/var/lib/docker", DockerDefaultDataRoot)
}

func TestPathConstantsWorkdirs(t *testing.T) {
	// Test for constants that were in constants.go but moved to workdirs.go or paths.go
	assert.Equal(t, ".kubexm", KubexmRootDirName) // From paths.go, was also like DefaultWorkDirName
	assert.Equal(t, DefaultWorkDirName, ".kubexm") // From workdirs.go
	assert.Equal(t, DefaultRemoteWorkDir, "/tmp/kubexms_work")
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
	assert.Equal(t, "/etc/etcd/pki", EtcdDefaultPKIDirTarget) // Updated from EtcdDefaultPKIDir and path
	assert.Equal(t, "server.crt", EtcdServerCertFileName)       // .crt now
	assert.Equal(t, "server.key", EtcdServerKeyFileName)       // .key now
	assert.Equal(t, "peer.crt", EtcdPeerCertFileName)           // .crt now
	assert.Equal(t, "peer.key", EtcdPeerKeyFileName)           // .key now
	assert.Equal(t, "admin.crt", EtcdAdminClientCertFileName)  // Updated name and .crt
	assert.Equal(t, "admin.key", EtcdAdminClientKeyFileName)   // Updated name and .key
	assert.Equal(t, "/usr/local/bin", DefaultBinDir)          // Updated from EtcdDefaultBinDir
	assert.Equal(t, "v3.5.13", DefaultEtcdVersionForBinInstall)
}

func TestContainerdMiscConstants(t *testing.T) {
	assert.Equal(t, "cri", ContainerdPluginCRI)
	assert.Equal(t, "1.7.11", DefaultContainerdVersion)
}

func TestStringEnumLikeConstantsForTypes(t *testing.T) {
	// These test the string values that might be used before full adoption of typed enums.
	assert.Equal(t, "calico", CNICalico)       // Updated from CNICalicoStr
	assert.Equal(t, "docker", RuntimeDocker)     // Updated from RuntimeDockerStr
}
