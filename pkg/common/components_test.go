package common

import "testing"

func TestComponentConstants(t *testing.T) {
	t.Run("ComponentNames", func(t *testing.T) {
		if KubeAPIServer != "kube-apiserver" {
			t.Errorf("KubeAPIServer constant is incorrect: got %s, want kube-apiserver", KubeAPIServer)
		}
		if KubeControllerManager != "kube-controller-manager" {
			t.Errorf("KubeControllerManager constant is incorrect: got %s, want kube-controller-manager", KubeControllerManager)
		}
		if KubeScheduler != "kube-scheduler" {
			t.Errorf("KubeScheduler constant is incorrect: got %s, want kube-scheduler", KubeScheduler)
		}
		if Kubelet != "kubelet" {
			t.Errorf("Kubelet constant is incorrect: got %s, want kubelet", Kubelet)
		}
		if KubeProxy != "kube-proxy" {
			t.Errorf("KubeProxy constant is incorrect: got %s, want kube-proxy", KubeProxy)
		}
		if Etcd != "etcd" {
			t.Errorf("Etcd constant is incorrect: got %s, want etcd", Etcd)
		}
		if Etcdctl != "etcdctl" {
			t.Errorf("Etcdctl constant is incorrect: got %s, want etcdctl", Etcdctl)
		}
		if Containerd != "containerd" {
			t.Errorf("Containerd constant is incorrect: got %s, want containerd", Containerd)
		}
		if Docker != "docker" {
			t.Errorf("Docker constant is incorrect: got %s, want docker", Docker)
		}
		if Runc != "runc" {
			t.Errorf("Runc constant is incorrect: got %s, want runc", Runc)
		}
		if CniDockerd != "cri-dockerd" {
			t.Errorf("CniDockerd constant is incorrect: got %s, want cri-dockerd", CniDockerd)
		}
		if Kubeadm != "kubeadm" {
			t.Errorf("Kubeadm constant is incorrect: got %s, want kubeadm", Kubeadm)
		}
		if Kubectl != "kubectl" {
			t.Errorf("Kubectl constant is incorrect: got %s, want kubectl", Kubectl)
		}
		if Keepalived != "keepalived" {
			t.Errorf("Keepalived constant is incorrect: got %s, want keepalived", Keepalived)
		}
		if HAProxy != "haproxy" {
			t.Errorf("HAProxy constant is incorrect: got %s, want haproxy", HAProxy)
		}
		if KubeVIP != "kube-vip" {
			t.Errorf("KubeVIP constant is incorrect: got %s, want kube-vip", KubeVIP)
		}
	})

	t.Run("ServiceNames", func(t *testing.T) {
		if KubeletServiceName != "kubelet.service" {
			t.Errorf("KubeletServiceName constant is incorrect: got %s, want kubelet.service", KubeletServiceName)
		}
		if ContainerdServiceName != "containerd.service" {
			t.Errorf("ContainerdServiceName constant is incorrect: got %s, want containerd.service", ContainerdServiceName)
		}
		if DockerServiceName != "docker.service" {
			t.Errorf("DockerServiceName constant is incorrect: got %s, want docker.service", DockerServiceName)
		}
		if EtcdServiceName != "etcd.service" {
			t.Errorf("EtcdServiceName constant is incorrect: got %s, want etcd.service", EtcdServiceName)
		}
		if CniDockerdServiceName != "cri-dockerd.service" {
			t.Errorf("CniDockerdServiceName constant is incorrect: got %s, want cri-dockerd.service", CniDockerdServiceName)
		}
		if KeepalivedServiceName != "keepalived.service" {
			t.Errorf("KeepalivedServiceName constant is incorrect: got %s, want keepalived.service", KeepalivedServiceName)
		}
		if HAProxyServiceName != "haproxy.service" {
			t.Errorf("HAProxyServiceName constant is incorrect: got %s, want haproxy.service", HAProxyServiceName)
		}
	})

	t.Run("DefaultPorts", func(t *testing.T) {
		if KubeAPIServerDefaultPort != 6443 {
			t.Errorf("KubeAPIServerDefaultPort constant is incorrect: got %d, want 6443", KubeAPIServerDefaultPort)
		}
		if KubeSchedulerDefaultPort != 10259 {
			t.Errorf("KubeSchedulerDefaultPort constant is incorrect: got %d, want 10259", KubeSchedulerDefaultPort)
		}
		if KubeControllerManagerDefaultPort != 10257 {
			t.Errorf("KubeControllerManagerDefaultPort constant is incorrect: got %d, want 10257", KubeControllerManagerDefaultPort)
		}
		if KubeletDefaultPort != 10250 {
			t.Errorf("KubeletDefaultPort constant is incorrect: got %d, want 10250", KubeletDefaultPort)
		}
		if EtcdDefaultClientPort != 2379 {
			t.Errorf("EtcdDefaultClientPort constant is incorrect: got %d, want 2379", EtcdDefaultClientPort)
		}
		if EtcdDefaultPeerPort != 2380 {
			t.Errorf("EtcdDefaultPeerPort constant is incorrect: got %d, want 2380", EtcdDefaultPeerPort)
		}
		if HAProxyDefaultFrontendPort != 6443 {
			t.Errorf("HAProxyDefaultFrontendPort constant is incorrect: got %d, want 6443", HAProxyDefaultFrontendPort)
		}
	})

	t.Run("CommonToolsAndUtils", func(t *testing.T) {
		if Helm != "helm" {
			t.Errorf("Helm constant is incorrect: got %s", Helm)
		}
		if Crictl != "crictl" {
			t.Errorf("Crictl constant is incorrect: got %s", Crictl)
		}
		if Socat != "socat" {
			t.Errorf("Socat constant is incorrect: got %s", Socat)
		}
		if Conntrack != "conntrack" {
			t.Errorf("Conntrack constant is incorrect: got %s", Conntrack)
		}
		if IPSet != "ipset" {
			t.Errorf("IPSet constant is incorrect: got %s", IPSet)
		}
		if Ipvsadm != "ipvsadm" {
			t.Errorf("Ipvsadm constant is incorrect: got %s", Ipvsadm)
		}
		if NfsUtils != "nfs-utils" {
			t.Errorf("NfsUtils constant is incorrect: got %s", NfsUtils)
		}
		if CephCommon != "ceph-common" {
			t.Errorf("CephCommon constant is incorrect: got %s", CephCommon)
		}
	})

	t.Run("ImageRepositoriesAndVersions", func(t *testing.T) {
		if CoreDNSImageRepository != "registry.k8s.io/coredns" {
			t.Errorf("CoreDNSImageRepository constant is incorrect: got %s", CoreDNSImageRepository)
		}
		if PauseImageRepository != "registry.k8s.io/pause" {
			t.Errorf("PauseImageRepository constant is incorrect: got %s", PauseImageRepository)
		}
		if DefaultPauseImageVersion != "3.9" { // Example, adjust if changed
			t.Errorf("DefaultPauseImageVersion constant is incorrect: got %s", DefaultPauseImageVersion)
		}
	})
}
