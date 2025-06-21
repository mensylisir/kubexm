package common

const (
	// KUBEXM is the default root directory name for kubexm operations.
	KUBEXM = ".kubexm"
	// DefaultLogsDir is the default directory name for logs within the KUBEXM work directory.
	DefaultLogsDir = "logs"
	// DefaultCertsDir is the default directory name for certificates.
	DefaultCertsDir = "certs"
	// DefaultContainerRuntimeDir is the default directory for container runtime artifacts.
	DefaultContainerRuntimeDir = "container_runtime"
	// DefaultKubernetesDir is the default directory for kubernetes artifacts.
	DefaultKubernetesDir = "kubernetes"
	// DefaultEtcdDir is the default directory for etcd artifacts.
	DefaultEtcdDir = "etcd"

	// ControlNodeHostName is the special hostname used for operations running locally on the control machine.
	ControlNodeHostName = "kubexm-control-node"
	// ControlNodeRole is the role assigned to the special local control node.
	ControlNodeRole = "control-node"

	// DefaultWorkDirName is the default name for the main working directory if not specified.
	// This constant seems unused if GlobalWorkDir defaults to KUBEXM directly.
	// DefaultWorkDirName = "kubexms_work"

)
