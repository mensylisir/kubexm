package common

// --- Host Roles ---
const (
	RoleMaster         = "master"
	RoleWorker         = "worker"
	RoleEtcd           = "etcd"
	RoleLoadBalancer   = "loadbalancer"
	RoleStorage        = "storage"
	RoleRegistry       = "registry"
	// RoleControlNode is defined in general.go
)

// --- Kubernetes Node Labels & Taints ---
const (
	// LabelNodeRoleMaster is a standard Kubernetes label for master nodes.
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
	// TaintKeyNodeRoleMaster is a standard Kubernetes taint key for master nodes.
	TaintKeyNodeRoleMaster = "node-role.kubernetes.io/master"

	// LabelNodeRoleControlPlane is a standard Kubernetes label for control-plane nodes.
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	// TaintKeyNodeRoleControlPlane is a standard Kubernetes taint key for control-plane nodes.
	TaintKeyNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	// LabelManagedBy is a custom label to identify nodes managed by kubexm.
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelValueKubexm is the value for the LabelManagedBy label.
	LabelValueKubexm = "kubexm"
)
