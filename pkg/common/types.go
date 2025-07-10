package common

// KubernetesDeploymentType defines the type of Kubernetes deployment.
type KubernetesDeploymentType string

const (
	// KubernetesDeploymentTypeKubeadm indicates deployment via Kubeadm.
	KubernetesDeploymentTypeKubeadm KubernetesDeploymentType = "kubeadm"
	// KubernetesDeploymentTypeKubexm indicates binary deployment.
	KubernetesDeploymentTypeKubexm KubernetesDeploymentType = "kubexm"
)

// EtcdDeploymentType defines the type of Etcd deployment.
type EtcdDeploymentType string

const (
	// EtcdDeploymentTypeKubeadm indicates Etcd deployed by Kubeadm (stacked).
	EtcdDeploymentTypeKubeadm EtcdDeploymentType = "kubeadm"
	// EtcdDeploymentTypeKubexm indicates Etcd deployed by Kubexm binaries.
	EtcdDeploymentTypeKubexm EtcdDeploymentType = "kubexm"
	// EtcdDeploymentTypeExternal indicates using an external Etcd cluster.
	EtcdDeploymentTypeExternal EtcdDeploymentType = "external"
)

// InternalLoadBalancerType defines the type of internal load balancer.
type InternalLoadBalancerType string

const (
	// InternalLBTypeKubeVIP indicates Kube-VIP as the internal load balancer.
	InternalLBTypeKubeVIP InternalLoadBalancerType = "kube-vip"
	// InternalLBTypeHAProxy indicates HAProxy (on workers/nodes) as the internal load balancer.
	InternalLBTypeHAProxy InternalLoadBalancerType = "haproxy"
	// InternalLBTypeNginx indicates Nginx (on workers/nodes) as the internal load balancer.
	InternalLBTypeNginx InternalLoadBalancerType = "nginx"
)

// ExternalLoadBalancerType defines the type of external load balancer.
type ExternalLoadBalancerType string

const (
	// ExternalLBTypeKubexmKH indicates Kubexm-managed Keepalived + HAProxy.
	ExternalLBTypeKubexmKH ExternalLoadBalancerType = "kubexm-kh"
	// ExternalLBTypeKubexmKN indicates Kubexm-managed Keepalived + Nginx.
	ExternalLBTypeKubexmKN ExternalLoadBalancerType = "kubexm-kn"
	// ExternalLBTypeExternal indicates a user-provided external load balancer.
	ExternalLBTypeExternal ExternalLoadBalancerType = "external"
	// ExternalLBTypeNone indicates no external load balancer (can be empty string in config).
	ExternalLBTypeNone ExternalLoadBalancerType = ""
)

// ContainerRuntimeType defines the type of container runtime.
type ContainerRuntimeType string

const (
	RuntimeTypeDocker     ContainerRuntimeType = "docker"
	RuntimeTypeContainerd ContainerRuntimeType = "containerd"
	RuntimeTypeCRIO       ContainerRuntimeType = "cri-o"
	RuntimeTypeIsula      ContainerRuntimeType = "isula"
)

// CNIType defines the type of CNI plugin.
type CNIType string

const (
	CNITypeCalico   CNIType = "calico"
	CNITypeFlannel  CNIType = "flannel"
	CNITypeCilium   CNIType = "cilium"
	CNITypeKubeOvn  CNIType = "kube-ovn"
	CNITypeMultus   CNIType = "multus"
	CNITypeHybridnet CNIType = "hybridnet"
)

// TaskStatus defines the status of a task or operation.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "Pending"
	TaskStatusProcessing TaskStatus = "Processing"
	TaskStatusSuccess    TaskStatus = "Success"
	TaskStatusFailed     TaskStatus = "Failed"
	TaskStatusSkipped    TaskStatus = "Skipped"
)

// BinaryType defines the type of a binary artifact.
// This was previously in pkg/util/binary_info.go's context but makes sense in common if used widely.
// Or it can be a sub-type within a more specific package if only used there.
// For now, placing a simplified version here as per the idea of centralizing types.
type BinaryType string

const (
	BinaryTypeEtcd         BinaryType = "etcd"
	BinaryTypeKube         BinaryType = "kube" // For kubeadm, kubelet, kubectl etc.
	BinaryTypeCNI          BinaryType = "cni"
	BinaryTypeHelm         BinaryType = "helm"
	BinaryTypeDocker       BinaryType = "docker"
	BinaryTypeContainerd   BinaryType = "containerd"
	BinaryTypeRunc         BinaryType = "runc"
	BinaryTypeCrictl       BinaryType = "crictl"
	BinaryTypeCriDockerd   BinaryType = "cri-dockerd"
	BinaryTypeCalicoctl    BinaryType = "calicoctl"
	BinaryTypeRegistry     BinaryType = "registry" // For Harbor, Docker Registry
	BinaryTypeCompose      BinaryType = "compose"   // Docker Compose
	BinaryTypeBuild        BinaryType = "build"     // For buildx
	BinaryTypeGeneric      BinaryType = "generic"   // For other generic binaries
)

// HostConnectionType defines the method of connecting to a host.
type HostConnectionType string

const (
	// HostConnectionTypeSSH indicates connection via SSH.
	HostConnectionTypeSSH HostConnectionType = "ssh"
	// HostConnectionTypeLocal indicates operations on the local machine.
	HostConnectionTypeLocal HostConnectionType = "local"
)
