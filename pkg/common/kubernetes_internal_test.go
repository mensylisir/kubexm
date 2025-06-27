package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubernetesInternalConstants(t *testing.T) {
	t.Run("CoreDNSConstants", func(t *testing.T) {
		assert.Equal(t, "coredns", CoreDNSConfigMapName, "CoreDNSConfigMapName constant is incorrect")
		assert.Equal(t, "coredns", CoreDNSDeploymentName, "CoreDNSDeploymentName constant is incorrect")
		assert.Equal(t, "kube-dns", CoreDNSServiceName, "CoreDNSServiceName constant is incorrect")
	})

	t.Run("KubeProxyConstants", func(t *testing.T) {
		assert.Equal(t, "kube-proxy", KubeProxyConfigMapName, "KubeProxyConfigMapName constant is incorrect")
		assert.Equal(t, "kube-proxy", KubeProxyDaemonSetName, "KubeProxyDaemonSetName constant is incorrect")
	})

	t.Run("ClusterInfoConstants", func(t *testing.T) {
		assert.Equal(t, "cluster-info", ClusterInfoConfigMapName, "ClusterInfoConfigMapName constant is incorrect")
		assert.Equal(t, "kubeadm-config", KubeadmConfigConfigMapName, "KubeadmConfigConfigMapName constant is incorrect")
	})

	t.Run("SecretsConstants", func(t *testing.T) {
		assert.Equal(t, "bootstrap-token-", BootstrapTokenSecretPrefix, "BootstrapTokenSecretPrefix constant is incorrect")
	})

	t.Run("RBACConstants", func(t *testing.T) {
		assert.Equal(t, "system:node-bootstrapper", NodeBootstrapperClusterRoleName, "NodeBootstrapperClusterRoleName constant is incorrect")
		assert.Equal(t, "kubeadm:node-admins", KubeadmNodeAdminClusterRoleBindingName, "KubeadmNodeAdminClusterRoleBindingName constant is incorrect")
	})

	t.Run("KubeletSettingsConstants", func(t *testing.T) {
		assert.Equal(t, "kubelet-csi-certs", KubeletCSICertsVolumeName, "KubeletCSICertsVolumeName constant is incorrect")
		assert.Equal(t, "/var/lib/kubelet/plugins_registry", KubeletCSICertsMountPath, "KubeletCSICertsMountPath constant is incorrect")
	})
}
