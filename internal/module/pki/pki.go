package pki

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	pkiEtcd "github.com/mensylisir/kubexm/internal/task/pki/etcd"
	pkiKubexm "github.com/mensylisir/kubexm/internal/task/pki/kubexm"
)

// PKIModule handles certificate renewal for Kubernetes and etcd.
// It chains generation → rollout → cleanup tasks for each cert type.
type PKIModule struct {
	module.BaseModule
	certType string
}

// NewPKIModule creates a new PKIModule for the specified certificate type.
func NewPKIModule(certType string) module.Module {
	var moduleTasks []task.Task

	switch certType {
	case "kubernetes-ca", "kubernetes-certs":
		// Kubernetes certificate renewal chain (without etcd):
		// 1. Generate new CA + leaf certs locally
		// 2. Activate certs + generate kubeconfigs
		// 3. Distribute leaf certs → restart control plane components
		// 4. Distribute CA bundle → restart control plane components
		// 5. Distribute final CA + kubeconfigs → restart kubelet, kube-proxy, control plane
		// 6. Distribute CA + kubeconfigs to worker nodes → restart kubelet, kube-proxy
		// 7. Cleanup temporary files
		moduleTasks = []task.Task{
			pkiKubexm.NewGenerateClusterCertsTask(),
			pkiKubexm.NewGenerateKubeconfigsTask(),
			pkiKubexm.NewRolloutLeafCertsToMastersTask(),
			pkiKubexm.NewRolloutK8sCABundleTask(),
			pkiKubexm.NewFinalizeMastersTask(),
			pkiKubexm.NewUpdateWorkerNodesTask(),
			pkiKubexm.NewCleanupTask(),
		}
	case "etcd-ca", "etcd-certs":
		// Etcd certificate renewal chain:
		// 1. Generate new CA + leaf certs locally
		// 2. Deploy new leaf certs → restart etcd
		// 3. Deploy CA bundle → restart etcd
		// 4. Deploy final CA → restart etcd
		// 5. Cleanup workspace
		moduleTasks = []task.Task{
			pkiEtcd.NewGenerateNewCertificatesTask(),
			pkiEtcd.NewDeployNewLeafCertsRollingTask(),
			pkiEtcd.NewDeployTrustBundleRollingTask(),
			pkiEtcd.NewDeployFinalCARollingTask(),
			pkiEtcd.NewFinalizeWorkspaceTask(),
		}
	case "all":
		// Complete certificate renewal: Kubernetes + etcd
		// Run Kubernetes chain first, then etcd chain
		moduleTasks = []task.Task{
			// Kubernetes certs
			pkiKubexm.NewGenerateClusterCertsTask(),
			pkiKubexm.NewGenerateKubeconfigsTask(),
			pkiKubexm.NewRolloutLeafCertsToMastersTask(),
			pkiKubexm.NewRolloutK8sCABundleTask(),
			pkiKubexm.NewFinalizeMastersTask(),
			pkiKubexm.NewUpdateWorkerNodesTask(),
			pkiKubexm.NewCleanupTask(),
			// Etcd certs
			pkiEtcd.NewGenerateNewCertificatesTask(),
			pkiEtcd.NewDeployNewLeafCertsRollingTask(),
			pkiEtcd.NewDeployTrustBundleRollingTask(),
			pkiEtcd.NewDeployFinalCARollingTask(),
			pkiEtcd.NewFinalizeWorkspaceTask(),
		}
	}

	return &PKIModule{
		BaseModule: module.NewBaseModule("PKIRenewal", moduleTasks),
		certType:   certType,
	}
}

// Plan generates the execution fragment for the PKI module.
func (m *PKIModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "cert_type", m.certType)

	// Skip etcd certificate renewal if etcd is managed by kubeadm.
	// Kubeadm manages its own etcd certs, and kubexm renewal logic is incompatible.
	cfg := ctx.GetClusterConfig()
	isEtcdTask := m.certType == "etcd-ca" || m.certType == "etcd-certs" || m.certType == "all"
	if isEtcdTask && cfg != nil && cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		logger.Warn("Skipping etcd certificate renewal because etcd is managed by kubeadm. Use 'kubeadm certs renew' instead.")
		if m.certType == "all" {
			// For "all", we continue with k8s certs only
			logger.Info("Continuing with Kubernetes certificate renewal only.")
		} else {
			// For etcd-only types, we skip entirely
			return plan.NewEmptyFragment(m.Name()), nil
		}
	}

	if len(m.Tasks()) == 0 {
		logger.Info("No tasks configured for this certificate type", "cert_type", m.certType)
		return plan.NewEmptyFragment(m.Name()), nil
	}

	// Use BaseModule.PlanTasks() which handles IsRequired checks,
	// task planning, fragment merging, and task chaining.
	moduleFragment, _, err := m.BaseModule.PlanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan tasks in PKI module: %w", err)
	}

	if moduleFragment.IsEmpty() {
		logger.Info("All PKI tasks were skipped (no renewal required for cert type)", "cert_type", m.certType)
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("PKI module planning complete.", "cert_type", m.certType, "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

// Ensure PKIModule implements the module.Module interface.
var _ module.Module = (*PKIModule)(nil)
