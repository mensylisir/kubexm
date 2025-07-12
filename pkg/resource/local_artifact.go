package resource

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

// LocalCertificateHandle represents a certificate resource that is expected to be
// generated or placed locally on the control node.
type LocalCertificateHandle struct {
	// User-defined properties
	ResourceName string // e.g., "etcd", "kube-apiserver". Used to determine subdirectory under "certs".
	CertFileName string // e.g., "ca.pem", "server.key", "apiserver-etcd-client.pem"
	ClusterName  string // Explicitly pass cluster name for path construction

	// Internal
	controlNodeHost []connector.Host // Cached control node host object
}

// NewLocalCertificateHandle creates a new local certificate resource handle.
func NewLocalCertificateHandle(resourceName, certFileName, clusterName string) Handle {
	return &LocalCertificateHandle{
		ResourceName: resourceName, // This will define the subdirectory, e.g., "etcd"
		CertFileName: certFileName,
		ClusterName:  clusterName,
	}
}

func (h *LocalCertificateHandle) ID() string {
	return fmt.Sprintf("%s-%s-cert-%s", h.ResourceName, filepath.Base(h.CertFileName), h.ClusterName)
}

// getControlNode retrieves and caches the control node host object.
func (h *LocalCertificateHandle) getControlNode(ctx task.TaskContext) ([]connector.Host, error) {
	if h.controlNodeHost != nil {
		return h.controlNodeHost, nil
	}
	nodes, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for resource %s: %w", h.ID(), err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no control node found for resource %s", h.ID())
	}
	h.controlNodeHost = []connector.Host{nodes[0]} // Use the first control node
	return h.controlNodeHost, nil
}

// Path returns the expected final local path of the certificate on the control node.
// Structure: GlobalWorkDir (e.g., $(pwd)/.kubexm)/${cluster_name}/certs/${h.ResourceName}/${h.CertFileName}
func (h *LocalCertificateHandle) Path(ctx task.TaskContext) (string, error) {
	if h.ClusterName == "" {
		err := fmt.Errorf("ClusterName is empty in LocalCertificateHandle for resource %s/%s", h.ResourceName, h.CertFileName)
		ctx.GetLogger().Error(err.Error())
		return "", err
	}
	// GlobalWorkDir is like $(pwd)/.kubexm
	// Path: $(pwd)/.kubexm/${cluster_name}/certs/${h.ResourceName}/${h.CertFileName}
	// Example: .kubexm/mycluster/certs/etcd/ca.pem
	// task.TaskContext provides GetGlobalWorkDir() and GetClusterConfig().Name for these.
	// However, the design doc 21-其他说明.md suggests paths like:
	// workdir/.kubexm/${cluster_name}/certs/etcd/
	// Where `workdir` is $(pwd).
	// The `runtime.Context.GetGlobalWorkDir()` is already `$(pwd)/.kubexm/${cluster_name}`.
	// So, the path should be `GetGlobalWorkDir()/certs/${h.ResourceName}/${h.CertFileName}`
	// Let's use `ctx.GetCertsDir()` which should resolve to `GlobalWorkDir/certs` and then add ResourceName.
	// This requires `GetCertsDir` to be on `TaskContext` or accessible.
	// Assuming TaskContext provides access to these path helpers from the main runtime.Context.
	// `ctx.GetGlobalWorkDir()` is `$(pwd)/.kubexm/${cluster_name}`.
	// `ctx.GetCertsDir()` is `$(pwd)/.kubexm/${cluster_name}/certs`.
	// `ctx.GetEtcdCertsDir()` is `$(pwd)/.kubexm/${cluster_name}/certs/etcd`.

	// Corrected path construction based on runtime context accessors:
	// The ResourceName field in LocalCertificateHandle (e.g., "etcd") is used as the subdirectory under "certs".
	// So, if ResourceName is "etcd", path is GetGlobalWorkDir()/certs/etcd/CertFileName
	// If ResourceName is "kubernetes", path is GetGlobalWorkDir()/certs/kubernetes/CertFileName
	// This matches common.DefaultCertsDir and common.DefaultEtcdDir usage.
	// The runtime.Context has GetCertsDir() and GetEtcdCertsDir().
	// For a generic cert, it should be runtime.Context.GetCertsDir() + h.ResourceName + h.CertFileName.
	// Let's assume task.TaskContext has a GetCertsDir() method.
	certsBaseDir := filepath.Join(ctx.GetGlobalWorkDir(), common.DefaultCertsDir) // GlobalWorkDir/certs
	resourceCertsDir := filepath.Join(certsBaseDir, h.ResourceName)               // GlobalWorkDir/certs/etcd
	return filepath.Join(resourceCertsDir, h.CertFileName), nil
}

// EnsurePlan for a LocalCertificateHandle primarily checks for the certificate's existence.
// The actual generation of the certificate is assumed to be handled by other dedicated steps
// (e.g., GenerateCACertStep, GenerateSignedCertStep) planned by a Task.
// This handle's role is to provide the standardized path and verify if it's already met.
func (h *LocalCertificateHandle) EnsurePlan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_handle", h.ID())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)

	controlNodeHosts, err := h.getControlNode(ctx)
	if err != nil {
		return nil, err
	}
	controlNode := controlNodeHosts[0]

	finalCertPath, err := h.Path(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not determine path for certificate %s: %w", h.ID(), err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(controlNode) // Should be LocalConnector
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node: %w", err)
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, finalCertPath)
	if err != nil {
		logger.Warn("Failed to check certificate existence, assuming it needs to be generated.", "path", finalCertPath, "error", err)
		return task.NewEmptyFragment(), nil
	}

	if exists {
		logger.Info("Certificate already exists locally on control node. No action needed by this handle.", "path", finalCertPath)
	} else {
		logger.Info("Certificate does not exist locally. Its generation should be handled by dedicated steps in the task.", "path", finalCertPath)
	}

	return task.NewEmptyFragment(), nil
}

// Ensure LocalCertificateHandle implements the Handle interface.
var _ Handle = (*LocalCertificateHandle)(nil)
