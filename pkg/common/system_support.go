package common

// Operating system support constants
var (
	// SupportedOperatingSystems lists the operating systems supported by Kubexm.
	SupportedOperatingSystems = []string{"linux", "darwin", "windows"}
	
	// SupportedLinuxDistributions lists the Linux distributions supported by Kubexm.
	SupportedLinuxDistributions = []string{
		"ubuntu", "debian", "centos", "rhel", "rocky", "almalinux", "fedora",
		"opensuse", "sles", "amzn", "oracle", "photon", "flatcar", "coreos",
	}
	
	// SupportedContainerRuntimes lists the container runtimes supported by Kubexm.
	SupportedContainerRuntimes = []string{
		string(RuntimeTypeDocker),
		string(RuntimeTypeContainerd),
		string(RuntimeTypeCRIO),
		string(RuntimeTypeIsula),
	}
	
	// SupportedCNITypes lists the CNI types supported by Kubexm.
	SupportedCNITypes = []string{
		string(CNITypeCalico),
		string(CNITypeFlannel),
		string(CNITypeCilium),
		string(CNITypeKubeOvn),
		string(CNITypeMultus),
		string(CNITypeHybridnet),
	}
	
	// SupportedInternalLoadBalancerTypes lists the internal load balancer types supported by Kubexm.
	SupportedInternalLoadBalancerTypes = []string{
		string(InternalLBTypeKubeVIP),
		string(InternalLBTypeHAProxy),
		string(InternalLBTypeNginx),
	}
	
	// SupportedExternalLoadBalancerTypes lists the external load balancer types supported by Kubexm.
	SupportedExternalLoadBalancerTypes = []string{
		string(ExternalLBTypeKubexmKH),
		string(ExternalLBTypeKubexmKN),
		string(ExternalLBTypeExternal),
		string(ExternalLBTypeNone),
	}
	
	// SupportedKubernetesDeploymentTypes lists the Kubernetes deployment types supported by Kubexm.
	SupportedKubernetesDeploymentTypes = []string{
		string(KubernetesDeploymentTypeKubeadm),
		string(KubernetesDeploymentTypeKubexm),
	}
	
	// SupportedEtcdDeploymentTypes lists the etcd deployment types supported by Kubexm.
	SupportedEtcdDeploymentTypes = []string{
		string(EtcdDeploymentTypeKubeadm),
		string(EtcdDeploymentTypeKubexm),
		string(EtcdDeploymentTypeExternal),
	}
)