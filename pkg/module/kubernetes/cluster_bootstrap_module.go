package module

import (
	// "fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// kubernetestask "github.com/mensylisir/kubexm/pkg/task/kubernetes"
)

// ClusterBootstrapModule groups tasks for bootstrapping the Kubernetes cluster
// using kubeadm (init master, join other masters, join workers).
type ClusterBootstrapModule struct {
	BaseModule
}

// NewClusterBootstrapModule creates a new ClusterBootstrapModule.
func NewClusterBootstrapModule() Module {
	return &ClusterBootstrapModule{
		BaseModule: NewBaseModule(
			"KubernetesClusterBootstrap",
			[]task.Task{
				// kubernetestask.NewInitMasterTask(),
				// kubernetestask.NewJoinMastersTask(),
				// kubernetestask.NewJoinWorkerNodesTask(),
			},
		),
	}
}

// Plan orchestrates the planning of tasks within this module.
func (m *ClusterBootstrapModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	// 1. Instantiate tasks: InitMaster, JoinMasters, JoinWorkers.
	// 2. Plan each task.
	// 3. Link task fragments:
	//    - JoinMasters depends on InitMaster.
	//    - JoinWorkers depends on InitMaster.
	//    - JoinMasters and JoinWorkers can run in parallel after InitMaster.
	// 4. This module typically depends on CoreComponentsModule (runtime, etcd, k8s binaries ready).
	return task.NewEmptyFragment(), nil // Placeholder
}

var _ Module = (*ClusterBootstrapModule)(nil)
