package common

// This file defines constants related to host roles within Kubexm
// and standard Kubernetes node labels and taints.

// --- Host Roles defined by Kubexm ---
// These roles are used in the ClusterSpec to assign responsibilities to hosts.
const (
	// RoleMaster identifies a host as a Kubernetes control-plane node.
	RoleMaster = "master"
	// RoleWorker identifies a host as a Kubernetes worker node.
	RoleWorker = "worker"
	// RoleEtcd identifies a host as an Etcd member.
	RoleEtcd = "etcd"
	// RoleLoadBalancer identifies a host designated to run load balancing software (e.g., Keepalived, HAProxy).
	RoleLoadBalancer = "loadbalancer"
	// RoleStorage identifies a host designated for storage solutions (e.g., Ceph OSDs, GlusterFS bricks).
	RoleStorage = "storage" // Although not explicitly in all configs, good to have for extensibility
	// RoleRegistry identifies a host designated to run a local container image registry.
	RoleRegistry = "registry"
	// RoleControlNode is defined in constants.go as "control-node", representing the Kubexm execution machine.
)

// ControlNodeHostName is the special hostname used for operations running locally on the machine executing Kubexm.
// This constant is defined in constants.go (`ControlNodeHostName = "kubexm-control-node"`).
// It's referenced here for context regarding roles.

// --- Standard Kubernetes Node Labels & Taints ---
// These are well-known labels and taint keys used in Kubernetes.
const (
	// LabelNodeRoleMaster is a standard Kubernetes label for master nodes.
	// Often used interchangeably with control-plane, but control-plane is more common now.
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
	// TaintKeyNodeRoleMaster is a standard Kubernetes taint key for master nodes,
	// often used with effect NoSchedule to prevent regular workloads from running on them.
	TaintKeyNodeRoleMaster = "node-role.kubernetes.io/master" // Effect would be TaintEffectNoSchedule

	// LabelNodeRoleControlPlane is the more current standard Kubernetes label for control-plane nodes.
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	// TaintKeyNodeRoleControlPlane is the more current standard Kubernetes taint key for control-plane nodes.
	TaintKeyNodeRoleControlPlane = "node-role.kubernetes.io/control-plane" // Effect would be TaintEffectNoSchedule

	// LabelManagedBy is a common label to identify which tool or controller manages a resource.
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelValueKubexm is the value for the LabelManagedBy label when Kubexm manages the node or resource.
	LabelValueKubexm = "kubexm"

	// Well-Known Topology Labels used for scheduling and awareness.
	// LabelHostname is the label key for the node's hostname.
	LabelHostname = "kubernetes.io/hostname"
	// LabelTopologyZone is the label key for the availability zone a node is in.
	LabelTopologyZone = "topology.kubernetes.io/zone"
	// LabelTopologyRegion is the label key for the geographical region a node is in.
	LabelTopologyRegion = "topology.kubernetes.io/region"
	// LabelInstanceType is a common label for cloud provider instance types.
	LabelInstanceType = "node.kubernetes.io/instance-type"
	// LabelOS is the label key for the node's operating system.
	LabelOS = "kubernetes.io/os"
	// LabelArch is the label key for the node's architecture.
	LabelArch = "kubernetes.io/arch"


	// Standard Taint Effects. These are also defined as ValidTaintEffects in constants.go.
	// TaintEffectNoSchedule means new pods will not be scheduled on the node unless they tolerate the taint.
	TaintEffectNoSchedule = "NoSchedule"
	// TaintEffectPreferNoSchedule is a "preference" version of NoSchedule; the scheduler will try to avoid it.
	TaintEffectPreferNoSchedule = "PreferNoSchedule"
	// TaintEffectNoExecute means existing pods on the node that do not tolerate the taint will be evicted.
	TaintEffectNoExecute = "NoExecute"
)
