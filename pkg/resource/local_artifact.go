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

type LocalCertificateHandle struct {
	ResourceName string
	CertFileName string
	ClusterName  string

	controlNodeHost []connector.Host
}

func NewLocalCertificateHandle(resourceName, certFileName, clusterName string) Handle {
	return &LocalCertificateHandle{
		ResourceName: resourceName,
		CertFileName: certFileName,
		ClusterName:  clusterName,
	}
}

func (h *LocalCertificateHandle) ID() string {
	return fmt.Sprintf("%s-%s-cert-%s", h.ResourceName, filepath.Base(h.CertFileName), h.ClusterName)
}

func (h *LocalCertificateHandle) getControlNode(ctx runtime.TaskContext) ([]connector.Host, error) {
	if h.controlNodeHost != nil {
		return h.controlNodeHost, nil
	}
	nodes := ctx.GetHostsByRole(common.ControlNodeRole)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no control node found for resource %s", h.ID())
	}
	h.controlNodeHost = []connector.Host{nodes[0]}
	return h.controlNodeHost, nil
}

func (h *LocalCertificateHandle) Path(ctx runtime.TaskContext) (string, error) {
	if h.ClusterName == "" {
		err := fmt.Errorf("ClusterName is empty in LocalCertificateHandle for resource %s/%s", h.ResourceName, h.CertFileName)
		ctx.GetLogger().Error(err.Error())
		return "", err
	}
	certsBaseDir := filepath.Join(ctx.GetGlobalWorkDir(), common.DefaultCertsDir)
	resourceCertsDir := filepath.Join(certsBaseDir, h.ResourceName)
	return filepath.Join(resourceCertsDir, h.CertFileName), nil
}

func (h *LocalCertificateHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
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
	conn, err := ctx.GetConnectorForHost(controlNode)
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
