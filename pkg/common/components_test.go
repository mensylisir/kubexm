package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentConstants(t *testing.T) {
	t.Run("ComponentNames", func(t *testing.T) {
		assert.Equal(t, "kube-apiserver", KubeAPIServer)
		assert.Equal(t, "kube-controller-manager", KubeControllerManager)
		assert.Equal(t, "kube-scheduler", KubeScheduler)
		assert.Equal(t, "kubelet", Kubelet)
		assert.Equal(t, "kube-proxy", KubeProxy)
		assert.Equal(t, "etcd", Etcd)
		assert.Equal(t, "etcdctl", Etcdctl)
		assert.Equal(t, "containerd", Containerd)
		assert.Equal(t, "docker", Docker)
		assert.Equal(t, "runc", Runc)
		assert.Equal(t, "cri-dockerd", CniDockerd)
		assert.Equal(t, "kubeadm", Kubeadm)
		assert.Equal(t, "kubectl", Kubectl)
		assert.Equal(t, "keepalived", Keepalived)
		assert.Equal(t, "haproxy", HAProxy)
		assert.Equal(t, "nginx", Nginx) // Added test
		assert.Equal(t, "kube-vip", KubeVIP)
		assert.Equal(t, "calicoctl", Calicoctl)         // Added test
		assert.Equal(t, "helm", Helm)                   // Added test
		assert.Equal(t, "crictl", Crictl)               // Added test
		assert.Equal(t, "node-local-dns", NodeLocalDNS) // Added test
	})

	t.Run("ServiceNames", func(t *testing.T) {
		assert.Equal(t, "kubelet.service", KubeletServiceName)
		assert.Equal(t, "containerd.service", ContainerdServiceName)
		assert.Equal(t, "docker.service", DockerServiceName)
		assert.Equal(t, "etcd.service", EtcdServiceName)
		assert.Equal(t, "cri-dockerd.service", CniDockerdServiceName)
		assert.Equal(t, "keepalived.service", KeepalivedServiceName)
		assert.Equal(t, "haproxy.service", HAProxyServiceName)
		assert.Equal(t, "nginx.service", NginxServiceName)                   // Added test
		assert.Equal(t, "crio.service", CrioServiceName)                     // Added test
		assert.Equal(t, "isulad.service", IsuladServiceName)                 // Added test
		assert.Equal(t, "etcd-defrag.timer", EtcdDefragTimerServiceName)     // Added test
		assert.Equal(t, "etcd-defrag.service", EtcdDefragSystemdServiceName) // Added test
	})

	t.Run("DefaultPorts", func(t *testing.T) {
		assert.Equal(t, 6443, KubeAPIServerDefaultPort)
		assert.Equal(t, 10259, KubeSchedulerDefaultPort)
		assert.Equal(t, 10257, KubeControllerManagerDefaultPort)
		assert.Equal(t, 10250, KubeletDefaultPort)
		assert.Equal(t, 2379, EtcdDefaultClientPort)
		assert.Equal(t, 2380, EtcdDefaultPeerPort)
		assert.Equal(t, 6443, HAProxyDefaultFrontendPort)
		assert.Equal(t, 9153, CoreDNSMetricsPort)      // Added test
		assert.Equal(t, 9253, NodeLocalDNSMetricsPort) // Added test
		assert.Equal(t, 10249, KubeProxyMetricsPort)   // Added test
		assert.Equal(t, 10256, KubeProxyHealthzPort)   // Added test
	})

	t.Run("CommonToolsAndUtils", func(t *testing.T) {
		assert.Equal(t, "socat", Socat)
		assert.Equal(t, "conntrack-tools", Conntrack) // Updated expected value
		assert.Equal(t, "ipset", IPSet)
		assert.Equal(t, "ipvsadm", Ipvsadm)
		assert.Equal(t, "nfs-utils", NfsUtils)
		assert.Equal(t, "ceph-common", CephCommon)
		assert.Equal(t, "curl", Curl)       // Added test
		assert.Equal(t, "pgrep", Pgrep)     // Added test
		assert.Equal(t, "killall", Killall) // Added test

	})

	t.Run("ImageRepositoriesAndVersions", func(t *testing.T) {
		assert.Equal(t, "registry.k8s.io", DefaultK8sImageRegistry) // Added test
		assert.Equal(t, "registry.k8s.io/coredns", DefaultCoreDNSImageRepository)
		assert.Equal(t, "registry.k8s.io", DefaultPauseImageRepository)             // Corrected, was "registry.k8s.io/pause"
		assert.Equal(t, "ghcr.io/kube-vip", DefaultKubeVIPImageRepository)          // Added test
		assert.Equal(t, "docker.io/library/haproxy", DefaultHAProxyImageRepository) // Added test
		assert.Equal(t, "docker.io/library/nginx", DefaultNginxImageRepository)     // Added test

		// DefaultPauseImageVersion and DefaultCoreDNSVersion are in constants.go, not components.go
		// DefaultKubeVIPImage in constants.go includes the version.
	})
}
