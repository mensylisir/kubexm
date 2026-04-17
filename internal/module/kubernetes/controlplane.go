package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// ControlPlaneModule is responsible for setting up the Kubernetes control plane.
// It delegates to mode-specific implementations based on cluster config.
type ControlPlaneModule struct {
	module.BaseModule
	delegate module.Module
}

// NewControlPlaneModule creates a new ControlPlaneModule.
// The actual module selected depends on clusterConfig.Spec.Kubernetes.Type.
func NewControlPlaneModule() module.Module {
	base := module.NewBaseModule("KubernetesControlPlaneSetup", nil)
	return &ControlPlaneModule{BaseModule: base}
}

func (m *ControlPlaneModule) Name() string {
	if m.delegate != nil {
		return m.delegate.Name()
	}
	return m.BaseModule.Name()
}

func (m *ControlPlaneModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := taskCtx.GetClusterConfig()
	kubeType := ""
	if clusterCfg.Spec.Kubernetes != nil {
		kubeType = clusterCfg.Spec.Kubernetes.Type
	}
	if kubeType == "" {
		kubeType = string(common.KubernetesDeploymentTypeKubeadm)
	}

	logger.Info("Routing to mode-specific control plane module", "type", kubeType)

	// Delegate to the appropriate module based on type
	m.delegate = NewControlPlaneModuleForType(kubeType)
	return m.delegate.Plan(ctx)
}

var _ module.Module = (*ControlPlaneModule)(nil)
