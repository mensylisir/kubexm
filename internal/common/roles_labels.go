package common

const (
	RoleMaster       = "master"
	RoleNode         = "node"
	RoleControlPlane = "control-plane"
	RoleWorker       = "worker"
	RoleEtcd         = "etcd"
	RoleLoadBalancer = "loadbalancer"
	RoleStorage      = "storage"
	RoleRegistry     = "registry"
	RoleKubernetes   = "kubernetes"
)

const (
	LabelNodeRoleMaster          = "node-role.kubernetes.io/master"
	TaintKeyNodeRoleMaster       = "node-role.kubernetes.io/master"
	LabelNodeRoleControlPlane    = "node-role.kubernetes.io/control-plane"
	TaintKeyNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	LabelNodeRoleWorker          = "node-role.kubernetes.io/worker"
	TaintKeyNodeRoleWorker       = "node-role.kubernetes.io/worker"
	LabelManagedBy               = "app.kubernetes.io/managed-by"
	LabelValueKubexm             = "kubexm"
	LabelHostname                = "kubernetes.io/hostname"
	LabelTopologyZone            = "topology.kubernetes.io/zone"
	LabelTopologyRegion          = "topology.kubernetes.io/region"
	LabelInstanceType            = "node.kubernetes.io/instance-type"
	LabelOS                      = "kubernetes.io/os"
	LabelArch                    = "kubernetes.io/arch"
	TaintEffectNoSchedule        = "NoSchedule"
	TaintEffectPreferNoSchedule  = "PreferNoSchedule"
	TaintEffectNoExecute         = "NoExecute"
)

var ValidTaintEffects = []string{TaintEffectNoSchedule, TaintEffectPreferNoSchedule, TaintEffectNoExecute}

const (
	AllHostsRole        = "all"
	ControlNodeRole     = "controlNode"
	ControlNodeHostName = "kubexm-control-node"
)
