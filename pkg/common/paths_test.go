package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalWorkstationPathConstants(t *testing.T) {
	assert.Equal(t, ".kubexm", KubexmRootDirName)
	assert.Equal(t, "logs", DefaultLogDirName)
	assert.Equal(t, "certs", DefaultCertsDir)
	assert.Equal(t, "artifacts", DefaultArtifactsDir)
	assert.Equal(t, "bin", DefaultBinDirName)
	assert.Equal(t, "conf", DefaultConfDirName)
	assert.Equal(t, "scripts", DefaultScriptsDirName)
	assert.Equal(t, "backup", DefaultBackupDirName)

	assert.Equal(t, "container_runtime", DefaultContainerRuntimeDir)
	assert.Equal(t, "kubernetes", DefaultKubernetesDir)
	assert.Equal(t, "etcd", DefaultEtcdDir)

	assert.Equal(t, "etcd", ArtifactsEtcdDir)
	assert.Equal(t, "kube", ArtifactsKubeDir)
	assert.Equal(t, "cni", ArtifactsCNIDir)
	assert.Equal(t, "helm", ArtifactsHelmDir)
	assert.Equal(t, "docker", ArtifactsDockerDir)
	assert.Equal(t, "containerd", ArtifactsContainerdDir)
	assert.Equal(t, "runc", ArtifactsRuncDir)
	assert.Equal(t, "crictl", ArtifactsCrictlDir)
	assert.Equal(t, "cri-dockerd", ArtifactsCriDockerdDir)
	assert.Equal(t, "calicoctl", ArtifactsCalicoctlDir)
	assert.Equal(t, "registry", ArtifactsRegistryDir)
	assert.Equal(t, "compose", ArtifactsComposeDir)
	assert.Equal(t, "build", ArtifactsBuildDir)
	assert.Equal(t, "generic", ArtifactsGenericBinariesDir)
}

