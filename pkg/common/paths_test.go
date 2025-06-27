package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathConstants(t *testing.T) {
	t.Run("GeneralDefaultDirectories", func(t *testing.T) {
		assert.Equal(t, "certs", DefaultCertsDir)
		assert.Equal(t, "container_runtime", DefaultContainerRuntimeDir)
		assert.Equal(t, "kubernetes", DefaultKubernetesDir)
		assert.Equal(t, "etcd", DefaultEtcdDir)
	})

	t.Run("KubexmWorkDirectories", func(t *testing.T) {
		assert.Equal(t, "bin", DefaultBinDir)
		assert.Equal(t, "conf", DefaultConfDir)
		assert.Equal(t, "scripts", DefaultScriptsDir)
		assert.Equal(t, "backup", DefaultBackupDir)
	})

	t.Run("KubernetesSystemDirectories", func(t *testing.T) {
		assert.Equal(t, "/var/lib/kubelet", KubeletHomeDir)
		assert.Equal(t, "/etc/kubernetes", KubernetesConfigDir)
		assert.Equal(t, "/etc/kubernetes/manifests", KubernetesManifestsDir)
		assert.Equal(t, "/etc/kubernetes/pki", KubernetesPKIDir)
		assert.Equal(t, "/root/.kube/config", DefaultKubeconfigPath)
	})

	t.Run("KubernetesPKIFileNames", func(t *testing.T) {
		assert.Equal(t, "ca.crt", CACertFileName)
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

	t.Run("KubernetesConfigFiles", func(t *testing.T) {
		assert.Equal(t, "kubeadm-config.yaml", KubeadmConfigFileName)
		assert.Equal(t, "kubelet.conf", KubeletConfigFileName)
		assert.Equal(t, "10-kubeadm.conf", KubeletSystemdEnvFileName)
		assert.Equal(t, "controller-manager.conf", ControllerManagerConfigFileName)
		assert.Equal(t, "scheduler.conf", SchedulerConfigFileName)
		assert.Equal(t, "admin.conf", AdminConfigFileName)
	})

	t.Run("StaticPodManifests", func(t *testing.T) {
		assert.Equal(t, "kube-apiserver.yaml", KubeAPIServerStaticPodFileName)
		assert.Equal(t, "kube-controller-manager.yaml", KubeControllerManagerStaticPodFileName)
		assert.Equal(t, "kube-scheduler.yaml", KubeSchedulerStaticPodFileName)
		assert.Equal(t, "etcd.yaml", EtcdStaticPodFileName)
	})

	t.Run("EtcdSystemDirectoriesFiles", func(t *testing.T) {
		assert.Equal(t, "/var/lib/etcd", EtcdDefaultDataDir)
		assert.Equal(t, "/var/lib/etcd/wal", EtcdDefaultWalDir)
		assert.Equal(t, "/etc/etcd", EtcdDefaultConfDir)
		assert.Equal(t, "/etc/etcd/pki", EtcdDefaultPKIDir)
		assert.Equal(t, "/usr/local/bin", EtcdDefaultBinDir)
		assert.Equal(t, "/etc/systemd/system/etcd.service", EtcdDefaultSystemdFile)
		assert.Equal(t, "/etc/etcd/etcd.conf.yml", EtcdDefaultConfFile)
	})

	t.Run("EtcdPKIFiles", func(t *testing.T) {
		assert.Equal(t, "server.crt", EtcdServerCert)
		assert.Equal(t, "server.key", EtcdServerKey)
		assert.Equal(t, "peer.crt", EtcdPeerCert)
		assert.Equal(t, "peer.key", EtcdPeerKey)
	})

	t.Run("ContainerRuntimePaths", func(t *testing.T) {
		assert.Equal(t, "/etc/containerd", ContainerdDefaultConfDir)
		assert.Equal(t, "/etc/containerd/config.toml", ContainerdDefaultConfigFile)
		assert.Equal(t, "/run/containerd/containerd.sock", ContainerdDefaultSocketPath)
		assert.Equal(t, "/etc/systemd/system/containerd.service", ContainerdDefaultSystemdFile)
		assert.Equal(t, "/etc/docker", DockerDefaultConfDir)
		assert.Equal(t, "/etc/docker/daemon.json", DockerDefaultConfigFile)
		assert.Equal(t, "/var/lib/docker", DockerDefaultDataRoot)
		assert.Equal(t, "/var/run/docker.sock", DockerDefaultSocketPath)
		assert.Equal(t, "/lib/systemd/system/docker.service", DockerDefaultSystemdFile)
		assert.Equal(t, "/var/run/cri-dockerd.sock", CniDockerdSocketPath)
		assert.Equal(t, "/etc/systemd/system/cri-dockerd.service", CniDockerdSystemdFile)
	})

	t.Run("HAComponentPaths", func(t *testing.T) {
		assert.Equal(t, "/etc/keepalived", KeepalivedDefaultConfDir)
		assert.Equal(t, "/etc/keepalived/keepalived.conf", KeepalivedDefaultConfigFile)
		assert.Equal(t, "/etc/systemd/system/keepalived.service", KeepalivedDefaultSystemdFile)
		assert.Equal(t, "/etc/haproxy", HAProxyDefaultConfDir)
		assert.Equal(t, "/etc/haproxy/haproxy.cfg", HAProxyDefaultConfigFile)
		assert.Equal(t, "/etc/systemd/system/haproxy.service", HAProxyDefaultSystemdFile)
		assert.Equal(t, "kube-vip.yaml", KubeVIPManifestFileName)
	})

	t.Run("SystemConfigPaths", func(t *testing.T) {
		assert.Equal(t, "/etc/sysctl.conf", SysctlDefaultConfFile)
		assert.Equal(t, "/etc/modules-load.d", ModulesLoadDefaultDir)
		assert.Equal(t, "/etc/sysctl.d/99-kubernetes-cri.conf", KubernetesSysctlConfFile)
		assert.Equal(t, "/etc/systemd/system/kubelet.service.d", KubeletSystemdDropinDir)
	})
}
