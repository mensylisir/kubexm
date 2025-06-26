package common

import "testing"

func TestRolesLabelsConstants(t *testing.T) {
	t.Run("HostRoles", func(t *testing.T) {
		if RoleMaster != "master" {
			t.Errorf("RoleMaster constant is incorrect: got %s", RoleMaster)
		}
		if RoleWorker != "worker" {
			t.Errorf("RoleWorker constant is incorrect: got %s", RoleWorker)
		}
		if RoleEtcd != "etcd" {
			t.Errorf("RoleEtcd constant is incorrect: got %s", RoleEtcd)
		}
		if RoleLoadBalancer != "loadbalancer" {
			t.Errorf("RoleLoadBalancer constant is incorrect: got %s", RoleLoadBalancer)
		}
		if RoleStorage != "storage" {
			t.Errorf("RoleStorage constant is incorrect: got %s", RoleStorage)
		}
		if RoleRegistry != "registry" {
			t.Errorf("RoleRegistry constant is incorrect: got %s", RoleRegistry)
		}
	})

	t.Run("KubernetesNodeLabelsAndTaints", func(t *testing.T) {
		if LabelNodeRoleMaster != "node-role.kubernetes.io/master" {
			t.Errorf("LabelNodeRoleMaster constant is incorrect: got %s", LabelNodeRoleMaster)
		}
		if TaintKeyNodeRoleMaster != "node-role.kubernetes.io/master" {
			t.Errorf("TaintKeyNodeRoleMaster constant is incorrect: got %s", TaintKeyNodeRoleMaster)
		}
		if LabelNodeRoleControlPlane != "node-role.kubernetes.io/control-plane" {
			t.Errorf("LabelNodeRoleControlPlane constant is incorrect: got %s", LabelNodeRoleControlPlane)
		}
		if TaintKeyNodeRoleControlPlane != "node-role.kubernetes.io/control-plane" {
			t.Errorf("TaintKeyNodeRoleControlPlane constant is incorrect: got %s", TaintKeyNodeRoleControlPlane)
		}
		if LabelManagedBy != "app.kubernetes.io/managed-by" {
			t.Errorf("LabelManagedBy constant is incorrect: got %s", LabelManagedBy)
		}
		if LabelValueKubexm != "kubexm" {
			t.Errorf("LabelValueKubexm constant is incorrect: got %s", LabelValueKubexm)
		}
	})
}
