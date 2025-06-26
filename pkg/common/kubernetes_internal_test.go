package common

import "testing"

func TestKubernetesInternalConstants(t *testing.T) {
	t.Run("CoreDNSConstants", func(t *testing.T) {
		if CoreDNSConfigMapName != "coredns" {
			t.Errorf("CoreDNSConfigMapName constant is incorrect: got %s, want coredns", CoreDNSConfigMapName)
		}
		if CoreDNSDeploymentName != "coredns" {
			t.Errorf("CoreDNSDeploymentName constant is incorrect: got %s, want coredns", CoreDNSDeploymentName)
		}
		if CoreDNSServiceName != "kube-dns" {
			t.Errorf("CoreDNSServiceName constant is incorrect: got %s, want kube-dns", CoreDNSServiceName)
		}
	})

	t.Run("KubeProxyConstants", func(t *testing.T) {
		if KubeProxyConfigMapName != "kube-proxy" {
			t.Errorf("KubeProxyConfigMapName constant is incorrect: got %s, want kube-proxy", KubeProxyConfigMapName)
		}
		if KubeProxyDaemonSetName != "kube-proxy" {
			t.Errorf("KubeProxyDaemonSetName constant is incorrect: got %s, want kube-proxy", KubeProxyDaemonSetName)
		}
	})

	t.Run("ClusterInfoConstants", func(t *testing.T) {
		if ClusterInfoConfigMapName != "cluster-info" {
			t.Errorf("ClusterInfoConfigMapName constant is incorrect: got %s, want cluster-info", ClusterInfoConfigMapName)
		}
		if KubeadmConfigConfigMapName != "kubeadm-config" {
			t.Errorf("KubeadmConfigConfigMapName constant is incorrect: got %s, want kubeadm-config", KubeadmConfigConfigMapName)
		}
	})

	t.Run("SecretsConstants", func(t *testing.T) {
		if BootstrapTokenSecretPrefix != "bootstrap-token-" {
			t.Errorf("BootstrapTokenSecretPrefix constant is incorrect: got %s, want bootstrap-token-", BootstrapTokenSecretPrefix)
		}
	})

	t.Run("RBACConstants", func(t *testing.T) {
		if NodeBootstrapperClusterRoleName != "system:node-bootstrapper" {
			t.Errorf("NodeBootstrapperClusterRoleName constant is incorrect: got %s, want system:node-bootstrapper", NodeBootstrapperClusterRoleName)
		}
		if KubeadmNodeAdminClusterRoleBindingName != "kubeadm:node-admins" {
			t.Errorf("KubeadmNodeAdminClusterRoleBindingName constant is incorrect: got %s, want kubeadm:node-admins", KubeadmNodeAdminClusterRoleBindingName)
		}
	})

	t.Run("KubeletSettingsConstants", func(t *testing.T) {
		if KubeletCSICertsVolumeName != "kubelet-csi-certs" {
			t.Errorf("KubeletCSICertsVolumeName constant is incorrect: got %s, want kubelet-csi-certs", KubeletCSICertsVolumeName)
		}
		if KubeletCSICertsMountPath != "/var/lib/kubelet/plugins_registry" {
			t.Errorf("KubeletCSICertsMountPath constant is incorrect: got %s, want /var/lib/kubelet/plugins_registry", KubeletCSICertsMountPath)
		}
	})
}
