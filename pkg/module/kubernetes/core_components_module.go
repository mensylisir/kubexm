package module

import (
	// "fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// containerdtask "github.com/mensylisir/kubexm/pkg/task/containerd"
	// dockertask "github.com/mensylisir/kubexm/pkg/task/docker"
	// etcdtask "github.com/mensylisir/kubexm/pkg/task/etcd"
	// kubernetestask "github.com/mensylisir/kubexm/pkg/task/kubernetes"
)

// CoreComponentsModule groups tasks for installing essential cluster components
// like container runtime, etcd, Kubernetes binaries, and pulling core images.
type CoreComponentsModule struct {
	BaseModule
}

// NewCoreComponentsModule creates a new CoreComponentsModule.
func NewCoreComponentsModule() Module {
	return &CoreComponentsModule{
		BaseModule: NewBaseModule(
			"CoreComponentsInstallation",
			[]task.Task{
				// Tasks will be instantiated here based on config (e.g., Containerd or Docker)
				// Example:
				// containerdtask.NewInstallContainerdTask([]string{common.MasterRole, common.WorkerRole}),
				// etcdtask.NewInstallETCDTask(),
				// kubernetestask.NewInstallBinariesTask(),
				// kubernetestask.NewPullImagesTask(),
			},
		),
	}
}

// Plan orchestrates the planning of tasks within this module.
func (m *CoreComponentsModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	// 1. Determine container runtime task (containerd or docker) based on ctx.GetClusterConfig().
	// 2. Instantiate tasks:
	//    - Container Runtime Install Task
	//    - Etcd Install Task
	//    - Kubernetes Binaries Install Task
	//    - Kubernetes Image Pull Task
	// 3. Plan each task: Call task.Plan(taskCtx).
	// 4. Link task fragments:
	//    - Etcd can often be installed in parallel with container runtime.
	//    - K8s binaries can be installed in parallel with container runtime/etcd.
	//    - Pulling images usually depends on container runtime being ready.
	//    - All these might depend on preflight/resource tasks from a previous module.
	// 5. Return the combined ExecutionFragment.
	return task.NewEmptyFragment(), nil // Placeholder
}

var _ Module = (*CoreComponentsModule)(nil)
