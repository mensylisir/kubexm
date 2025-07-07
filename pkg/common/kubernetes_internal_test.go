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

	t.Run("KubeadmConstants", func(t *testing.T) {
		assert.Equal(t, "init-config.yaml", KubeadmInitConfigFileName)
		assert.Equal(t, "join-config.yaml", KubeadmJoinConfigFileName)
		assert.Equal(t, "reset", KubeadmResetCommand)
		assert.Equal(t, "24h0m0s", KubeadmTokenDefaultTTL)
		assert.Equal(t, "sha256:", KubeadmDiscoveryTokenCACertHashPrefix)
	})

	t.Run("CertificateConstants", func(t *testing.T) {
		assert.Equal(t, "system:masters", DefaultCertificateOrganization)
		assert.Equal(t, "system:nodes", KubeletCertificateOrganization)
		assert.Equal(t, "system:node:", KubeletCertificateCNPrefix)
	})

	t.Run("AnnotationKeyConstants", func(t *testing.T) {
		assert.Equal(t, "node.kubernetes.io/exclude-from-external-load-balancers", AnnotationNodeKubeadmAlphaExcludeFromExternalLB)
	})
}
