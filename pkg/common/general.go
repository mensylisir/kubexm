package common

const (
	// KUBEXM is the default root directory name for kubexm operations.
	KUBEXM = ".kubexm"
	// DefaultLogsDir is the default directory name for logs within the KUBEXM work directory.
	DefaultLogsDir = "logs"

	// ControlNodeHostName is the special hostname used for operations running locally on the control machine.
	ControlNodeHostName = "kubexm-control-node"
	// ControlNodeRole is the role assigned to the special local control node.
	ControlNodeRole = "control-node"
)

// --- Status Constants ---
const (
	StatusPending    = "Pending"
	StatusProcessing = "Processing"
	StatusSuccess    = "Success"
	StatusFailed     = "Failed"
)

// --- Node Conditions (from k8s.io/api/core/v1) ---
const (
	NodeConditionReady = "Ready"
)

// --- CNI Plugin Names ---
const (
	CNICalico   = "calico"
	CNIFlannel  = "flannel"
	CNICilium   = "cilium"
	CNIMultus   = "multus"
)

// --- Cache Key Constants ---
const (
	// CacheKeyHostFactsPrefix is the prefix for caching host facts.
	CacheKeyHostFactsPrefix = "facts.host."
	// CacheKeyClusterCACert is the key for the cluster CA certificate.
	CacheKeyClusterCACert = "pki.ca.cert"
	// CacheKeyClusterCAKey is the key for the cluster CA key.
	CacheKeyClusterCAKey = "pki.ca.key"
)
