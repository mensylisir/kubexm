package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRolesLabelsConstants(t *testing.T) {
	t.Run("HostRoles", func(t *testing.T) {
		assert.Equal(t, "master", RoleMaster, "RoleMaster constant is incorrect")
		assert.Equal(t, "worker", RoleWorker, "RoleWorker constant is incorrect")
		assert.Equal(t, "etcd", RoleEtcd, "RoleEtcd constant is incorrect")
		assert.Equal(t, "loadbalancer", RoleLoadBalancer, "RoleLoadBalancer constant is incorrect")
		assert.Equal(t, "storage", RoleStorage, "RoleStorage constant is incorrect")
		assert.Equal(t, "registry", RoleRegistry, "RoleRegistry constant is incorrect")
	})

	t.Run("KubernetesNodeLabelsAndTaints", func(t *testing.T) {
		assert.Equal(t, "node-role.kubernetes.io/master", LabelNodeRoleMaster, "LabelNodeRoleMaster constant is incorrect")
		assert.Equal(t, "node-role.kubernetes.io/master", TaintKeyNodeRoleMaster, "TaintKeyNodeRoleMaster constant is incorrect")
		assert.Equal(t, "node-role.kubernetes.io/control-plane", LabelNodeRoleControlPlane, "LabelNodeRoleControlPlane constant is incorrect")
		assert.Equal(t, "node-role.kubernetes.io/control-plane", TaintKeyNodeRoleControlPlane, "TaintKeyNodeRoleControlPlane constant is incorrect")
		assert.Equal(t, "app.kubernetes.io/managed-by", LabelManagedBy, "LabelManagedBy constant is incorrect")
		assert.Equal(t, "kubexm", LabelValueKubexm, "LabelValueKubexm constant is incorrect")
	})
}
