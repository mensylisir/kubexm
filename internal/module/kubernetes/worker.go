package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// WorkerModule is responsible for setting up Kubernetes worker nodes.
// It delegates to mode-specific implementations based on cluster config.
type WorkerModule struct {
	module.BaseModule
	delegate module.Module
}

// NewWorkerModule creates a new WorkerModule.
// The actual module selected depends on clusterConfig.Spec.Kubernetes.Type.
func NewWorkerModule() module.Module {
	base := module.NewBaseModule("KubernetesWorkerSetup", nil)
	return &WorkerModule{BaseModule: base}
}

func (m *WorkerModule) Name() string {
	if m.delegate != nil {
		return m.delegate.Name()
	}
	return m.BaseModule.Name()
}

func (m *WorkerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
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

	logger.Info("Routing to mode-specific worker module", "type", kubeType)

	// Delegate to the appropriate module based on type
	m.delegate = NewWorkerModuleForType(kubeType)
	return m.delegate.Plan(ctx)
}

var _ module.Module = (*WorkerModule)(nil)