func TestTargetNodePathConstants(t *testing.T) {
	t.Run("KubernetesSystemPaths", func(t *testing.T) {
		assert.Equal(t, "/var/lib/kubelet", KubeletHomeDir)
		assert.Equal(t, "/etc/kubernetes", KubernetesConfigDir)
		assert.Equal(t, "/etc/kubernetes/manifests", KubernetesManifestsDir)
		assert.Equal(t, "/etc/kubernetes/pki", KubernetesPKIDir)
		assert.Equal(t, "/root/.kube/config", DefaultKubeconfigPath)
	})

	t.Run("KubernetesPKIFileNames", func(t *testing.T) {
		assert.Equal(t, "ca.key", CAKeyFileName)
		assert.Equal(t, "apiserver.crt", APIServerCertFileName)
		assert.Equal(t, "apiserver.key", APIServerKeyFileName)
		assert.Equal(t, "apiserver-kubelet-client.crt", APIServerKubeletClientCertFileName)
		assert.Equal(t, "apiserver-kubelet-client.key", APIServerKubeletClientKeyFileName)
		assert.Equal(t, "front-proxy-ca.crt", FrontProxyCACertFileName)
		assert.Equal(t, "front-proxy-ca.key", FrontProxyCAKeyFileName)
		assert.Equal(t, "front-proxy-client.crt", FrontProxyClientCertFileName)
		assert.Equal(t, "front-proxy-client.key", FrontProxyClientKeyFileName)
		assert.Equal(t, "sa.pub", ServiceAccountPublicKeyFileName)
		assert.Equal(t, "sa.key", ServiceAccountPrivateKeyFileName)
		assert.Equal(t, "apiserver-etcd-client.crt", APIServerEtcdClientCertFileName)
		assert.Equal(t, "apiserver-etcd-client.key", APIServerEtcdClientKeyFileName)
	})

	t.Run("KubernetesConfigFileNames", func(t *testing.T) {
		assert.Equal(t, "kubeadm-config.yaml", KubeadmConfigFileName)
		assert.Equal(t, "kubelet.conf", KubeletKubeconfigFileName)
		assert.Equal(t, "10-kubeadm.conf", KubeletSystemdEnvFileName)
		assert.Equal(t, "controller-manager.conf", ControllerManagerKubeconfigFileName)
		assert.Equal(t, "scheduler.conf", SchedulerKubeconfigFileName)
		assert.Equal(t, "admin.conf", AdminKubeconfigFileName)
		assert.Equal(t, "kube-proxy.conf", KubeProxyKubeconfigFileName)
	})

	t.Run("StaticPodManifestFileNames", func(t *testing.T) {
		assert.Equal(t, "kube-apiserver.yaml", KubeAPIServerStaticPodFileName)
		assert.Equal(t, "kube-controller-manager.yaml", KubeControllerManagerStaticPodFileName)
		assert.Equal(t, "kube-scheduler.yaml", KubeSchedulerStaticPodFileName)
		assert.Equal(t, "etcd.yaml", EtcdStaticPodFileName)
	})

	t.Run("EtcdTargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/var/lib/etcd/wal", EtcdDefaultWalDir)
		assert.Equal(t, "/etc/etcd", EtcdDefaultConfDirTarget)
		assert.Equal(t, "/etc/etcd/pki", EtcdDefaultPKIDirTarget)
		assert.Equal(t, "/etc/etcd.env", EtcdEnvFileTarget)
	})

	t.Run("ContainerRuntimeTargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/containerd", ContainerdDefaultConfDirTarget)
		assert.Equal(t, "/etc/containerd/config.toml", ContainerdDefaultConfigFileTarget)
		assert.Equal(t, "/etc/systemd/system/containerd.service", ContainerdDefaultSystemdFile)
		assert.Equal(t, "/etc/docker", DockerDefaultConfDirTarget)
		assert.Equal(t, "/etc/docker/daemon.json", DockerDefaultConfigFileTarget)
		assert.Equal(t, "/lib/systemd/system/docker.service", DockerDefaultSystemdFile)
		assert.Equal(t, "/etc/systemd/system/cri-dockerd.service", CniDockerdSystemdFile)
	})

	t.Run("HAComponentTargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/keepalived", KeepalivedDefaultConfDirTarget)
		assert.Equal(t, "/etc/keepalived/keepalived.conf", KeepalivedDefaultConfigFileTarget)
		assert.Equal(t, "/etc/systemd/system/keepalived.service", KeepalivedDefaultSystemdFile)
		assert.Equal(t, "/etc/haproxy", HAProxyDefaultConfDirTarget)
		assert.Equal(t, "/etc/haproxy/haproxy.cfg", HAProxyDefaultConfigFileTarget)
		assert.Equal(t, "/etc/systemd/system/haproxy.service", HAProxyDefaultSystemdFile)
		assert.Equal(t, "kube-vip.yaml", KubeVIPManifestFileName)
	})

	t.Run("SystemConfigTargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/sysctl.conf", SysctlDefaultConfFileTarget)
		assert.Equal(t, "/etc/modules-load.d", ModulesLoadDefaultDirTarget)
		assert.Equal(t, "/etc/sysctl.d/99-kubernetes-cri.conf", KubernetesSysctlConfFileTarget)
		assert.Equal(t, "/etc/systemd/system/kubelet.service.d", KubeletSystemdDropinDirTarget)
	})

	t.Run("CNITargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/cni/net.d", DefaultCNIConfDirTarget)
		assert.Equal(t, "/opt/cni/bin", DefaultCNIBinDirTarget)
	})

	t.Run("HelmLocalPaths", func(t *testing.T) {
		assert.Equal(t, "/root/.helm", DefaultHelmHome)
		assert.Equal(t, "/root/.cache/helm", DefaultHelmCache)
	})

	t.Run("KubeletConfigTargetNodePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/kubernetes/kubelet.conf", KubeletKubeconfigPathTarget)
		assert.Equal(t, "/etc/kubernetes/bootstrap-kubelet.conf", KubeletBootstrapKubeconfigPathTarget)
		assert.Equal(t, "/var/lib/kubelet/config.yaml", KubeletConfigYAMLPathTarget)
		assert.Equal(t, "/var/lib/kubelet/kubeadm-flags.env", KubeletFlagsEnvPathTarget)
		assert.Equal(t, "/var/lib/kubelet/pki", KubeletPKIDirTarget)
	})
}
